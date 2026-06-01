package tools

// PermissionMode mirrors the Rust runtime PermissionMode enum and orders modes
// from least to most privileged so a tool's required mode can be compared
// against the session's active mode.
type PermissionMode int

const (
	// PermissionReadOnly allows read-only tools (read_file, glob, grep, …).
	PermissionReadOnly PermissionMode = iota
	// PermissionWorkspaceWrite allows mutating workspace files (write/edit).
	PermissionWorkspaceWrite
	// PermissionDangerFullAccess allows arbitrary execution (bash, agent, …).
	PermissionDangerFullAccess
)

// String returns the canonical hyphenated label matching the Rust as_str.
func (m PermissionMode) String() string {
	switch m {
	case PermissionReadOnly:
		return "read-only"
	case PermissionWorkspaceWrite:
		return "workspace-write"
	case PermissionDangerFullAccess:
		return "danger-full-access"
	default:
		return "unknown"
	}
}

// Allows reports whether a session running in mode m may invoke a tool that
// requires the given permission. A higher (more privileged) active mode
// satisfies a lower required mode.
func (m PermissionMode) Allows(required PermissionMode) bool {
	return m >= required
}
