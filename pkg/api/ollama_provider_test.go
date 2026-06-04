package api

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestOllamaProvider_StreamingConcatenation(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/chat" {
			t.Fatalf("unexpected path %s", r.URL.Path)
		}
		if r.Method != http.MethodPost {
			t.Fatalf("unexpected method %s", r.Method)
		}
		w.Header().Set("Content-Type", "application/x-ndjson")
		fmt.Fprintln(w, `{"model":"test","message":{"role":"assistant","content":"He"},"done":false}`)
		fmt.Fprintln(w, `{"model":"test","message":{"role":"assistant","content":"llo"},"done":false}`)
		fmt.Fprintln(w, `{"model":"test","done":true}`)
	}))
	defer srv.Close()

	p := NewOllamaProvider(srv.URL, "test")
	resp, err := p.SendMessage(MessageRequest{Model: "test", Prompt: "hi"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.Text != "Hello" {
		t.Fatalf("got %q want %q", resp.Text, "Hello")
	}
}

func TestOllamaProvider_EmptyStream(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/x-ndjson")
		fmt.Fprintln(w, `{"model":"test","done":true}`)
	}))
	defer srv.Close()

	p := NewOllamaProvider(srv.URL, "test")
	resp, err := p.SendMessage(MessageRequest{Model: "test", Prompt: "hi"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.Text != "" {
		t.Fatalf("got %q want empty string", resp.Text)
	}
}

func TestOllamaProvider_HTTPError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "internal server error", http.StatusInternalServerError)
	}))
	defer srv.Close()

	p := NewOllamaProvider(srv.URL, "test")
	_, err := p.SendMessage(MessageRequest{Model: "test", Prompt: "hi"})
	if err == nil {
		t.Fatal("expected error for non-200 status, got nil")
	}
}

func TestOllamaProvider_DefaultBaseURL(t *testing.T) {
	p := NewOllamaProvider("", "llama3")
	if p.BaseURL == "" {
		t.Fatal("expected non-empty BaseURL")
	}
	if p.Model != "llama3" {
		t.Fatalf("got model %q want %q", p.Model, "llama3")
	}
}

func TestOllamaProvider_DefaultModelFromEnv(t *testing.T) {
	t.Setenv("OLLAMA_MODEL", "env-model")
	p := NewOllamaProvider("", "")
	if p.Model != "env-model" {
		t.Fatalf("got model %q want %q", p.Model, "env-model")
	}
}

// TestNormalizeOllamaBaseURL verifies the base URL is canonicalized
// idempotently and host/port agnostically: both the native root form and the
// OpenAI-compatible "/v1" form collapse to the same native root, and existing
// root configs are untouched.
func TestNormalizeOllamaBaseURL(t *testing.T) {
	cases := []struct {
		in   string
		want string
	}{
		{"http://localhost:11434", "http://localhost:11434"},
		{"http://localhost:11434/", "http://localhost:11434"},
		{"http://localhost:11434/v1", "http://localhost:11434"},
		{"http://localhost:11434/v1/", "http://localhost:11434"},
		{"http://127.0.0.1:9999/v1", "http://127.0.0.1:9999"},
		{"http://remote-host:8080", "http://remote-host:8080"},
		{"  http://localhost:11434/v1  ", "http://localhost:11434"},
	}
	for _, tc := range cases {
		if got := normalizeOllamaBaseURL(tc.in); got != tc.want {
			t.Errorf("normalizeOllamaBaseURL(%q) = %q, want %q", tc.in, got, tc.want)
		}
		// Idempotency: normalizing an already-normalized value is a no-op.
		if got := normalizeOllamaBaseURL(tc.want); got != tc.want {
			t.Errorf("normalizeOllamaBaseURL is not idempotent for %q: got %q", tc.want, got)
		}
	}
}

// TestOllamaProvider_V1SuffixReachesChatEndpoint verifies that a base URL with
// the OpenAI-compatible "/v1" suffix still routes to the native /api/chat path
// (D1) rather than 404ing on /v1/api/chat.
func TestOllamaProvider_V1SuffixReachesChatEndpoint(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/chat" {
			t.Fatalf("unexpected path %s (base /v1 suffix not normalized)", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/x-ndjson")
		fmt.Fprintln(w, `{"model":"test","message":{"role":"assistant","content":"hi"},"done":true}`)
	}))
	defer srv.Close()

	p := NewOllamaProvider(srv.URL+"/v1", "test")
	resp, err := p.SendMessage(MessageRequest{Model: "test", Prompt: "hi"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.Text != "hi" {
		t.Fatalf("got %q want %q", resp.Text, "hi")
	}
}

// captureOllamaRequest spins up a stub /api/chat server, runs one SendMessage
// against the supplied provider, and returns the decoded request body the
// provider sent. It fails the test on any transport error.
func captureOllamaRequest(t *testing.T, p *OllamaProvider, req MessageRequest) ollamaChatRequest {
	t.Helper()
	var captured ollamaChatRequest
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, err := io.ReadAll(r.Body)
		if err != nil {
			t.Fatalf("read request body: %v", err)
		}
		if err := json.Unmarshal(body, &captured); err != nil {
			t.Fatalf("decode request body: %v", err)
		}
		w.Header().Set("Content-Type", "application/x-ndjson")
		fmt.Fprintln(w, `{"model":"test","message":{"role":"assistant","content":"ok"},"done":true}`)
	}))
	defer srv.Close()

	p.BaseURL = srv.URL
	if _, err := p.SendMessage(req); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	return captured
}

// TestOllamaProvider_SendsModelAwareNumPredict asserts the request body carries
// the model-aware default num_predict (mirroring the Rust reference) when no
// explicit override is configured.
func TestOllamaProvider_SendsModelAwareNumPredict(t *testing.T) {
	p := NewOllamaProvider("http://placeholder", "qwen3:8b")
	got := captureOllamaRequest(t, p, MessageRequest{Model: "qwen3:8b", Prompt: "hi"})

	if got.Options == nil {
		t.Fatal("expected options to be present in request body, got nil")
	}
	if got.Options.NumPredict != defaultOllamaNumPredict {
		t.Fatalf("num_predict = %d, want %d", got.Options.NumPredict, defaultOllamaNumPredict)
	}
}

// TestOllamaProvider_OpusModelUsesTighterBound mirrors the reference's
// opus-class branch of max_tokens_for_model.
func TestOllamaProvider_OpusModelUsesTighterBound(t *testing.T) {
	p := NewOllamaProvider("http://placeholder", "some-opus-tag")
	got := captureOllamaRequest(t, p, MessageRequest{Model: "some-opus-tag", Prompt: "hi"})

	if got.Options == nil || got.Options.NumPredict != opusOllamaNumPredict {
		t.Fatalf("num_predict = %v, want %d", got.Options, opusOllamaNumPredict)
	}
}

// TestOllamaProvider_EnvOverridesNumPredict asserts OLLAMA_NUM_PREDICT takes
// precedence over the model-aware default and is sent verbatim.
func TestOllamaProvider_EnvOverridesNumPredict(t *testing.T) {
	t.Setenv("OLLAMA_NUM_PREDICT", "2048")
	p := NewOllamaProvider("http://placeholder", "qwen3:8b")
	if p.NumPredict != 2048 {
		t.Fatalf("provider NumPredict = %d, want 2048", p.NumPredict)
	}
	got := captureOllamaRequest(t, p, MessageRequest{Model: "qwen3:8b", Prompt: "hi"})

	if got.Options == nil || got.Options.NumPredict != 2048 {
		t.Fatalf("num_predict = %v, want 2048", got.Options)
	}
}

// TestOllamaProvider_EnvUnboundedSentinel verifies the -1 ("infinite
// generation") sentinel is honored verbatim so operators can opt back into
// unbounded output explicitly.
func TestOllamaProvider_EnvUnboundedSentinel(t *testing.T) {
	t.Setenv("OLLAMA_NUM_PREDICT", "-1")
	p := NewOllamaProvider("http://placeholder", "qwen3:8b")
	got := captureOllamaRequest(t, p, MessageRequest{Model: "qwen3:8b", Prompt: "hi"})

	if got.Options == nil || got.Options.NumPredict != -1 {
		t.Fatalf("num_predict = %v, want -1", got.Options)
	}
}

// TestMaxNumPredictForModel locks the model-aware bound to the reference's
// max_tokens_for_model heuristic.
func TestMaxNumPredictForModel(t *testing.T) {
	if got := maxNumPredictForModel("claude-opus-4-6"); got != opusOllamaNumPredict {
		t.Fatalf("opus bound = %d, want %d", got, opusOllamaNumPredict)
	}
	if got := maxNumPredictForModel("qwen3:32b"); got != defaultOllamaNumPredict {
		t.Fatalf("default bound = %d, want %d", got, defaultOllamaNumPredict)
	}
}

// TestOllamaProvider_SendsSystemPrompt asserts the outgoing /api/chat request
// prepends a "system" role message carrying the ported agent system prompt
// (identified by a stable intro marker line) ahead of the user message. This is
// the core parity guarantee: the model is now framed as the agent before it
// sees the user prompt.
func TestOllamaProvider_SendsSystemPrompt(t *testing.T) {
	p := NewOllamaProvider("http://placeholder", "qwen3:8b")
	got := captureOllamaRequest(t, p, MessageRequest{Model: "qwen3:8b", Prompt: "hi"})

	if len(got.Messages) != 2 {
		t.Fatalf("expected [system, user] messages, got %d: %+v", len(got.Messages), got.Messages)
	}
	if got.Messages[0].Role != "system" {
		t.Fatalf("first message role = %q, want \"system\"", got.Messages[0].Role)
	}
	if !strings.Contains(got.Messages[0].Content, systemPromptIntroMarker) {
		t.Fatalf("system message missing intro marker %q; content: %q", systemPromptIntroMarker, got.Messages[0].Content)
	}
	if got.Messages[1].Role != "user" || got.Messages[1].Content != "hi" {
		t.Fatalf("second message = %+v, want user/\"hi\"", got.Messages[1])
	}
}

func TestMockProvider_RetainsInterface(t *testing.T) {
	var p Provider = MockProvider{}
	resp, err := p.SendMessage(MessageRequest{Model: "m", Prompt: "hi"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.Text == "" {
		t.Fatal("expected non-empty text from MockProvider")
	}
}
