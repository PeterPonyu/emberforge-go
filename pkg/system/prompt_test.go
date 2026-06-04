package system

import "testing"

// TestRunPromptOnceRunsExactlyOneTurn verifies the one-shot `ember prompt`
// direct loop reuses the existing runtime: a plain prompt dispatches via
// RoutePrompt and runs ConversationRuntime.RunTurn exactly once, recording the
// turn in the session. The provider call may fail (no local Ollama in CI), but
// the dispatch/turn wiring is exercised regardless of provider outcome.
func TestRunPromptOnceRunsExactlyOneTurn(t *testing.T) {
	app := NewStarterSystemApplication(DefaultStarterSystemConfig())
	defer app.Shutdown()

	output := RunPromptOnce(app, "explain goroutines")

	if output == "" {
		t.Fatal("expected non-empty output from one-shot prompt turn")
	}
	if got := app.Runtime.TurnCount(); got != 1 {
		t.Fatalf("expected exactly one runtime turn, got %d", got)
	}
	last, ok := app.Runtime.LastTurn()
	if !ok || last.Input != "explain goroutines" {
		t.Fatalf("expected last turn input to be the prompt, got %+v (ok=%t)", last, ok)
	}
	record, ok := app.Sequence.LastRecord()
	if !ok || record.Route != RoutePrompt {
		t.Fatalf("expected prompt to dispatch via RoutePrompt, got route=%q (ok=%t)", record.Route, ok)
	}
	if record.Output != output {
		t.Fatalf("expected returned output to match the sequence record output")
	}
}

// TestRunPromptOnceTrimsWhitespace verifies surrounding whitespace is trimmed
// before the prompt reaches the runtime, so `ember prompt "  hi  "` records the
// trimmed input.
func TestRunPromptOnceTrimsWhitespace(t *testing.T) {
	app := NewStarterSystemApplication(DefaultStarterSystemConfig())
	defer app.Shutdown()

	RunPromptOnce(app, "  hello world  ")

	last, ok := app.Runtime.LastTurn()
	if !ok || last.Input != "hello world" {
		t.Fatalf("expected trimmed prompt input, got %+v (ok=%t)", last, ok)
	}
}
