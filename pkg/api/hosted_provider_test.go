package api

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestAnthropicProvider_RequestConstructionAndResponse(t *testing.T) {
	var gotPath, gotAPIKey, gotVersion, gotAuth string
	var gotBody anthropicRequest

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		gotAPIKey = r.Header.Get("x-api-key")
		gotVersion = r.Header.Get("anthropic-version")
		gotAuth = r.Header.Get("authorization")
		raw, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(raw, &gotBody)
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, `{"content":[{"type":"text","text":"Hello "},{"type":"text","text":"world"}]}`)
	}))
	defer srv.Close()

	p := NewAnthropicProvider(AuthSource{APIKey: "k-1", BearerToken: "b-1"}, "claude-opus-4-6")
	p.BaseURL = srv.URL

	resp, err := p.SendMessage(MessageRequest{Prompt: "hi there"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.Text != "Hello world" {
		t.Errorf("got %q want %q", resp.Text, "Hello world")
	}
	if gotPath != "/v1/messages" {
		t.Errorf("path %q want /v1/messages", gotPath)
	}
	if gotAPIKey != "k-1" {
		t.Errorf("x-api-key %q want k-1", gotAPIKey)
	}
	if gotAuth != "Bearer b-1" {
		t.Errorf("authorization %q want Bearer b-1", gotAuth)
	}
	if gotVersion != anthropicVersion {
		t.Errorf("anthropic-version %q want %q", gotVersion, anthropicVersion)
	}
	if gotBody.Model != "claude-opus-4-6" || len(gotBody.Messages) != 1 || gotBody.Messages[0].Content != "hi there" {
		t.Errorf("unexpected request body: %+v", gotBody)
	}
	// The ported agent system prompt is carried in Anthropic's top-level
	// `system` field (not a message role), so the messages array stays user-only.
	if !strings.Contains(gotBody.System, systemPromptIntroMarker) {
		t.Errorf("expected system field to contain %q, got %q", systemPromptIntroMarker, gotBody.System)
	}
}

func TestAnthropicProvider_HTTPErrorWrapped(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
	}))
	defer srv.Close()

	p := NewAnthropicProvider(AuthSource{APIKey: "k"}, "claude-opus-4-6")
	p.BaseURL = srv.URL
	if _, err := p.SendMessage(MessageRequest{Prompt: "hi"}); err == nil {
		t.Fatal("expected error for non-200 status")
	}
}

func TestOpenAICompatProvider_RequestConstructionAndResponse(t *testing.T) {
	var gotPath, gotAuth string
	var gotBody chatCompletionRequest

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		gotAuth = r.Header.Get("authorization")
		raw, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(raw, &gotBody)
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, `{"choices":[{"message":{"role":"assistant","content":"grok says hi"}}]}`)
	}))
	defer srv.Close()

	cfg := XAIConfig()
	p := NewOpenAICompatProvider(cfg, AuthSource{APIKey: "xai-key"}, "grok-3")
	p.BaseURL = srv.URL

	resp, err := p.SendMessage(MessageRequest{Prompt: "hello grok"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.Text != "grok says hi" {
		t.Errorf("got %q want %q", resp.Text, "grok says hi")
	}
	if gotPath != "/chat/completions" {
		t.Errorf("path %q want /chat/completions", gotPath)
	}
	if gotAuth != "Bearer xai-key" {
		t.Errorf("authorization %q want Bearer xai-key", gotAuth)
	}
	// The ported agent system prompt is prepended as a "system" role message, so
	// the messages array is [system, user].
	if gotBody.Model != "grok-3" || len(gotBody.Messages) != 2 {
		t.Errorf("unexpected request body: %+v", gotBody)
	}
	if gotBody.Messages[0].Role != "system" || !strings.Contains(gotBody.Messages[0].Content, systemPromptIntroMarker) {
		t.Errorf("expected first message to be system prompt, got %+v", gotBody.Messages[0])
	}
	if gotBody.Messages[1].Role != "user" || gotBody.Messages[1].Content != "hello grok" {
		t.Errorf("expected second message to be the user prompt, got %+v", gotBody.Messages[1])
	}
}

func TestOpenAICompatProvider_EmptyChoices(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, `{"choices":[]}`)
	}))
	defer srv.Close()

	p := NewOpenAICompatProvider(XAIConfig(), AuthSource{APIKey: "k"}, "grok-3")
	p.BaseURL = srv.URL
	resp, err := p.SendMessage(MessageRequest{Prompt: "hi"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.Text != "" {
		t.Errorf("got %q want empty", resp.Text)
	}
}

func TestChatCompletionsEndpoint(t *testing.T) {
	cases := map[string]string{
		"https://api.x.ai/v1":                  "https://api.x.ai/v1/chat/completions",
		"https://api.x.ai/v1/":                 "https://api.x.ai/v1/chat/completions",
		"https://api.x.ai/v1/chat/completions": "https://api.x.ai/v1/chat/completions",
	}
	for in, want := range cases {
		if got := chatCompletionsEndpoint(in); got != want {
			t.Errorf("chatCompletionsEndpoint(%q)=%q want %q", in, got, want)
		}
	}
}
