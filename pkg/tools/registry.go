package tools

import "slices"

const ToolsReference = "github.com/PeterPonyu/emberforge-go/pkg/tools"

// defaultTools is the full ported tool registry (EFPORT-7).
var defaultTools = CanonicalToolSpecs()

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

// Find returns the spec registered under toolName, if present.
func (r ToolRegistry) Find(toolName string) (ToolSpec, bool) {
	for _, tool := range r.tools {
		if tool.Name == toolName {
			return tool, true
		}
	}
	return ToolSpec{}, false
}

func GetTools() []ToolSpec {
	return NewToolRegistry(nil).List()
}
