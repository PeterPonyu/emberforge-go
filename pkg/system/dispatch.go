package system

import (
	"strings"

	"github.com/PeterPonyu/emberforge-go/pkg/commands"
	"github.com/PeterPonyu/emberforge-go/pkg/tools"
)

type DispatchRoute string

const (
	RouteCommand DispatchRoute = "command"
	RouteTool    DispatchRoute = "tool"
	RoutePrompt  DispatchRoute = "prompt"
)

type DispatchDecision struct {
	Route       DispatchRoute
	Payload     string
	CommandName string
	ToolName    string
}

type SystemDispatcher struct {
	Commands commands.CommandRegistry
	Tools    tools.ToolRegistry
}

func NewSystemDispatcher(commandRegistry commands.CommandRegistry, toolRegistry tools.ToolRegistry) SystemDispatcher {
	return SystemDispatcher{Commands: commandRegistry, Tools: toolRegistry}
}

func (d SystemDispatcher) Dispatch(input string) DispatchDecision {
	trimmed := strings.TrimSpace(input)
	if strings.HasPrefix(trimmed, "/tool ") {
		toolName := ""
		if d.Tools.Has("bash") {
			toolName = "bash"
		}
		return DispatchDecision{Route: RouteTool, ToolName: toolName, Payload: strings.TrimPrefix(trimmed, "/tool ")}
	}
	if strings.HasPrefix(trimmed, "/") {
		withoutSlash := strings.TrimPrefix(trimmed, "/")
		parts := strings.Fields(withoutSlash)
		commandName := ""
		payload := ""
		if len(parts) > 0 {
			commandName = parts[0]
		}
		if len(parts) > 1 {
			payload = strings.Join(parts[1:], " ")
		}
		return DispatchDecision{Route: RouteCommand, CommandName: commandName, Payload: payload}
	}
	return DispatchDecision{Route: RoutePrompt, Payload: trimmed}
}
