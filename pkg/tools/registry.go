package tools

import "slices"

const ClaudeToolsReference = "/home/zeyufu/Desktop/claude-code-src/tools.ts"

var defaultTools = []ToolSpec{
	{Name: "read_file", Description: "Read workspace files"},
	{Name: "grep_search", Description: "Search text across files"},
	{Name: "bash", Description: "Run shell commands"},
}

type ToolRegistry struct {
	tools []ToolSpec
}

func NewToolRegistry(tools []ToolSpec) ToolRegistry {
	if tools == nil {
		tools = defaultTools
	}
	return ToolRegistry{tools: slices.Clone(tools)}
}

func (r ToolRegistry) List() []ToolSpec {
	return slices.Clone(r.tools)
}

func (r ToolRegistry) Has(toolName string) bool {
	for _, tool := range r.tools {
		if tool.Name == toolName {
			return true
		}
	}
	return false
}

func GetTools() []ToolSpec {
	return NewToolRegistry(nil).List()
}
