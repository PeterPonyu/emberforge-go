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

func (r *ConversationRuntime) RunTurn(input string) string {
	r.Telemetry.Record(telemetry.Event{Name: "turn_started", Details: input})

	var output string

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
			output = fmt.Sprintf("[ollama error] %s", err.Error())
			r.Telemetry.Record(telemetry.Event{Name: "provider_error", Details: output})
		} else {
			output = response.Text
			r.Telemetry.Record(telemetry.Event{Name: "provider_completed", Details: output})
		}
	}

	r.Session.AddTurn(SessionTurn{Input: input, Output: output})
	return output
}

func (r *ConversationRuntime) TurnCount() int {
	return r.Session.Count()
}

func (r *ConversationRuntime) LastTurn() (SessionTurn, bool) {
	return r.Session.LastTurn()
}
