package commands

type CommandCategory string

const (
	CommandCategoryCore       CommandCategory = "core"
	CommandCategoryWorkspace  CommandCategory = "workspace"
	CommandCategorySession    CommandCategory = "session"
	CommandCategoryGit        CommandCategory = "git"
	CommandCategoryAutomation CommandCategory = "automation"
)

type CommandSpec struct {
	Name         string
	Description  string
	Category     CommandCategory
	ArgumentHint string
}
