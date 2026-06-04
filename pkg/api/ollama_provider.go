package api

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strconv"
	"strings"
)

const (
	// ollamaNumPredictEnv overrides the per-request output-token bound
	// (Ollama's options.num_predict). When set to a positive integer it takes
	// precedence over the model-aware default for every request. A value of
	// -1 selects Ollama's "infinite generation" sentinel (unbounded), and -2
	// fills the context — these are passed through verbatim so operators can
	// opt back into unbounded behavior explicitly.
	ollamaNumPredictEnv = "OLLAMA_NUM_PREDICT"

	// defaultOllamaNumPredict is the generous default output-token bound used
	// for non-opus-class models. It mirrors the Rust reference's
	// max_tokens_for_model (crates/api/src/providers/mod.rs), which returns
	// 64_000 for the general case. The value is deliberately large so normal
	// answers are never truncated; it exists only to prevent pathological,
	// unbounded generation from thinking models (e.g. qwen3 emitting <think>
	// until natural stop).
	defaultOllamaNumPredict = 64_000

	// opusOllamaNumPredict mirrors the Rust reference's tighter bound for
	// opus-class models (32_000). Ollama tags rarely contain "opus", but the
	// heuristic is kept identical to the reference for parity.
	opusOllamaNumPredict = 32_000
)

// OllamaProvider implements the Provider interface using Ollama's /api/chat endpoint.
type OllamaProvider struct {
	BaseURL string
	Model   string
	Client  *http.Client
	// NumPredict is the output-token bound applied to every request via
	// Ollama's options.num_predict. A value of 0 means "unset" — the provider
	// falls back to the model-aware default (maxNumPredictForModel). It is
	// populated from OLLAMA_NUM_PREDICT in NewOllamaProvider and may be set
	// directly by callers/tests.
	NumPredict int
}

// maxNumPredictForModel returns the default output-token bound for a model,
// mirroring the Rust reference's max_tokens_for_model: a tighter bound for
// opus-class models and a generous default for everything else. Ollama's
// native field for this limit is options.num_predict.
func maxNumPredictForModel(model string) int {
	if strings.Contains(strings.ToLower(model), "opus") {
		return opusOllamaNumPredict
	}
	return defaultOllamaNumPredict
}

// numPredictFromEnv parses OLLAMA_NUM_PREDICT. It returns (value, true) only
// when the env var holds a valid non-zero integer; a zero or unparseable value
// yields (0, false) so the caller falls back to the model-aware default.
// Negative sentinels (-1 unbounded, -2 fill-context) are honored verbatim.
func numPredictFromEnv() (int, bool) {
	raw := strings.TrimSpace(os.Getenv(ollamaNumPredictEnv))
	if raw == "" {
		return 0, false
	}
	v, err := strconv.Atoi(raw)
	if err != nil || v == 0 {
		return 0, false
	}
	return v, true
}

// NewOllamaProvider constructs an OllamaProvider. BaseURL is read from OLLAMA_BASE_URL
// env (default http://localhost:11434); model is supplied by the caller.
func NewOllamaProvider(baseURL, model string) *OllamaProvider {
	if baseURL == "" {
		baseURL = os.Getenv("OLLAMA_BASE_URL")
	}
	if baseURL == "" {
		baseURL = "http://localhost:11434"
	}
	baseURL = normalizeOllamaBaseURL(baseURL)
	if model == "" {
		model = os.Getenv("OLLAMA_MODEL")
	}
	if model == "" {
		model = os.Getenv("EMBER_MODEL")
	}
	if model == "" {
		model = DefaultModel
	}
	numPredict := 0
	if v, ok := numPredictFromEnv(); ok {
		numPredict = v
	}
	return &OllamaProvider{
		BaseURL:    baseURL,
		Model:      model,
		Client:     &http.Client{},
		NumPredict: numPredict,
	}
}

// normalizeOllamaBaseURL canonicalizes an Ollama base URL so that both the
// native root form (http://HOST:PORT) and the OpenAI-compatible form
// (http://HOST:PORT/v1) resolve to the same native root. The provider appends
// the native "/api/chat" path, so a trailing "/v1" (and any trailing slashes)
// must be stripped to avoid a double-path 404. It is idempotent and host/port
// agnostic — no hardcoded host is assumed.
func normalizeOllamaBaseURL(baseURL string) string {
	trimmed := strings.TrimRight(strings.TrimSpace(baseURL), "/")
	if strings.HasSuffix(trimmed, "/v1") {
		trimmed = strings.TrimRight(strings.TrimSuffix(trimmed, "/v1"), "/")
	}
	return trimmed
}

type ollamaChatRequest struct {
	Model    string          `json:"model"`
	Messages []ollamaMessage `json:"messages"`
	Stream   bool            `json:"stream"`
	// Tools carries the native tool specs (Ollama function-calling). It is
	// omitted entirely for plain text turns so the single-turn SendMessage wire
	// format is unchanged.
	Tools   []ollamaTool   `json:"tools,omitempty"`
	Options *ollamaOptions `json:"options,omitempty"`
}

// ollamaTool / ollamaToolFunction render a ToolDefinition into Ollama's native
// /api/chat `tools` array shape: {"type":"function","function":{name,
// description, parameters}}. Parameters is the JSON-Schema object taken verbatim
// from the registered tool spec — schemas are never hand-written here.
type ollamaTool struct {
	Type     string             `json:"type"`
	Function ollamaToolFunction `json:"function"`
}

type ollamaToolFunction struct {
	Name        string         `json:"name"`
	Description string         `json:"description"`
	Parameters  map[string]any `json:"parameters"`
}

// ollamaToolCall mirrors an entry of Ollama's response message.tool_calls. The
// arguments come back as a decoded JSON object (a map), not an encoded string,
// so they are consumed directly.
type ollamaToolCall struct {
	Function ollamaToolCallFunction `json:"function"`
}

type ollamaToolCallFunction struct {
	Name      string         `json:"name"`
	Arguments map[string]any `json:"arguments"`
}

// ollamaOptions carries Ollama's native generation options. Only num_predict
// (the output-token bound) is set here; the omitempty on the parent field plus
// these tags keep the wire format minimal.
type ollamaOptions struct {
	NumPredict int `json:"num_predict"`
}

type ollamaMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
	// ToolCalls is set on assistant responses that request tools, and when
	// re-sending the accumulated assistant turn back to the model. Omitted for
	// plain turns so the single-turn wire format is unchanged.
	ToolCalls []ollamaToolCall `json:"tool_calls,omitempty"`
	// ToolName labels a "tool" role message with the tool it reports on (Ollama
	// native field). Omitted for non-tool messages.
	ToolName string `json:"tool_name,omitempty"`
}

type ollamaChatStreamLine struct {
	Model     string         `json:"model"`
	CreatedAt string         `json:"created_at"`
	Message   *ollamaMessage `json:"message,omitempty"`
	Done      bool           `json:"done"`
}

// SendMessage POSTs to {BaseURL}/api/chat with streaming enabled, accumulates
// all content deltas, and returns the full response text.
func (p *OllamaProvider) SendMessage(request MessageRequest) (MessageResponse, error) {
	model := request.Model
	if model == "" {
		model = p.Model
	}

	// Resolve the output-token bound: an explicit OLLAMA_NUM_PREDICT override
	// (captured on the provider) wins; otherwise fall back to the model-aware
	// default that mirrors the Rust reference's max_tokens_for_model. This
	// prevents unbounded generation from thinking models while keeping the
	// limit generous enough that normal answers are never truncated.
	numPredict := p.NumPredict
	if numPredict == 0 {
		numPredict = maxNumPredictForModel(model)
	}

	// Prepend the ported agent system prompt ahead of the user message so the
	// model is framed identically to the Rust reference (true parity).
	reqBody := ollamaChatRequest{
		Model: model,
		Messages: []ollamaMessage{
			{Role: "system", Content: BuildSystemPrompt()},
			{Role: "user", Content: request.Prompt},
		},
		Stream:  true,
		Options: &ollamaOptions{NumPredict: numPredict},
	}

	bodyBytes, err := json.Marshal(reqBody)
	if err != nil {
		return MessageResponse{}, fmt.Errorf("ollama: marshal request: %w", err)
	}

	httpResp, err := p.Client.Post(p.BaseURL+"/api/chat", "application/json", bytes.NewReader(bodyBytes))
	if err != nil {
		return MessageResponse{}, fmt.Errorf("ollama: http post: %w", err)
	}
	defer httpResp.Body.Close()

	if httpResp.StatusCode != http.StatusOK {
		return MessageResponse{}, fmt.Errorf("ollama: unexpected status %d", httpResp.StatusCode)
	}

	var accumulated string
	scanner := bufio.NewScanner(httpResp.Body)
	// Increase scanner buffer to handle long lines gracefully.
	scanner.Buffer(make([]byte, 256*1024), 256*1024)
	for scanner.Scan() {
		line := scanner.Bytes()
		if len(line) == 0 {
			continue
		}
		var streamLine ollamaChatStreamLine
		if err := json.Unmarshal(line, &streamLine); err != nil {
			return MessageResponse{}, fmt.Errorf("ollama: decode stream line: %w", err)
		}
		if streamLine.Message != nil {
			accumulated += streamLine.Message.Content
		}
		if streamLine.Done {
			break
		}
	}
	if err := scanner.Err(); err != nil {
		return MessageResponse{}, fmt.Errorf("ollama: read stream: %w", err)
	}

	return MessageResponse{Text: accumulated}, nil
}

// Chat runs ONE agentic turn against {BaseURL}/api/chat using Ollama's native
// tool-calling. It sends the accumulated conversation (request.Messages) plus
// the available tool definitions as the `tools` array, streams assistant text
// deltas to request.Stream as they arrive (when non-nil), and returns the full
// assistant text together with any tool calls parsed from the structured
// message.tool_calls field. The runtime owns the multi-turn loop; this method
// is a thin, single-turn transport mirroring the Rust reference's per-iteration
// api_client.stream call (crates/runtime/src/conversation.rs).
func (p *OllamaProvider) Chat(request ChatRequest) (ChatResponse, error) {
	model := request.Model
	if model == "" {
		model = p.Model
	}

	numPredict := p.NumPredict
	if numPredict == 0 {
		numPredict = maxNumPredictForModel(model)
	}

	messages := make([]ollamaMessage, 0, len(request.Messages))
	for _, m := range request.Messages {
		messages = append(messages, toOllamaMessage(m))
	}

	var tools []ollamaTool
	if len(request.Tools) > 0 {
		tools = make([]ollamaTool, 0, len(request.Tools))
		for _, t := range request.Tools {
			tools = append(tools, ollamaTool{
				Type: "function",
				Function: ollamaToolFunction{
					Name:        t.Name,
					Description: t.Description,
					Parameters:  t.Parameters,
				},
			})
		}
	}

	reqBody := ollamaChatRequest{
		Model:    model,
		Messages: messages,
		// Stream so assistant text can be surfaced incrementally; tool_calls are
		// aggregated across the streamed chunks regardless.
		Stream:  true,
		Tools:   tools,
		Options: &ollamaOptions{NumPredict: numPredict},
	}

	bodyBytes, err := json.Marshal(reqBody)
	if err != nil {
		return ChatResponse{}, fmt.Errorf("ollama: marshal chat request: %w", err)
	}

	httpResp, err := p.Client.Post(p.BaseURL+"/api/chat", "application/json", bytes.NewReader(bodyBytes))
	if err != nil {
		return ChatResponse{}, fmt.Errorf("ollama: http post: %w", err)
	}
	defer httpResp.Body.Close()

	if httpResp.StatusCode != http.StatusOK {
		return ChatResponse{}, fmt.Errorf("ollama: unexpected status %d", httpResp.StatusCode)
	}

	var accumulated strings.Builder
	var toolCalls []ToolCall
	scanner := bufio.NewScanner(httpResp.Body)
	scanner.Buffer(make([]byte, 256*1024), 256*1024)
	for scanner.Scan() {
		line := scanner.Bytes()
		if len(line) == 0 {
			continue
		}
		var streamLine ollamaChatStreamLine
		if err := json.Unmarshal(line, &streamLine); err != nil {
			return ChatResponse{}, fmt.Errorf("ollama: decode chat stream line: %w", err)
		}
		if streamLine.Message != nil {
			if delta := streamLine.Message.Content; delta != "" {
				accumulated.WriteString(delta)
				if request.Stream != nil {
					// Best-effort incremental surface; a write failure must not
					// abort the turn (the full text is still returned).
					fmt.Fprint(request.Stream, delta)
				}
			}
			for i, tc := range streamLine.Message.ToolCalls {
				toolCalls = append(toolCalls, ToolCall{
					ID:        fmt.Sprintf("call_%d", len(toolCalls)+i),
					Name:      tc.Function.Name,
					Arguments: tc.Function.Arguments,
				})
			}
		}
		if streamLine.Done {
			break
		}
	}
	if err := scanner.Err(); err != nil {
		return ChatResponse{}, fmt.Errorf("ollama: read chat stream: %w", err)
	}

	return ChatResponse{Text: accumulated.String(), ToolCalls: toolCalls}, nil
}

// toOllamaMessage renders a provider-agnostic ChatMessage into the Ollama wire
// message, carrying tool calls (assistant turns) and tool name (tool turns).
func toOllamaMessage(m ChatMessage) ollamaMessage {
	out := ollamaMessage{Role: m.Role, Content: m.Content, ToolName: m.ToolName}
	if len(m.ToolCalls) > 0 {
		out.ToolCalls = make([]ollamaToolCall, 0, len(m.ToolCalls))
		for _, tc := range m.ToolCalls {
			out.ToolCalls = append(out.ToolCalls, ollamaToolCall{
				Function: ollamaToolCallFunction{Name: tc.Name, Arguments: tc.Arguments},
			})
		}
	}
	return out
}
