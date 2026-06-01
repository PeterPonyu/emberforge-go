package tools

// schema is a small helper that builds a JSON-Schema object map for a tool's
// input, keeping the spec definitions compact and consistent with the Rust
// canonical specs (object type, declared properties, required keys).
func schema(properties map[string]any, required ...string) map[string]any {
	if properties == nil {
		properties = map[string]any{}
	}
	req := make([]any, 0, len(required))
	for _, r := range required {
		req = append(req, r)
	}
	return map[string]any{
		"type":                 "object",
		"properties":           properties,
		"required":             req,
		"additionalProperties": false,
	}
}

// CanonicalToolSpecs returns the full ported tool registry: bash, read_file,
// write_file, edit_file, glob_search, grep_search, web, notebook, agent, and
// skill. Each carries an input schema and the permission mode required to run
// it, mirroring the Rust mvp_tool_specs permission routing.
func CanonicalToolSpecs() []ToolSpec {
	return []ToolSpec{
		{
			Name:        "bash",
			Description: "Execute a shell command in the current workspace.",
			InputSchema: schema(map[string]any{
				"command":     map[string]any{"type": "string"},
				"timeout":     map[string]any{"type": "integer", "minimum": 1},
				"description": map[string]any{"type": "string"},
			}, "command"),
			RequiredPermission: PermissionDangerFullAccess,
		},
		{
			Name:        "read_file",
			Description: "Read a text file from the workspace.",
			InputSchema: schema(map[string]any{
				"path":   map[string]any{"type": "string"},
				"offset": map[string]any{"type": "integer", "minimum": 0},
				"limit":  map[string]any{"type": "integer", "minimum": 1},
			}, "path"),
			RequiredPermission: PermissionReadOnly,
		},
		{
			Name:        "write_file",
			Description: "Write a text file in the workspace.",
			InputSchema: schema(map[string]any{
				"path":    map[string]any{"type": "string"},
				"content": map[string]any{"type": "string"},
			}, "path", "content"),
			RequiredPermission: PermissionWorkspaceWrite,
		},
		{
			Name:        "edit_file",
			Description: "Replace text in a workspace file.",
			InputSchema: schema(map[string]any{
				"path":        map[string]any{"type": "string"},
				"old_string":  map[string]any{"type": "string"},
				"new_string":  map[string]any{"type": "string"},
				"replace_all": map[string]any{"type": "boolean"},
			}, "path", "old_string", "new_string"),
			RequiredPermission: PermissionWorkspaceWrite,
		},
		{
			Name:        "glob_search",
			Description: "Find files by glob pattern.",
			InputSchema: schema(map[string]any{
				"pattern": map[string]any{"type": "string"},
				"path":    map[string]any{"type": "string"},
			}, "pattern"),
			RequiredPermission: PermissionReadOnly,
		},
		{
			Name:        "grep_search",
			Description: "Search file contents with a regex pattern.",
			InputSchema: schema(map[string]any{
				"pattern": map[string]any{"type": "string"},
				"path":    map[string]any{"type": "string"},
				"glob":    map[string]any{"type": "string"},
			}, "pattern"),
			RequiredPermission: PermissionReadOnly,
		},
		{
			Name:        "web",
			Description: "Fetch a URL or search the web for current information.",
			InputSchema: schema(map[string]any{
				"url":    map[string]any{"type": "string"},
				"query":  map[string]any{"type": "string"},
				"prompt": map[string]any{"type": "string"},
			}),
			RequiredPermission: PermissionReadOnly,
		},
		{
			Name:        "notebook",
			Description: "Replace, insert, or delete a cell in a Jupyter notebook.",
			InputSchema: schema(map[string]any{
				"notebook_path": map[string]any{"type": "string"},
				"cell_id":       map[string]any{"type": "string"},
				"new_source":    map[string]any{"type": "string"},
				"cell_type":     map[string]any{"type": "string", "enum": []any{"code", "markdown"}},
				"edit_mode":     map[string]any{"type": "string", "enum": []any{"replace", "insert", "delete"}},
			}, "notebook_path"),
			RequiredPermission: PermissionWorkspaceWrite,
		},
		{
			Name:        "agent",
			Description: "Launch a specialized agent task and persist its handoff metadata.",
			InputSchema: schema(map[string]any{
				"description":   map[string]any{"type": "string"},
				"prompt":        map[string]any{"type": "string"},
				"subagent_type": map[string]any{"type": "string"},
				"model":         map[string]any{"type": "string"},
			}, "description", "prompt"),
			RequiredPermission: PermissionDangerFullAccess,
		},
		{
			Name:        "skill",
			Description: "Load a local skill definition and its instructions.",
			InputSchema: schema(map[string]any{
				"skill": map[string]any{"type": "string"},
				"args":  map[string]any{"type": "string"},
			}, "skill"),
			RequiredPermission: PermissionReadOnly,
		},
	}
}
