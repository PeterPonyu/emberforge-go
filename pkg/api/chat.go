package api

import "io"

// ChatMessage is one entry in a multi-turn conversation sent to a provider.
// It mirrors the structured conversation messages of the Rust reference
// (crates/runtime conversation.rs): a role plus text content, and — for
// assistant turns — any tool calls the model requested. Tool result turns use
// role "tool" with ToolName set so the provider can correlate the result.
type ChatMessage struct {
	// Role is one of "system", "user", "assistant", or "tool".
	Role string
	// Content is the message text. For assistant turns that only request tools
	// it may be empty; for "tool" turns it carries the tool's output.
	Content string
	// ToolCalls are the tool invocations an assistant turn requested. Empty for
	// non-assistant turns or assistant turns that returned only text.
	ToolCalls []ToolCall
	// ToolName names the tool a "tool" role message reports the result for.
	ToolName string
}

// ToolCall is a single structured tool invocation requested by the model. It is
// parsed from Ollama's native message.tool_calls field (the structured field —
// never string-scraped). Arguments is the decoded JSON object the model passed.
type ToolCall struct {
	// ID optionally correlates a call with its result. Ollama does not always
	// supply one; when absent the runtime derives a stable index-based id.
	ID string
	// Name is the tool name (must match a registered tool spec).
	Name string
	// Arguments is the decoded JSON argument object for the tool.
	Arguments map[string]any
}

// ToolDefinition is a provider-agnostic description of a callable tool, built
// from the existing tool registry's specs (never hand-written schemas). The
// runtime converts tools.ToolSpec into this shape so pkg/api stays independent
// of pkg/tools (no import cycle), and the Ollama provider renders it into the
// native /api/chat `tools` array.
type ToolDefinition struct {
	Name        string
	Description string
	// Parameters is the JSON-Schema object describing the tool's input, taken
	// verbatim from the registered spec.
	Parameters map[string]any
}

// ChatRequest carries one agentic turn's worth of state to a ChatProvider: the
// full accumulated conversation, the available tool definitions, and an optional
// streaming sink. The model is resolved by the provider when Model is empty.
type ChatRequest struct {
	Model    string
	Messages []ChatMessage
	Tools    []ToolDefinition
	// Stream, when non-nil, receives assistant text deltas as they arrive so
	// callers can surface output incrementally. When nil the provider buffers
	// the full response. Tool calls are always aggregated regardless.
	Stream io.Writer
}

// ChatResponse is the result of a single provider turn: the assistant text and
// any tool calls the model requested. Mirrors the Rust reference's assistant
// message (text blocks + ToolUse blocks).
type ChatResponse struct {
	Text      string
	ToolCalls []ToolCall
}

// ChatProvider is implemented by providers that support the structured,
// multi-turn, tool-calling chat used by the agentic runtime loop. Providers
// that only support single-turn text generation implement Provider alone; the
// runtime detects ChatProvider via a type assertion and falls back gracefully.
type ChatProvider interface {
	Chat(request ChatRequest) (ChatResponse, error)
}
