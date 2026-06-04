package api

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strings"
)

// OllamaProvider implements the Provider interface using Ollama's /api/chat endpoint.
type OllamaProvider struct {
	BaseURL string
	Model   string
	Client  *http.Client
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
	return &OllamaProvider{
		BaseURL: baseURL,
		Model:   model,
		Client:  &http.Client{},
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

	reqBody := ollamaChatRequest{
		Model: model,
		Messages: []ollamaMessage{
			{Role: "user", Content: request.Prompt},
		},
		Stream: true,
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
