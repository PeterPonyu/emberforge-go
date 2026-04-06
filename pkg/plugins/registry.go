package plugins

type PluginRegistry struct {
	plugins []Plugin
}

func NewPluginRegistry(plugins []Plugin) PluginRegistry {
	return PluginRegistry{plugins: plugins}
}

func (r PluginRegistry) List() []Plugin {
	return append([]Plugin(nil), r.plugins...)
}

func GetPlugins() []Plugin {
	return NewPluginRegistry([]Plugin{NewExamplePlugin()}).List()
}
