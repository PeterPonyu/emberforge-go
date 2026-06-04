package runtime

import (
	"fmt"
	"strings"

	"github.com/PeterPonyu/emberforge-go/pkg/api"
	"github.com/PeterPonyu/emberforge-go/pkg/telemetry"
	"github.com/PeterPonyu/emberforge-go/pkg/tools"
)

const RustRuntimeReference = "github.com/PeterPonyu/emberforge-go/pkg/runtime"

type ConversationRuntime struct {
	Provider     api.Provider
	ToolExecutor tools.ToolExecutor
	Telemetry    telemetry.TelemetrySink
	Session      *Session
}

func NewConversationRuntime(provider api.Provider, toolExecutor tools.ToolExecutor, telemetrySink telemetry.TelemetrySink) *ConversationRuntime {
	return &ConversationRuntime{
		Provider:     provider,
		ToolExecutor: toolExecutor,
		Telemetry:    telemetrySink,
		Session:      NewSession(),
	}
}

// RunTurn runs a single turn and returns the rendered output, discarding any
// underlying provider error. It is preserved for callers that only need the
// display text (e.g. the REPL and demo paths); callers that must react to a
// genuine failure (e.g. the one-shot `ember prompt` exit code) should use
// RunTurnResult instead.
func (r *ConversationRuntime) RunTurn(input string) string {
	output, _ := r.RunTurnResult(input)
	return output
}

// RunTurnResult runs a single turn and returns both the rendered output and the
// real error value from the provider (or tool) when the turn genuinely failed.
// The error is plumbed through unchanged — callers exit non-zero on it rather
// than string-matching the rendered "[ollama error] ..." text.
func (r *ConversationRuntime) RunTurnResult(input string) (string, error) {
	r.Telemetry.Record(telemetry.Event{Name: "turn_started", Details: input})

	var output string
	var turnErr error

	if strings.HasPrefix(input, "/tool ") {
		payload := strings.TrimPrefix(input, "/tool ")
		output = r.ToolExecutor.Execute("bash", payload)
		r.Telemetry.Record(telemetry.Event{Name: "tool_executed", Details: output})
	} else {
		response, err := r.Provider.SendMessage(api.MessageRequest{
			Model:  "",
			Prompt: input,
		})
		if err != nil {
			turnErr = err
			output = fmt.Sprintf("[ollama error] %s", err.Error())
			r.Telemetry.Record(telemetry.Event{Name: "provider_error", Details: output})
		} else {
			output = response.Text
			r.Telemetry.Record(telemetry.Event{Name: "provider_completed", Details: output})
		}
	}

	r.Session.AddTurn(SessionTurn{Input: input, Output: output})
	return output, turnErr
}

func (r *ConversationRuntime) TurnCount() int {
	return r.Session.Count()
}

func (r *ConversationRuntime) LastTurn() (SessionTurn, bool) {
	return r.Session.LastTurn()
}
