package plugins

const RustPluginsReference = "/home/zeyufu/Desktop/emberforge/crates/plugins/src/types.rs"

type ExamplePlugin struct {
	info PluginMetadata
}

func NewExamplePlugin() ExamplePlugin {
	return ExamplePlugin{info: PluginMetadata{
		ID:          "example.bundled",
		Name:        "ExamplePlugin",
		Version:     "0.1.0",
		Description: "A minimal plugin mirroring emberforge::plugins::Plugin",
	}}
}

func (p ExamplePlugin) Metadata() PluginMetadata {
	return p.info
}

func (p ExamplePlugin) Validate() bool {
	return p.info.ID != "" && p.info.Name != ""
}
