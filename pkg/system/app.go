package system

import (
	"fmt"
	"os"
	"time"

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
	Config            StarterSystemConfig
	SessionID         string
	Provider          api.Provider
	ToolExecutor      tools.ToolExecutor
	Telemetry         telemetry.TelemetrySink
	Runtime           *runtime.ConversationRuntime
	Plugin            plugins.ExamplePlugin
	PluginRegistry    plugins.PluginRegistry
	Buddy             *StarterBuddyState
	TaskQuestionStore *TaskQuestionStateStore
	CommandRegistry   commands.CommandRegistry
	ToolRegistry      tools.ToolRegistry
	Server            server.Server
	LSP               lsp.Manager
	Paths             compat.UpstreamPaths
	Lifecycle         *LifecycleTracker
	Dispatcher        SystemDispatcher
	Sequence          *ControlSequenceEngine
	Turn              *TurnEngine
}

// newSessionID derives a process-and-time based session identifier used to
// name telemetry logs and stamp telemetry records.
func newSessionID() string {
	return fmt.Sprintf("session-%d-%d", time.Now().UnixNano(), os.Getpid())
}

// buildTelemetrySink selects the telemetry sink for the configured mode. JSONL
// selection that fails to open its log file degrades gracefully to the console
// sink so the application never aborts on telemetry setup.
func buildTelemetrySink(config StarterSystemConfig, sessionID string) telemetry.TelemetrySink {
	if config.TelemetryMode != TelemetryModeJSONL {
		return telemetry.ConsoleTelemetrySink{Writer: config.ConsoleTelemetryWriter}
	}
	path := config.TelemetryPath
	if path == "" {
		path = telemetry.DefaultJsonlPath(sessionID)
	}
	sink, err := telemetry.NewJsonlTelemetrySink(sessionID, path)
	if err != nil {
		fmt.Fprintf(os.Stderr, "telemetry: falling back to console sink: %v\n", err)
		return telemetry.ConsoleTelemetrySink{Writer: config.ConsoleTelemetryWriter}
	}
	return sink
}

// resolveProvider selects the api.Provider for the configured model using the
// env/.ember credential precedence. Resolution failures (e.g. a hosted model
// with no credentials) degrade gracefully to the Ollama default so the
// application never aborts on provider setup.
func resolveProvider(config StarterSystemConfig) api.Provider {
	settings, err := api.LoadEmberSettings("")
	if err != nil {
		fmt.Fprintf(os.Stderr, "provider: ignoring unreadable .ember settings: %v\n", err)
		settings = api.EmberSettings{}
	}
	provider, err := api.NewProviderForModel(config.Model, settings)
	if err != nil {
		fmt.Fprintf(os.Stderr, "provider: falling back to ollama: %v\n", err)
		return api.NewOllamaProvider("", config.Model)
	}
	return provider
}

func NewStarterSystemApplication(config StarterSystemConfig) *StarterSystemApplication {
	provider := resolveProvider(config)
	toolExecutor := tools.NewRealToolExecutor("")
	sessionID := newSessionID()
	telemetrySink := buildTelemetrySink(config, sessionID)
	plugin := plugins.NewExamplePlugin()
	buddy := NewStarterBuddyState("")
	taskQuestionStore := NewTaskQuestionStateStore("")
	commandRegistry := commands.NewCommandRegistry(nil)
	toolRegistry := tools.NewToolRegistry(nil)
	lifecycle := NewLifecycleTracker()
	dispatcher := NewSystemDispatcher(commandRegistry, toolRegistry)
	runtimeCore := runtime.NewConversationRuntime(provider, toolExecutor, telemetrySink)
	sequence := NewControlSequenceEngine(runtimeCore, dispatcher, lifecycle, telemetrySink)
	turn := NewTurnEngine(sequence, TurnBudget{MaxTurns: config.MaxTurns, MaxCostUSD: config.MaxCostUSD})
	return &StarterSystemApplication{
		Config:            config,
		SessionID:         sessionID,
		Provider:          provider,
		ToolExecutor:      toolExecutor,
		Telemetry:         telemetrySink,
		Runtime:           runtimeCore,
		Plugin:            plugin,
		PluginRegistry:    plugins.NewPluginRegistry([]plugins.Plugin{plugin}),
		Buddy:             buddy,
		TaskQuestionStore: taskQuestionStore,
		CommandRegistry:   commandRegistry,
		ToolRegistry:      toolRegistry,
		Server:            server.New(server.Config{Port: config.Port}),
		LSP:               lsp.Manager{},
		Paths:             compat.DefaultUpstreamPaths(),
		Lifecycle:         lifecycle,
		Dispatcher:        dispatcher,
		Sequence:          sequence,
		Turn:              turn,
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
	if closer, ok := app.Telemetry.(interface{ Close() error }); ok {
		_ = closer.Close()
	}
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
		AppName:             app.Config.AppName,
		CommandCount:        len(app.CommandRegistry.List()),
		ToolCount:           len(app.ToolRegistry.List()),
		PluginCount:         len(app.PluginRegistry.List()),
		ServerDescription:   app.Server.Describe(),
		LSPSummary:          app.LSP.Summary(),
		RuntimeAnchor:       app.Paths.EmberRuntimeLib,
		TurnCount:           app.Runtime.TurnCount(),
		HandledRequestCount: len(app.Sequence.RecordsLog),
		LifecycleState:      string(app.Sequence.Lifecycle.Current()),
		LastRoute:           lastRoute,
		LastPhaseHistory:    lastPhaseHistory,
		LastTurnInput:       lastTurnInput,
	}
}
