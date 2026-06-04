package system

import "io"

// TelemetryMode selects how the application records telemetry events.
type TelemetryMode string

const (
	// TelemetryModeConsole prints events to stdout (the original behaviour).
	TelemetryModeConsole TelemetryMode = "console"
	// TelemetryModeJSONL appends structured events to a JSONL log under
	// $EMBER_CONFIG_HOME/telemetry/<session>.jsonl.
	TelemetryModeJSONL TelemetryMode = "jsonl"
)

type StarterSystemConfig struct {
	AppName         string
	Port            int
	CommandDemoName string
	Greeting        string
	ToolDemoCommand string
	MaxTurns        int
	MaxCostUSD      float64
	// Model selects the model/alias the provider-detection layer routes on.
	// Empty falls back to the Ollama default model.
	Model string
	// TelemetryMode selects the telemetry sink. Defaults to console.
	TelemetryMode TelemetryMode
	// TelemetryPath overrides the JSONL log path when TelemetryMode is JSONL.
	// When empty, the default $EMBER_CONFIG_HOME/telemetry/<session>.jsonl
	// location is used.
	TelemetryPath string
	// ConsoleTelemetryWriter overrides where the console telemetry sink writes.
	// When nil, the sink writes to os.Stdout (the original behaviour). One-shot
	// prompt mode sets this to os.Stderr so stdout carries only the model answer.
	ConsoleTelemetryWriter io.Writer
}

func DefaultStarterSystemConfig() StarterSystemConfig {
	return StarterSystemConfig{
		AppName:         "emberforge-go system",
		Port:            8080,
		CommandDemoName: "help",
		Greeting:        "hello from go system",
		ToolDemoCommand: "printf hello",
		MaxTurns:        16,
		MaxCostUSD:      1.0,
		TelemetryMode:   TelemetryModeConsole,
	}
}
