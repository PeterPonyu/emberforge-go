package system

import (
	"github.com/zeyufu/emberforge-translations/emberforge-go/pkg/api"
	"github.com/zeyufu/emberforge-translations/emberforge-go/pkg/commands"
	"github.com/zeyufu/emberforge-translations/emberforge-go/pkg/compat"
	"github.com/zeyufu/emberforge-translations/emberforge-go/pkg/lsp"
	"github.com/zeyufu/emberforge-translations/emberforge-go/pkg/plugins"
	"github.com/zeyufu/emberforge-translations/emberforge-go/pkg/runtime"
	"github.com/zeyufu/emberforge-translations/emberforge-go/pkg/server"
	"github.com/zeyufu/emberforge-translations/emberforge-go/pkg/telemetry"
	"github.com/zeyufu/emberforge-translations/emberforge-go/pkg/tools"
)

type StarterSystemApplication struct {
	Config         StarterSystemConfig
	Provider       api.MockProvider
	ToolExecutor   tools.MockToolExecutor
	Telemetry      telemetry.ConsoleTelemetrySink
	Runtime        *runtime.ConversationRuntime
	Plugin         plugins.ExamplePlugin
	PluginRegistry plugins.PluginRegistry
	CommandRegistry commands.CommandRegistry
	ToolRegistry    tools.ToolRegistry
	Server          server.Server
	LSP             lsp.Manager
	Paths           compat.UpstreamPaths
}

func NewStarterSystemApplication(config StarterSystemConfig) *StarterSystemApplication {
	provider := api.MockProvider{}
	toolExecutor := tools.MockToolExecutor{}
	telemetrySink := telemetry.ConsoleTelemetrySink{}
	plugin := plugins.NewExamplePlugin()
	return &StarterSystemApplication{
		Config:          config,
		Provider:        provider,
		ToolExecutor:    toolExecutor,
		Telemetry:       telemetrySink,
		Runtime:         runtime.NewConversationRuntime(provider, toolExecutor, telemetrySink),
		Plugin:          plugin,
		PluginRegistry:  plugins.NewPluginRegistry([]plugins.Plugin{plugin}),
		CommandRegistry: commands.NewCommandRegistry(nil),
		ToolRegistry:    tools.NewToolRegistry(nil),
		Server:          server.New(server.Config{Port: config.Port}),
		LSP:             lsp.Manager{},
		Paths:           compat.DefaultUpstreamPaths(),
	}
}

func (app *StarterSystemApplication) RunDemo() []string {
	return []string{
		app.Runtime.RunTurn(app.Config.Greeting),
		app.Runtime.RunTurn("/tool " + app.Config.ToolDemoCommand),
	}
}

func (app *StarterSystemApplication) Report() StarterSystemReport {
	lastTurn, ok := app.Runtime.LastTurn()
	lastTurnInput := ""
	if ok {
		lastTurnInput = lastTurn.Input
	}
	return StarterSystemReport{
		AppName:           app.Config.AppName,
		CommandCount:      len(app.CommandRegistry.List()),
		ToolCount:         len(app.ToolRegistry.List()),
		PluginCount:       len(app.PluginRegistry.List()),
		ServerDescription: app.Server.Describe(),
		LSPSummary:        app.LSP.Summary(),
		RustAnchor:        app.Paths.EmberRuntimeLib,
		TurnCount:         app.Runtime.TurnCount(),
		LastTurnInput:     lastTurnInput,
	}
}
