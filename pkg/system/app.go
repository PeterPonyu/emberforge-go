package system

import (
	"github.com/PeterPonyu/emberforge-go/pkg/api"
	"github.com/PeterPonyu/emberforge-go/pkg/commands"
	"github.com/PeterPonyu/emberforge-go/pkg/compat"
	"github.com/PeterPonyu/emberforge-go/pkg/lsp"
	"github.com/PeterPonyu/emberforge-go/pkg/plugins"
	"github.com/PeterPonyu/emberforge-go/pkg/runtime"
	"github.com/PeterPonyu/emberforge-go/pkg/server"
	"github.com/PeterPonyu/emberforge-go/pkg/telemetry"
	"github.com/PeterPonyu/emberforge-go/pkg/tools"
)

type StarterSystemApplication struct {
	Config         StarterSystemConfig
	Provider       api.Provider
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
	Lifecycle       *LifecycleTracker
	Dispatcher      SystemDispatcher
	Sequence        *ControlSequenceEngine
	Turn            *TurnEngine
}

func NewStarterSystemApplication(config StarterSystemConfig) *StarterSystemApplication {
	provider := api.NewOllamaProvider("", api.DefaultModel)
	toolExecutor := tools.MockToolExecutor{}
	telemetrySink := telemetry.ConsoleTelemetrySink{}
	plugin := plugins.NewExamplePlugin()
	commandRegistry := commands.NewCommandRegistry(nil)
	toolRegistry := tools.NewToolRegistry(nil)
	lifecycle := NewLifecycleTracker()
	dispatcher := NewSystemDispatcher(commandRegistry, toolRegistry)
	runtimeCore := runtime.NewConversationRuntime(provider, toolExecutor, telemetrySink)
	sequence := NewControlSequenceEngine(runtimeCore, dispatcher, lifecycle, telemetrySink)
	turn := NewTurnEngine(sequence, TurnBudget{MaxTurns: config.MaxTurns, MaxCostUSD: config.MaxCostUSD})
	return &StarterSystemApplication{
		Config:          config,
		Provider:        provider,
		ToolExecutor:    toolExecutor,
		Telemetry:       telemetrySink,
		Runtime:         runtimeCore,
		Plugin:          plugin,
		PluginRegistry:  plugins.NewPluginRegistry([]plugins.Plugin{plugin}),
		CommandRegistry: commandRegistry,
		ToolRegistry:    toolRegistry,
		Server:          server.New(server.Config{Port: config.Port}),
		LSP:             lsp.Manager{},
		Paths:           compat.DefaultUpstreamPaths(),
		Lifecycle:       lifecycle,
		Dispatcher:      dispatcher,
		Sequence:        sequence,
		Turn:            turn,
	}
}

func (app *StarterSystemApplication) RunDemo() []string {
	app.Sequence.Bootstrap()
	return []string{
		app.Sequence.Handle("/" + app.Config.CommandDemoName).Output,
		app.Sequence.Handle(app.Config.Greeting).Output,
		app.Sequence.Handle("/tool " + app.Config.ToolDemoCommand).Output,
	}
}

func (app *StarterSystemApplication) Shutdown() {
	app.Sequence.Shutdown()
}

func (app *StarterSystemApplication) Report() StarterSystemReport {
	lastTurn, ok := app.Runtime.LastTurn()
	lastTurnInput := ""
	if ok {
		lastTurnInput = lastTurn.Input
	}
	lastRecord, hasLastRecord := app.Sequence.LastRecord()
	lastRoute := ""
	lastPhaseHistory := []string{}
	if hasLastRecord {
		lastRoute = string(lastRecord.Route)
		lastPhaseHistory = PhaseStrings(lastRecord.Phases)
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
		HandledRequestCount: len(app.Sequence.RecordsLog),
		LifecycleState:    string(app.Sequence.Lifecycle.Current()),
		LastRoute:         lastRoute,
		LastPhaseHistory:  lastPhaseHistory,
		LastTurnInput:     lastTurnInput,
	}
}
