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
