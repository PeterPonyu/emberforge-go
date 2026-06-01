package tools

// ToolSpec describes a registered tool: its name, human description, a JSON
// Schema describing its input (as a structural map mirroring the Rust
// input_schema), and the permission mode required to invoke it.
type ToolSpec struct {
	Name               string
	Description        string
	InputSchema        map[string]any
	RequiredPermission PermissionMode
}
