package tools

import "slices"

const ToolsReference = "github.com/PeterPonyu/emberforge-go/pkg/tools"

var defaultTools = []ToolSpec{
	{Name: "read_file", Description: "Read workspace files"},
	{Name: "grep_search", Description: "Search text across files"},
	{Name: "bash", Description: "Run shell commands"},
	{Name: "ask_user_question", Description: "Create a task-linked clarification request"},
	{Name: "task_create", Description: "Create a tracked task record"},
	{Name: "task_get", Description: "Read a tracked task record"},
	{Name: "task_list", Description: "List tracked task records"},
	{Name: "task_stop", Description: "Stop a tracked task record"},
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
