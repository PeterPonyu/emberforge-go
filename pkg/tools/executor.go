package tools

type ToolExecutor interface {
	Execute(toolName string, input string) string
}

type MockToolExecutor struct{}

func (MockToolExecutor) Execute(toolName string, input string) string {
	return "[go tool] " + toolName + " => " + input
}
