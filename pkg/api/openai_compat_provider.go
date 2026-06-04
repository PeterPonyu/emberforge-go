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
	defaultXAIBaseURL    = "https://api.x.ai/v1"
	defaultOpenAIBaseURL = "https://api.openai.com/v1"
	defaultCompatMaxTok  = 1024
)

// OpenAICompatConfig describes an OpenAI-compatible provider (xAI, OpenAI, …),
// mirroring the Rust OpenAiCompatConfig: which provider it is, the credential
// env var, the base-url override env var, and the default base url.
type OpenAICompatConfig struct {
	ProviderName   string
	APIKeyEnv      string
	BaseURLEnv     string
	DefaultBaseURL string
}

// XAIConfig returns the configuration for xAI's OpenAI-compatible API.
func XAIConfig() OpenAICompatConfig {
	return OpenAICompatConfig{
		ProviderName:   "xAI",
		APIKeyEnv:      "XAI_API_KEY",
		BaseURLEnv:     "XAI_BASE_URL",
		DefaultBaseURL: defaultXAIBaseURL,
	}
}

// OpenAIConfig returns the configuration for OpenAI's API.
func OpenAIConfig() OpenAICompatConfig {
	return OpenAICompatConfig{
		ProviderName:   "OpenAI",
		APIKeyEnv:      "OPENAI_API_KEY",
		BaseURLEnv:     "OPENAI_BASE_URL",
		DefaultBaseURL: defaultOpenAIBaseURL,
	}
}

// OpenAICompatProvider implements Provider against an OpenAI-compatible
// /chat/completions endpoint using bearer authentication, mirroring the Rust
// OpenAiCompatClient request construction.
type OpenAICompatProvider struct {
	Config  OpenAICompatConfig
	BaseURL string
	Model   string
	Auth    AuthSource
	Client  *http.Client
}

// NewOpenAICompatProvider constructs a provider for the given config. The base
// URL is read from the config's BaseURLEnv (default config.DefaultBaseURL).
func NewOpenAICompatProvider(config OpenAICompatConfig, auth AuthSource, model string) *OpenAICompatProvider {
	baseURL := readEnvNonEmpty(config.BaseURLEnv)
	if baseURL == "" {
		baseURL = config.DefaultBaseURL
	}
	return &OpenAICompatProvider{
		Config:  config,
		BaseURL: baseURL,
		Model:   model,
		Auth:    auth,
		Client:  &http.Client{},
	}
}

type chatMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type chatCompletionRequest struct {
	Model     string        `json:"model"`
	MaxTokens int           `json:"max_tokens"`
	Messages  []chatMessage `json:"messages"`
	Stream    bool          `json:"stream"`
}

type chatCompletionChoice struct {
	Message chatMessage `json:"message"`
}

type chatCompletionResponse struct {
	Choices []chatCompletionChoice `json:"choices"`
}

// chatCompletionsEndpoint mirrors the Rust chat_completions_endpoint helper:
// it appends /chat/completions unless the base url already ends with it.
func chatCompletionsEndpoint(baseURL string) string {
	trimmed := strings.TrimRight(baseURL, "/")
	if strings.HasSuffix(trimmed, "/chat/completions") {
		return trimmed
	}
	return trimmed + "/chat/completions"
}

// SendMessage POSTs a non-streaming chat completion and returns the first
// choice's message content.
func (p *OpenAICompatProvider) SendMessage(request MessageRequest) (MessageResponse, error) {
	model := request.Model
	if model == "" {
		model = p.Model
	}

	// Prepend the ported agent system prompt as a "system" role message ahead of
	// the user message, framing the turn identically to the other providers.
	reqBody := chatCompletionRequest{
		Model:     model,
		MaxTokens: defaultCompatMaxTok,
		Messages: []chatMessage{
			{Role: "system", Content: BuildSystemPrompt()},
			{Role: "user", Content: request.Prompt},
		},
		Stream: false,
	}
	bodyBytes, err := json.Marshal(reqBody)
	if err != nil {
		return MessageResponse{}, fmt.Errorf("%s: marshal request: %w", p.Config.ProviderName, err)
	}

	url := chatCompletionsEndpoint(p.BaseURL)
	httpReq, err := http.NewRequest(http.MethodPost, url, bytes.NewReader(bodyBytes))
	if err != nil {
		return MessageResponse{}, fmt.Errorf("%s: build request: %w", p.Config.ProviderName, err)
	}
	httpReq.Header.Set("content-type", "application/json")
	if p.Auth.APIKey != "" {
		httpReq.Header.Set("authorization", "Bearer "+p.Auth.APIKey)
	} else if p.Auth.BearerToken != "" {
		httpReq.Header.Set("authorization", "Bearer "+p.Auth.BearerToken)
	}

	httpResp, err := p.Client.Do(httpReq)
	if err != nil {
		return MessageResponse{}, fmt.Errorf("%s: http post: %w", p.Config.ProviderName, err)
	}
	defer httpResp.Body.Close()

	if httpResp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(httpResp.Body)
		return MessageResponse{}, fmt.Errorf("%s: unexpected status %d: %s", p.Config.ProviderName, httpResp.StatusCode, strings.TrimSpace(string(body)))
	}

	var parsed chatCompletionResponse
	if err := json.NewDecoder(httpResp.Body).Decode(&parsed); err != nil {
		return MessageResponse{}, fmt.Errorf("%s: decode response: %w", p.Config.ProviderName, err)
	}
	if len(parsed.Choices) == 0 {
		return MessageResponse{}, nil
	}
	return MessageResponse{Text: parsed.Choices[0].Message.Content}, nil
}
