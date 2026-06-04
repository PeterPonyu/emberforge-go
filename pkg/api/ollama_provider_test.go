package api

import (
	"fmt"
	"net/http"
	"net/http/httptest"
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
