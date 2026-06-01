package api

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
)

const (
	defaultAnthropicBaseURL = "https://api.anthropic.com"
	anthropicVersion        = "2023-06-01"
	defaultAnthropicMaxTok  = 1024
)

// AnthropicProvider implements Provider against Anthropic's /v1/messages
// endpoint. It mirrors the Rust ClawApiClient request construction: the
// x-api-key and/or bearer Authorization headers are attached from the resolved
// AuthSource, alongside the anthropic-version header.
type AnthropicProvider struct {
	BaseURL string
	Model   string
	Auth    AuthSource
	Client  *http.Client
}

// NewAnthropicProvider constructs an AnthropicProvider. The base URL is read
// from ANTHROPIC_BASE_URL (default https://api.anthropic.com).
func NewAnthropicProvider(auth AuthSource, model string) *AnthropicProvider {
	baseURL := readEnvNonEmpty("ANTHROPIC_BASE_URL")
	if baseURL == "" {
		baseURL = defaultAnthropicBaseURL
	}
	return &AnthropicProvider{
		BaseURL: baseURL,
		Model:   model,
		Auth:    auth,
		Client:  &http.Client{},
	}
}

type anthropicMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type anthropicRequest struct {
	Model     string             `json:"model"`
	MaxTokens int                `json:"max_tokens"`
	Messages  []anthropicMessage `json:"messages"`
	Stream    bool               `json:"stream"`
}

type anthropicContentBlock struct {
	Type string `json:"type"`
	Text string `json:"text"`
}

type anthropicResponse struct {
	Content []anthropicContentBlock `json:"content"`
}

// SendMessage POSTs a non-streaming request to {BaseURL}/v1/messages and
// concatenates the returned text content blocks.
func (p *AnthropicProvider) SendMessage(request MessageRequest) (MessageResponse, error) {
	model := request.Model
	if model == "" {
		model = p.Model
	}

	reqBody := anthropicRequest{
		Model:     model,
		MaxTokens: defaultAnthropicMaxTok,
		Messages: []anthropicMessage{
			{Role: "user", Content: request.Prompt},
		},
		Stream: false,
	}
	bodyBytes, err := json.Marshal(reqBody)
	if err != nil {
		return MessageResponse{}, fmt.Errorf("anthropic: marshal request: %w", err)
	}

	url := strings.TrimRight(p.BaseURL, "/") + "/v1/messages"
	httpReq, err := http.NewRequest(http.MethodPost, url, bytes.NewReader(bodyBytes))
	if err != nil {
		return MessageResponse{}, fmt.Errorf("anthropic: build request: %w", err)
	}
	httpReq.Header.Set("content-type", "application/json")
	httpReq.Header.Set("anthropic-version", anthropicVersion)
	if p.Auth.APIKey != "" {
		httpReq.Header.Set("x-api-key", p.Auth.APIKey)
	}
	if p.Auth.BearerToken != "" {
		httpReq.Header.Set("authorization", "Bearer "+p.Auth.BearerToken)
	}

	httpResp, err := p.Client.Do(httpReq)
	if err != nil {
		return MessageResponse{}, fmt.Errorf("anthropic: http post: %w", err)
	}
	defer httpResp.Body.Close()

	if httpResp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(httpResp.Body)
		return MessageResponse{}, fmt.Errorf("anthropic: unexpected status %d: %s", httpResp.StatusCode, strings.TrimSpace(string(body)))
	}

	var parsed anthropicResponse
	if err := json.NewDecoder(httpResp.Body).Decode(&parsed); err != nil {
		return MessageResponse{}, fmt.Errorf("anthropic: decode response: %w", err)
	}

	var sb strings.Builder
	for _, block := range parsed.Content {
		if block.Type == "text" {
			sb.WriteString(block.Text)
		}
	}
	return MessageResponse{Text: sb.String()}, nil
}
