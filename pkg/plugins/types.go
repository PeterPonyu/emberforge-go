package plugins

type PluginMetadata struct {
	ID          string
	Name        string
	Version     string
	Description string
}

type Plugin interface {
	Metadata() PluginMetadata
	Validate() bool
}
