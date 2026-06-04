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
	Options  *ollamaOptions  `json:"options,omitempty"`
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
