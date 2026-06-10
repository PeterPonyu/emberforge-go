package api

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// TestStripLeadingThinkBlock covers the parser directly: a well-formed leading
// think block is separated; legitimate content (including a non-leading mention
// of the tag or a leading "<" that is not a think tag) is never mangled.
func TestStripLeadingThinkBlock(t *testing.T) {
	cases := []struct {
		name         string
		in           string
		wantAnswer   string
		wantThinking string
	}{
		{"leading think", "<think>reasoning here</think>The answer", "The answer", "reasoning here"},
		{"leading think with ws", "\n  <think>plan</think>\nHello", "Hello", "plan"},
		{"no think", "Just the answer", "Just the answer", ""},
		{"non-leading think mention", "Use <think> tags in HTML", "Use <think> tags in HTML", ""},
		{"leading angle not think", "<div>markup</div>", "<div>markup</div>", ""},
		{"unclosed think", "<think>still thinking", "", "still thinking"},
		{"multiline think", "<think>line1\nline2</think>answer", "answer", "line1\nline2"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			answer, thinking := stripLeadingThinkBlock(tc.in)
			if answer != tc.wantAnswer {
				t.Fatalf("answer = %q, want %q", answer, tc.wantAnswer)
			}
			if thinking != tc.wantThinking {
				t.Fatalf("thinking = %q, want %q", thinking, tc.wantThinking)
			}
		})
	}
}

// TestThinkSplitterStreamingSplitTag verifies the splitter correctly handles tag
// boundaries that arrive split across streamed deltas.
func TestThinkSplitterStreamingSplitTag(t *testing.T) {
	var s thinkSplitter
	var answer, thinking strings.Builder
	// "<think>reason</think>done" delivered one rune at a time.
	for _, r := range "<think>reason</think>done" {
		a, th := s.push(string(r))
		answer.WriteString(a)
		thinking.WriteString(th)
	}
	a, th := s.flush()
	answer.WriteString(a)
	thinking.WriteString(th)

	if answer.String() != "done" {
		t.Fatalf("answer = %q, want %q", answer.String(), "done")
	}
	if thinking.String() != "reason" {
		t.Fatalf("thinking = %q, want %q", thinking.String(), "reason")
	}
}

// TestOllamaChatStripsLeadingThinkFromStream verifies the Chat path keeps inline
// <think> reasoning out of the streamed answer and the returned text, and routes
// it to the thinking sink only when EMBER_SHOW_THINKING is enabled.
func TestOllamaChatStripsLeadingThinkFromStream(t *testing.T) {
	newServer := func() *httptest.Server {
		return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/x-ndjson")
			fmt.Fprintln(w, `{"model":"qwen3","message":{"role":"assistant","content":"<think>reasoning"},"done":false}`)
			fmt.Fprintln(w, `{"model":"qwen3","message":{"role":"assistant","content":" steps</think>Final answer"},"done":false}`)
			fmt.Fprintln(w, `{"model":"qwen3","done":true}`)
		}))
	}

	// Thinking OFF (default): answer only on the stream + returned text; thinking
	// sink stays empty.
	t.Run("thinking off", func(t *testing.T) {
		t.Setenv(EmberShowThinkingEnv, "")
		srv := newServer()
		defer srv.Close()
		var stream, thinkSink strings.Builder
		p := NewOllamaProvider(srv.URL, "qwen3")
		p.ThinkingWriter = &thinkSink
		resp, err := p.Chat(ChatRequest{Model: "qwen3", Messages: []ChatMessage{{Role: "user", Content: "hi"}}, Stream: &stream})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if resp.Text != "Final answer" {
			t.Fatalf("returned text = %q, want %q", resp.Text, "Final answer")
		}
		if strings.Contains(stream.String(), "<think>") || strings.Contains(stream.String(), "reasoning") {
			t.Fatalf("think content leaked to stdout stream: %q", stream.String())
		}
		if stream.String() != "Final answer" {
			t.Fatalf("streamed answer = %q, want %q", stream.String(), "Final answer")
		}
		if thinkSink.Len() != 0 {
			t.Fatalf("thinking surfaced while disabled: %q", thinkSink.String())
		}
	})

	// Thinking ON: reasoning is surfaced to the thinking sink; stdout stays clean.
	t.Run("thinking on", func(t *testing.T) {
		t.Setenv(EmberShowThinkingEnv, "1")
		srv := newServer()
		defer srv.Close()
		var stream, thinkSink strings.Builder
		p := NewOllamaProvider(srv.URL, "qwen3")
		p.ThinkingWriter = &thinkSink
		resp, err := p.Chat(ChatRequest{Model: "qwen3", Messages: []ChatMessage{{Role: "user", Content: "hi"}}, Stream: &stream})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if resp.Text != "Final answer" {
			t.Fatalf("returned text = %q, want %q", resp.Text, "Final answer")
		}
		if stream.String() != "Final answer" {
			t.Fatalf("streamed answer = %q, want %q", stream.String(), "Final answer")
		}
		if got := thinkSink.String(); got != "reasoning steps" {
			t.Fatalf("thinking sink = %q, want %q", got, "reasoning steps")
		}
	})
}

// TestOllamaChatPrefersStructuredThinking verifies the structured message.thinking
// field is kept out of the answer and surfaced when enabled.
func TestOllamaChatPrefersStructuredThinking(t *testing.T) {
	t.Setenv(EmberShowThinkingEnv, "on")
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/x-ndjson")
		fmt.Fprintln(w, `{"model":"qwen3","message":{"role":"assistant","thinking":"deep thoughts","content":"Answer"},"done":false}`)
		fmt.Fprintln(w, `{"model":"qwen3","done":true}`)
	}))
	defer srv.Close()

	var stream, thinkSink strings.Builder
	p := NewOllamaProvider(srv.URL, "qwen3")
	p.ThinkingWriter = &thinkSink
	resp, err := p.Chat(ChatRequest{Model: "qwen3", Messages: []ChatMessage{{Role: "user", Content: "hi"}}, Stream: &stream})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.Text != "Answer" {
		t.Fatalf("returned text = %q, want %q", resp.Text, "Answer")
	}
	if !strings.Contains(thinkSink.String(), "deep thoughts") {
		t.Fatalf("structured thinking not surfaced: %q", thinkSink.String())
	}
}

// TestSendMessageStripsThinking verifies the single-shot SendMessage path also
// returns the answer only, with the leading think block stripped.
func TestSendMessageStripsThinking(t *testing.T) {
	t.Setenv(EmberShowThinkingEnv, "")
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/x-ndjson")
		fmt.Fprintln(w, `{"model":"qwen3","message":{"role":"assistant","content":"<think>hmm</think>Hi there"},"done":true}`)
	}))
	defer srv.Close()

	p := NewOllamaProvider(srv.URL, "qwen3")
	resp, err := p.SendMessage(MessageRequest{Model: "qwen3", Prompt: "hi"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.Text != "Hi there" {
		t.Fatalf("text = %q, want %q", resp.Text, "Hi there")
	}
}

func TestEnvFlagEnabled(t *testing.T) {
	for _, v := range []string{"1", "true", "TRUE", "yes", "On"} {
		if !envFlagEnabled(v) {
			t.Fatalf("envFlagEnabled(%q) = false, want true", v)
		}
	}
	for _, v := range []string{"", "0", "false", "no", "off", "bogus"} {
		if envFlagEnabled(v) {
			t.Fatalf("envFlagEnabled(%q) = true, want false", v)
		}
	}
}
