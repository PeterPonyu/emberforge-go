package runtime

import (
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"

	"github.com/PeterPonyu/emberforge-go/pkg/api"
	"github.com/PeterPonyu/emberforge-go/pkg/telemetry"
	"github.com/PeterPonyu/emberforge-go/pkg/tools"
)

const RustRuntimeReference = "github.com/PeterPonyu/emberforge-go/pkg/runtime"

const (
	// maxIterationsEnv overrides the agentic loop's per-turn iteration bound.
	// A positive integer takes precedence over defaultMaxIterations.
	maxIterationsEnv = "EMBER_MAX_ITERATIONS"

	// defaultMaxIterations bounds how many model<->tool round-trips a single
	// turn may take before the loop aborts, mirroring the Rust reference's
	// max_iterations (crates/runtime/src/conversation.rs). It prevents a model
	// that keeps requesting tools from looping forever.
	defaultMaxIterations = 25
)

type ConversationRuntime struct {
	Provider     api.Provider
	ToolExecutor tools.ToolExecutor
	Telemetry    telemetry.TelemetrySink
	Session      *Session
	// ToolRegistry supplies the tool specs advertised to the model (the `tools`
	// array) and backs permission gating during the agentic loop. Defaults to
	// the canonical registry.
	ToolRegistry tools.ToolRegistry
	// PermissionMode is the active session permission mode the agentic loop runs
	// tools under. Defaults to danger-full-access so the agent can use bash and
	// other privileged tools; a lower mode genuinely denies them via the
	// executor's permission check.
	PermissionMode tools.PermissionMode
	// MaxIterations bounds the agentic loop. Zero falls back to the
	// env-override-or-default resolved by maxIterations().
	MaxIterations int
	// Stream, when non-nil, receives assistant text deltas as they arrive during
	// the agentic loop so callers can surface output incrementally. Nil buffers.
	Stream io.Writer
	// RoutingStrategy selects the model for each turn from the query's estimated
	// complexity. The zero value is a Fixed strategy that defers to the provider's
	// configured model, preserving the original single-model behavior. The
	// `/model auto` and `/model hybrid` commands install a multi-model strategy.
	RoutingStrategy api.RoutingStrategy
	// ProviderForModel, when non-nil, returns the provider that should serve a
	// routed model. It lets Auto/Hybrid routing cross provider boundaries (e.g.
	// hybrid's cloud leg) correctly. When nil the routed model is still applied
	// via the request's Model field against the existing provider (sufficient for
	// same-provider routing such as Auto's local fast/capable split).
	ProviderForModel func(model string) api.Provider
}

func NewConversationRuntime(provider api.Provider, toolExecutor tools.ToolExecutor, telemetrySink telemetry.TelemetrySink) *ConversationRuntime {
	return &ConversationRuntime{
		Provider:       provider,
		ToolExecutor:   toolExecutor,
		Telemetry:      telemetrySink,
		Session:        NewSession(),
		ToolRegistry:   tools.NewToolRegistry(nil),
		PermissionMode: tools.PermissionDangerFullAccess,
	}
}

// maxIterations resolves the active iteration bound: an explicit MaxIterations
// field wins, then a positive EMBER_MAX_ITERATIONS env override, then the
// default. Mirrors the Rust reference's configurable max_iterations.
func (r *ConversationRuntime) maxIterations() int {
	if r.MaxIterations > 0 {
		return r.MaxIterations
	}
	if raw := strings.TrimSpace(os.Getenv(maxIterationsEnv)); raw != "" {
		if v, err := strconv.Atoi(raw); err == nil && v > 0 {
			return v
		}
	}
	return defaultMaxIterations
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
//
// When the configured provider supports structured tool-calling (api.ChatProvider)
// the turn runs the multi-turn agentic loop (model -> tools -> model -> ...),
// mirroring the Rust reference's `'conversation` loop. Providers without that
// capability — and explicit `/tool` invocations — fall back to the original
// single-shot path. The error is plumbed through unchanged so callers exit
// non-zero rather than string-matching the rendered "[ollama error] ..." text.
func (r *ConversationRuntime) RunTurnResult(input string) (string, error) {
	r.Telemetry.Record(telemetry.Event{Name: "turn_started", Details: input})

	if payload, ok := strings.CutPrefix(input, "/tool "); ok {
		output := r.ToolExecutor.Execute("bash", payload)
		r.Telemetry.Record(telemetry.Event{Name: "tool_executed", Details: output})
		r.Session.AddTurn(SessionTurn{Input: input, Output: output})
		return output, nil
	}

	// Resolve the per-turn model and provider from the routing strategy. For a
	// Fixed/zero strategy this is a no-op (empty model, existing provider).
	routedModel := r.RoutingStrategy.SelectModel(input)
	provider := r.Provider
	if routedModel != "" && r.ProviderForModel != nil {
		if p := r.ProviderForModel(routedModel); p != nil {
			provider = p
		}
	}

	if chatProvider, ok := provider.(api.ChatProvider); ok {
		return r.runAgenticLoop(chatProvider, input, routedModel)
	}

	return r.runSingleShot(provider, input)
}

// runSingleShot is the original single-turn path for providers that do not
// implement api.ChatProvider. The provider is supplied by the caller so routing
// can substitute a per-turn provider.
func (r *ConversationRuntime) runSingleShot(provider api.Provider, input string) (string, error) {
	var output string
	var turnErr error
	response, err := provider.SendMessage(api.MessageRequest{Model: "", Prompt: input})
	if err != nil {
		turnErr = err
		output = fmt.Sprintf("[ollama error] %s", err.Error())
		r.Telemetry.Record(telemetry.Event{Name: "provider_error", Details: output})
	} else {
		output = response.Text
		r.Telemetry.Record(telemetry.Event{Name: "provider_completed", Details: output})
	}
	r.Session.AddTurn(SessionTurn{Input: input, Output: output})
	return output, turnErr
}

// runAgenticLoop drives the multi-turn tool loop for a single user turn. It
// seeds the conversation with the agent system prompt and the user message,
// then repeatedly asks the model for a turn: if the model returns tool calls it
// executes each via the existing tool executor (permission-gated), appends the
// results as `tool` role messages, and re-sends — until the model returns no
// tool calls or the iteration bound is exceeded. Mirrors the Rust reference's
// `'conversation` loop (crates/runtime/src/conversation.rs:210-258).
func (r *ConversationRuntime) runAgenticLoop(provider api.ChatProvider, input, routedModel string) (string, error) {
	messages := []api.ChatMessage{
		{Role: "system", Content: api.BuildSystemPrompt()},
		{Role: "user", Content: input},
	}
	toolDefs := toolDefinitions(r.ToolRegistry)
	limit := r.maxIterations()

	var finalText string
	for iteration := 1; ; iteration++ {
		if iteration > limit {
			output := fmt.Sprintf("[agent error] conversation loop exceeded the maximum number of iterations (%d)", limit)
			err := fmt.Errorf("conversation loop exceeded the maximum number of iterations (%d)", limit)
			r.Telemetry.Record(telemetry.Event{Name: "provider_error", Details: output})
			r.Session.AddTurn(SessionTurn{Input: input, Output: output})
			return output, err
		}

		resp, err := provider.Chat(api.ChatRequest{
			Model:    routedModel,
			Messages: messages,
			Tools:    toolDefs,
			Stream:   r.Stream,
		})
		if err != nil {
			output := fmt.Sprintf("[ollama error] %s", err.Error())
			r.Telemetry.Record(telemetry.Event{Name: "provider_error", Details: output})
			r.Session.AddTurn(SessionTurn{Input: input, Output: output})
			return output, err
		}

		// Append the assistant turn (text + any tool calls) to the conversation
		// so the model sees its own prior request alongside the tool results.
		messages = append(messages, api.ChatMessage{
			Role:      "assistant",
			Content:   resp.Text,
			ToolCalls: resp.ToolCalls,
		})
		if resp.Text != "" {
			finalText = resp.Text
		}

		if len(resp.ToolCalls) == 0 {
			r.Telemetry.Record(telemetry.Event{Name: "provider_completed", Details: finalText})
			r.Session.AddTurn(SessionTurn{Input: input, Output: finalText})
			return finalText, nil
		}

		for _, call := range resp.ToolCalls {
			r.Telemetry.Record(telemetry.Event{Name: "tool_call_started", Details: call.Name})
			result := r.executeToolCall(call)
			r.Telemetry.Record(telemetry.Event{Name: "tool_executed", Details: result})
			messages = append(messages, api.ChatMessage{
				Role:     "tool",
				Content:  result,
				ToolName: call.Name,
			})
		}
	}
}

// executeToolCall runs a single tool call through the executor, respecting
// permission gating when the executor supports it (PermissionedToolExecutor),
// and falling back to plain Execute otherwise. The structured arguments are
// bridged to the executor's input contract via tools.EncodeToolInput.
func (r *ConversationRuntime) executeToolCall(call api.ToolCall) string {
	input := tools.EncodeToolInput(call.Name, call.Arguments)
	if gated, ok := r.ToolExecutor.(tools.PermissionedToolExecutor); ok {
		output, _ := gated.ExecuteWithPermission(r.ToolRegistry, call.Name, input, r.PermissionMode)
		return output
	}
	return r.ToolExecutor.Execute(call.Name, input)
}

// toolDefinitions converts the registry's tool specs into the provider-agnostic
// ToolDefinition list advertised to the model. Schemas are taken verbatim from
// the registered specs — never hand-written here.
func toolDefinitions(registry tools.ToolRegistry) []api.ToolDefinition {
	specs := registry.List()
	defs := make([]api.ToolDefinition, 0, len(specs))
	for _, spec := range specs {
		defs = append(defs, api.ToolDefinition{
			Name:        spec.Name,
			Description: spec.Description,
			Parameters:  spec.InputSchema,
		})
	}
	return defs
}

// StreamsOutput reports whether a prompt turn will surface its assistant text
// incrementally to r.Stream (rather than only returning it). This is true only
// when a stream sink is configured AND the provider supports the structured
// chat loop that does the streaming. Callers use it to avoid re-printing text
// that has already been streamed to the terminal.
func (r *ConversationRuntime) StreamsOutput() bool {
	if r.Stream == nil {
		return false
	}
	_, ok := r.Provider.(api.ChatProvider)
	return ok
}

func (r *ConversationRuntime) TurnCount() int {
	return r.Session.Count()
}

func (r *ConversationRuntime) LastTurn() (SessionTurn, bool) {
	return r.Session.LastTurn()
}
