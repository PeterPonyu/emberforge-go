package plugins

import "context"

// HookRunResult aggregates the outcomes of every hook fired for one event.
// It mirrors HookRunResult in the Rust reference: a deny is sticky and stops
// further hooks, while allow/warn messages accumulate.
type HookRunResult struct {
	denied   bool
	messages []string
}

// Denied reports whether any hook blocked the tool call.
func (r HookRunResult) Denied() bool { return r.denied }

// Messages returns the accumulated allow/warn/deny messages, in fire order.
func (r HookRunResult) Messages() []string {
	return append([]string(nil), r.messages...)
}

// allowResult builds an allowing result from collected messages.
func allowResult(messages []string) HookRunResult {
	return HookRunResult{denied: false, messages: messages}
}

// HookDispatcher resolves the hooks registered for an event, applies match
// rules for tool events, runs each via its backend executor, and aggregates
// the outcomes. It is the Go analogue of the Rust HookRunner/dispatch loop and
// forms the skeleton the tool-execution pipeline wires into.
type HookDispatcher struct {
	// hooks maps an event to the ordered hook definitions registered for it.
	hooks map[HookEvent][]HookDefinition
}

// NewHookDispatcher builds a dispatcher from a flat list of hook definitions,
// indexing them by event while preserving registration order.
func NewHookDispatcher(defs []HookDefinition) *HookDispatcher {
	hooks := make(map[HookEvent][]HookDefinition)
	for _, def := range defs {
		hooks[def.Event] = append(hooks[def.Event], def)
	}
	return &HookDispatcher{hooks: hooks}
}

// HookDefinitionsFor returns the hooks registered for an event (nil if none).
func (d *HookDispatcher) HookDefinitionsFor(event HookEvent) []HookDefinition {
	return append([]HookDefinition(nil), d.hooks[event]...)
}

// FireEvent dispatches a lifecycle event with no tool context. Lifecycle hooks
// are fire-and-forget for outcomes other than deny; the aggregated result is
// still returned so callers may inspect messages.
func (d *HookDispatcher) FireEvent(ctx context.Context, event HookEvent) HookRunResult {
	return d.Dispatch(ctx, event, HookContext{})
}

// FireEventWithContext dispatches an event carrying a single context key/value
// pair, recorded on the payload via the tool fields for transport parity with
// fire_event_with_context in the Rust reference.
func (d *HookDispatcher) FireEventWithContext(ctx context.Context, event HookEvent, key, value string) HookRunResult {
	return d.Dispatch(ctx, event, HookContext{ToolName: key, ToolInput: value})
}

// Dispatch runs every hook registered for the event against the given context.
// Tool events additionally honour each hook's match rule. The first deny stops
// the loop and returns immediately with denied = true.
func (d *HookDispatcher) Dispatch(ctx context.Context, event HookEvent, hctx HookContext) HookRunResult {
	defs := d.hooks[event]
	if len(defs) == 0 {
		return allowResult(nil)
	}

	var messages []string
	for _, def := range defs {
		if event.IsToolEvent() && def.Match != nil && !def.Match.Matches(hctx.ToolName, hctx.ToolInput) {
			continue
		}

		executor, err := executorFor(def)
		if err != nil {
			// Misconfigured backend: warn but never block the pipeline.
			messages = append(messages, event.String()+" hook skipped: "+err.Error())
			continue
		}

		outcome, message := executor.Execute(ctx, def, event, hctx)
		switch outcome {
		case OutcomeAllow:
			if message != "" {
				messages = append(messages, message)
			}
		case OutcomeWarn:
			if message != "" {
				messages = append(messages, message)
			}
		case OutcomeDeny:
			messages = append(messages, message)
			return HookRunResult{denied: true, messages: messages}
		}
	}

	return allowResult(messages)
}

// RunPreToolUse is a convenience wrapper firing the PreToolUse event for a tool
// call. A denied result means the pipeline must abort the tool execution.
func (d *HookDispatcher) RunPreToolUse(ctx context.Context, toolName, toolInput string) HookRunResult {
	return d.Dispatch(ctx, PreToolUse, HookContext{ToolName: toolName, ToolInput: toolInput})
}

// RunPostToolUse fires the PostToolUse event after a tool has executed.
func (d *HookDispatcher) RunPostToolUse(ctx context.Context, toolName, toolInput, toolOutput string, isError bool) HookRunResult {
	return d.Dispatch(ctx, PostToolUse, HookContext{
		ToolName:   toolName,
		ToolInput:  toolInput,
		ToolOutput: &toolOutput,
		IsError:    isError,
	})
}
