package commands

import "slices"

const CommandsReference = "github.com/PeterPonyu/emberforge-go/pkg/commands"

var defaultCommands = []CommandSpec{
	{Name: "help", Description: "Show the command registry", Category: CommandCategoryCore},
	{Name: "status", Description: "Report runtime status", Category: CommandCategoryCore},
	{Name: "doctor", Description: "Run translated environment diagnostics", Category: CommandCategoryCore, ArgumentHint: "[quick|status]"},
	{Name: "model", Description: "Switch or inspect the active model", Category: CommandCategoryCore, ArgumentHint: "[model|list]"},
	{Name: "questions", Description: "Inspect and answer task-linked questions", Category: CommandCategorySession, ArgumentHint: "[ask <task-id> <text>|pending|answer <question-id> <text>]"},
	{Name: "tasks", Description: "Create and inspect translated background tasks", Category: CommandCategoryAutomation, ArgumentHint: "[create prompt <text>|list|show <task-id>|stop <task-id>]"},
	{Name: "buddy", Description: "Manage the translated companion buddy", Category: CommandCategoryCore, ArgumentHint: "[hatch|rehatch|pet|mute|unmute]"},
	{Name: "compact", Description: "Summarize the current conversation state", Category: CommandCategoryCore},
	{Name: "review", Description: "Review the current workspace changes", Category: CommandCategoryGit, ArgumentHint: "[scope]"},
	{Name: "commit", Description: "Prepare a translated commit summary", Category: CommandCategoryGit},
	{Name: "pr", Description: "Prepare a translated pull request summary", Category: CommandCategoryGit, ArgumentHint: "[context]"},
}

type CommandRegistry struct {
	commands []CommandSpec
}

func NewCommandRegistry(commands []CommandSpec) CommandRegistry {
	if commands == nil {
		commands = defaultCommands
	}
	return CommandRegistry{commands: slices.Clone(commands)}
}

func (r CommandRegistry) List() []CommandSpec {
	return slices.Clone(r.commands)
}

func (r CommandRegistry) Find(name string) (CommandSpec, bool) {
	for _, command := range r.commands {
		if command.Name == name {
			return command, true
		}
	}
	return CommandSpec{}, false
}

func GetCommands() []CommandSpec {
	return NewCommandRegistry(nil).List()
}
