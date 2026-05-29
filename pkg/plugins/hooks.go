package plugins

import "strings"

// HookEvent enumerates the lifecycle and tool-execution events that hooks can
// subscribe to. The wire names (see [HookEvent.String]) are the explicit
// PascalCase identifiers shared across every emberforge port, matching the
// cross-port contract (CROSS_PORT_CONTRACT.md §4) and the Rust reference in
// crates/runtime/src/hooks.rs.
type HookEvent int

// The 17 hook event variants defined by the cross-port contract.
const (
	PreToolUse HookEvent = iota
	PostToolUse
	SessionStart
	SessionEnd
	SubagentStart
	SubagentStop
	CompactStart
	CompactEnd
	ToolError
	PermissionDenied
	ConfigChange
	UserPromptSubmit
	Notification
	PluginLoad
	PluginUnload
	CwdChanged
	FileChanged
)

// hookEventNames maps each event to its stable PascalCase wire name. The order
// mirrors the const block above so the slice index equals the HookEvent value.
var hookEventNames = [...]string{
	"PreToolUse",
	"PostToolUse",
	"SessionStart",
	"SessionEnd",
	"SubagentStart",
	"SubagentStop",
	"CompactStart",
	"CompactEnd",
	"ToolError",
	"PermissionDenied",
	"ConfigChange",
	"UserPromptSubmit",
	"Notification",
	"PluginLoad",
	"PluginUnload",
	"CwdChanged",
	"FileChanged",
}

// AllHookEvents returns every defined hook event, in contract order.
func AllHookEvents() []HookEvent {
	events := make([]HookEvent, len(hookEventNames))
	for i := range hookEventNames {
		events[i] = HookEvent(i)
	}
	return events
}

// String returns the stable PascalCase wire name for the event. Unknown values
// yield "Unknown(<n>)" so logging never silently drops information.
func (e HookEvent) String() string {
	if e >= 0 && int(e) < len(hookEventNames) {
		return hookEventNames[e]
	}
	return "Unknown(" + itoa(int(e)) + ")"
}

// ParseHookEvent resolves a wire name (e.g. "PreToolUse") to its HookEvent.
// The boolean reports whether the name was recognised.
func ParseHookEvent(name string) (HookEvent, bool) {
	for i, n := range hookEventNames {
		if n == name {
			return HookEvent(i), true
		}
	}
	return 0, false
}

// IsToolEvent reports whether the event carries tool context (tool_name and
// tool_input). Matches HookEvent::is_tool_event in the Rust reference.
func (e HookEvent) IsToolEvent() bool {
	switch e {
	case PreToolUse, PostToolUse, ToolError:
		return true
	default:
		return false
	}
}

// HookMatchRule filters which tool calls trigger a hook. It only applies to
// tool events; lifecycle events ignore it. Mirrors HookMatchRule in the Rust
// reference (crates/runtime/src/hooks.rs).
type HookMatchRule struct {
	// ToolNames, when non-empty, restricts the hook to these tool names.
	// An empty slice matches every tool.
	ToolNames []string `json:"tool_names,omitempty"`
	// Commands holds glob-style patterns matched against the tool input
	// (typically a bash command). A trailing '*' is a prefix-style wildcard;
	// otherwise the pattern must appear as a substring. Empty matches all.
	Commands []string `json:"commands,omitempty"`
}

// Matches reports whether the rule fires for the given tool name and input.
// Matching is case-insensitive, consistent with the Rust reference.
func (r HookMatchRule) Matches(toolName, toolInput string) bool {
	if len(r.ToolNames) > 0 {
		matched := false
		for _, name := range r.ToolNames {
			if name == toolName {
				matched = true
				break
			}
		}
		if !matched {
			return false
		}
	}

	if len(r.Commands) > 0 {
		inputLower := strings.ToLower(toolInput)
		for _, pattern := range r.Commands {
			if matchGlob(pattern, inputLower) {
				return true
			}
		}
		return false
	}

	return true
}

// matchGlob implements the contract's minimal glob grammar against an
// already-lowercased input. A pattern ending in '*' matches when the prefix
// occurs anywhere in the input ("rm *" -> contains "rm "); otherwise the whole
// pattern must occur as a substring.
func matchGlob(pattern, inputLower string) bool {
	p := strings.ToLower(pattern)
	if strings.HasSuffix(p, "*") {
		return strings.Contains(inputLower, p[:len(p)-1])
	}
	return strings.Contains(inputLower, p)
}

// itoa is a tiny dependency-free integer formatter used by HookEvent.String so
// the type's Stringer never pulls in strconv for the fast common path.
func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	neg := n < 0
	if neg {
		n = -n
	}
	var buf [20]byte
	i := len(buf)
	for n > 0 {
		i--
		buf[i] = byte('0' + n%10)
		n /= 10
	}
	if neg {
		i--
		buf[i] = '-'
	}
	return string(buf[i:])
}
