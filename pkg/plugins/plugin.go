package plugins

const PluginsReference = "github.com/PeterPonyu/emberforge-go/pkg/plugins"

type ExamplePlugin struct {
	info PluginMetadata
}

func NewExamplePlugin() ExamplePlugin {
	return ExamplePlugin{info: PluginMetadata{
		ID:          "example.bundled",
		Name:        "ExamplePlugin",
		Version:     "0.1.0",
		Description: "A minimal bundled example plugin",
	}}
}

func (p ExamplePlugin) Metadata() PluginMetadata {
	return p.info
}

func (p ExamplePlugin) Validate() bool {
	return p.info.ID != "" && p.info.Name != ""
}
