package runtime

import (
	"errors"
	"io"
	"testing"

	"github.com/PeterPonyu/emberforge-go/pkg/api"
	"github.com/PeterPonyu/emberforge-go/pkg/telemetry"
	"github.com/PeterPonyu/emberforge-go/pkg/tools"
)

// failingProvider always returns a real error, standing in for an unreachable
// Ollama endpoint without any network access.
type failingProvider struct{}

func (failingProvider) SendMessage(api.MessageRequest) (api.MessageResponse, error) {
	return api.MessageResponse{}, errors.New("dial tcp 127.0.0.1:1: connect: connection refused")
}

func newTestRuntime(p api.Provider) *ConversationRuntime {
	sink := telemetry.ConsoleTelemetrySink{Writer: io.Discard}
	return NewConversationRuntime(p, tools.NewRealToolExecutor(""), sink)
}

// TestRunTurnResult_PropagatesProviderError verifies the real provider error is
// returned (not just rendered into the output text) so callers can exit
// non-zero on a genuine failure.
func TestRunTurnResult_PropagatesProviderError(t *testing.T) {
	rt := newTestRuntime(failingProvider{})

	output, err := rt.RunTurnResult("say hi")
	if err == nil {
		t.Fatal("expected a non-nil error when the provider fails")
	}
	if output == "" {
		t.Fatal("expected a rendered error message even on failure")
	}
}

// TestRunTurnResult_NilErrorOnSuccess verifies the success path returns the
// model answer and a nil error.
func TestRunTurnResult_NilErrorOnSuccess(t *testing.T) {
	rt := newTestRuntime(api.MockProvider{})

	output, err := rt.RunTurnResult("say hi")
	if err != nil {
		t.Fatalf("expected nil error on success, got %v", err)
	}
	if output == "" {
		t.Fatal("expected non-empty output on success")
	}
}

// TestRunTurn_DiscardsError verifies the legacy RunTurn wrapper still returns
// the rendered output (and swallows the error) for display-only callers.
func TestRunTurn_DiscardsError(t *testing.T) {
	rt := newTestRuntime(failingProvider{})

	if output := rt.RunTurn("say hi"); output == "" {
		t.Fatal("expected non-empty rendered output from RunTurn")
	}
}
