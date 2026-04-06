package commands

import "slices"

const ClaudeCommandsReference = "/home/zeyufu/Desktop/claude-code-src/commands.ts"

var defaultCommands = []CommandSpec{
	{Name: "help", Description: "Show the translated command registry"},
	{Name: "status", Description: "Report starter runtime status"},
	{Name: "model", Description: "Mirror a Rust-style CLI command"},
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
