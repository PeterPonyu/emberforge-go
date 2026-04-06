package commands

import "slices"

const CommandsReference = "github.com/PeterPonyu/emberforge-go/pkg/commands"

var defaultCommands = []CommandSpec{
	{Name: "help", Description: "Show the command registry"},
	{Name: "status", Description: "Report runtime status"},
	{Name: "model", Description: "Switch or inspect the active model"},
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
