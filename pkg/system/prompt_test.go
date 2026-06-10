package system

import (
	"errors"
	"io"
	"testing"

	"github.com/PeterPonyu/emberforge-go/pkg/api"
	"github.com/PeterPonyu/emberforge-go/pkg/runtime"
)

// failingProvider is a stub api.Provider that always returns a real error,
// standing in for an unreachable Ollama endpoint without any network access.
type failingProvider struct{}

func (failingProvider) SendMessage(api.MessageRequest) (api.MessageResponse, error) {
	return api.MessageResponse{}, errors.New("dial tcp 127.0.0.1:1: connect: connection refused")
}

// appWithProvider builds a starter application whose runtime/sequence are wired
// to the supplied provider, with console telemetry discarded so test output
// stays clean. It exercises the same RunPromptOnce path the `ember prompt`
// command uses.
func appWithProvider(p api.Provider) *StarterSystemApplication {
	config := DefaultStarterSystemConfig()
	config.ConsoleTelemetryWriter = io.Discard
	app := NewStarterSystemApplication(config)
	rt := runtime.NewConversationRuntime(p, app.ToolExecutor, app.Telemetry)
	app.Runtime = rt
	app.Sequence = NewControlSequenceEngine(rt, app.Dispatcher, app.Lifecycle, app.Telemetry)
	return app
}

// TestRunPromptOnceSucceedsWithoutError verifies the success path: a working
// provider yields the model answer and a nil error, so `ember prompt` exits 0.
func TestRunPromptOnceSucceedsWithoutError(t *testing.T) {
	app := appWithProvider(api.MockProvider{})
	defer app.Shutdown()

	output, err := RunPromptOnce(app, "say hi")
	if err != nil {
		t.Fatalf("expected nil error on success, got %v", err)
	}
	if output == "" {
		t.Fatal("expected non-empty model answer on success")
	}
}

// TestRunPromptOncePropagatesProviderError verifies the failure path: a genuine
// provider error is plumbed up as a real error value (not string-matched), so
// `ember prompt` can exit non-zero on it.
func TestRunPromptOncePropagatesProviderError(t *testing.T) {
	app := appWithProvider(failingProvider{})
	defer app.Shutdown()

	_, err := RunPromptOnce(app, "say hi")
	if err == nil {
		t.Fatal("expected a non-nil error when the provider fails")
	}
}

// TestRunPromptOnceRunsExactlyOneTurn verifies the one-shot `ember prompt`
// direct loop reuses the existing runtime: a plain prompt dispatches via
// RoutePrompt and runs ConversationRuntime.RunTurn exactly once, recording the
// turn in the session. The provider call may fail (no local Ollama in CI), but
// the dispatch/turn wiring is exercised regardless of provider outcome.
func TestRunPromptOnceRunsExactlyOneTurn(t *testing.T) {
	app := NewStarterSystemApplication(DefaultStarterSystemConfig())
	defer app.Shutdown()

	output, _ := RunPromptOnce(app, "explain goroutines")

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
