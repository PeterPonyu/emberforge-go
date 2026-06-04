package tools

import (
	"encoding/json"
	"fmt"
)

// PermissionedToolExecutor is a ToolExecutor that can also gate execution behind
// the registry-declared permission for a tool. *RealToolExecutor satisfies it.
// The runtime type-asserts to this interface so the agentic loop respects
// permission gating when available, and falls back to plain Execute otherwise.
type PermissionedToolExecutor interface {
	ToolExecutor
	ExecuteWithPermission(registry ToolRegistry, toolName, input string, activeMode PermissionMode) (string, bool)
}

// EncodeToolInput bridges a structured tool-call argument object (as parsed from
// a model's native tool_calls) into the single-string input format the existing
// Execute / ExecuteWithPermission entrypoints expect. The encoding mirrors each
// tool's documented input contract in real_executor.go; keeping it here (next to
// the executor that defines those formats) preserves cohesion and lets the
// runtime stay agnostic of per-tool quirks.
//
// Argument keys are the JSON-Schema property names declared in the canonical
// tool specs (specs.go) — never hardcoded model- or call-specific values.
func EncodeToolInput(toolName string, args map[string]any) string {
	switch toolName {
	case "bash":
		return argString(args, "command")
	case "read_file":
		return argString(args, "path")
	case "write_file":
		return argString(args, "path") + ":" + argString(args, "content")
	case "edit_file":
		return argString(args, "path") + "\x00" + argString(args, "old_string") + "\x00" + argString(args, "new_string")
	case "glob_search":
		return argString(args, "pattern")
	case "grep_search":
		pattern := argString(args, "pattern")
		if path := argString(args, "path"); path != "" {
			return pattern + "\x00" + path
		}
		return pattern
	default:
		// web, notebook, agent, skill and any future tool: pass the decoded
		// arguments as a compact JSON object so the receipt path is observable.
		if len(args) == 0 {
			return ""
		}
		if encoded, err := json.Marshal(args); err == nil {
			return string(encoded)
		}
		return fmt.Sprintf("%v", args)
	}
}

// argString returns the named argument coerced to a string. Strings pass
// through; other JSON scalars are rendered with %v so numeric/bool arguments
// (e.g. an integer timeout) are not dropped. A missing key yields "".
func argString(args map[string]any, key string) string {
	v, ok := args[key]
	if !ok || v == nil {
		return ""
	}
	if s, ok := v.(string); ok {
		return s
	}
	return fmt.Sprintf("%v", v)
}
