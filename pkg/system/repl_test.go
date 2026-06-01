package system

import (
	"strings"
	"testing"
)

func TestRunStarterReplDispatchesAndExitsOnEOF(t *testing.T) {
	app := NewStarterSystemApplication(DefaultStarterSystemConfig())
	defer app.Shutdown()

	input := strings.NewReader("/help\n/status\n\n")
	var output strings.Builder

	if err := RunStarterRepl(app, input, &output); err != nil {
		t.Fatalf("repl returned error: %v", err)
	}

	rendered := output.String()
	for _, expected := range []string{
		"emberforge-go REPL",
		ReplPrompt,
		"available commands:",
		"[command] status:",
		"[repl] goodbye",
	} {
		if !strings.Contains(rendered, expected) {
			t.Fatalf("expected repl output to contain %q, got:\n%s", expected, rendered)
		}
	}

	// A prompt is printed before every read: /help, /status, the skipped blank
	// line, and the final read that hits EOF -- four prompts in total.
	if got := strings.Count(rendered, ReplPrompt); got != 4 {
		t.Fatalf("expected 4 prompts (two inputs + blank line + EOF read), got %d:\n%s", got, rendered)
	}
}

func TestRunStarterReplExitCommand(t *testing.T) {
	app := NewStarterSystemApplication(DefaultStarterSystemConfig())
	defer app.Shutdown()

	input := strings.NewReader("/status\n/exit\n/help\n")
	var output strings.Builder

	if err := RunStarterRepl(app, input, &output); err != nil {
		t.Fatalf("repl returned error: %v", err)
	}

	rendered := output.String()
	if !strings.Contains(rendered, "[command] status:") {
		t.Fatalf("expected status output before exit, got:\n%s", rendered)
	}
	if strings.Contains(rendered, "available commands:") {
		t.Fatalf("expected /help after /exit to be ignored, got:\n%s", rendered)
	}
	if !strings.Contains(rendered, "[repl] goodbye") {
		t.Fatalf("expected goodbye on /exit, got:\n%s", rendered)
	}
}

func TestRunStarterReplPromptRoutesToSequence(t *testing.T) {
	app := NewStarterSystemApplication(DefaultStarterSystemConfig())
	defer app.Shutdown()

	input := strings.NewReader("just a prompt\n")
	var output strings.Builder

	if err := RunStarterRepl(app, input, &output); err != nil {
		t.Fatalf("repl returned error: %v", err)
	}

	if app.Runtime.TurnCount() == 0 {
		t.Fatal("expected prompt to accumulate a session turn")
	}
	last, ok := app.Runtime.LastTurn()
	if !ok || last.Input != "just a prompt" {
		t.Fatalf("expected last session turn to be the prompt, got %+v (ok=%t)", last, ok)
	}
}
