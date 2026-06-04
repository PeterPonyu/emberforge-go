package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"strings"
	"syscall"

	"github.com/PeterPonyu/emberforge-go/pkg/server"
	"github.com/PeterPonyu/emberforge-go/pkg/system"
)

func main() {
	serveAddr := flag.String("serve", "", "address to listen on for HTTP/SSE server (e.g. :8080); if empty, run demo mode")
	model := flag.String("model", "", "model to use with the Ollama provider")
	telemetryMode := flag.String("telemetry", "console", "telemetry sink: console or jsonl (jsonl appends to $EMBER_CONFIG_HOME/telemetry/<session>.jsonl)")
	flag.Parse()

	if strings.TrimSpace(*model) != "" {
		os.Setenv("OLLAMA_MODEL", strings.TrimSpace(*model))
		os.Setenv("EMBER_MODEL", strings.TrimSpace(*model))
	}

	config := system.DefaultStarterSystemConfig()
	if strings.EqualFold(strings.TrimSpace(*telemetryMode), string(system.TelemetryModeJSONL)) {
		config.TelemetryMode = system.TelemetryModeJSONL
	}

	if flag.Arg(0) == "doctor" {
		app := system.NewStarterSystemApplication(config)
		fmt.Println(system.BuildDoctorReport(app.Report()))
		app.Shutdown()
		return
	}

	if flag.Arg(0) == "prompt" {
		promptText := strings.TrimSpace(strings.Join(flag.Args()[1:], " "))
		if promptText == "" {
			fmt.Fprintln(os.Stderr, "usage: ember prompt \"<text>\"")
			os.Exit(2)
		}
		// One-shot prompt mode: stdout must carry ONLY the model answer, so
		// route console telemetry/diagnostics to stderr via the sink writer.
		promptConfig := config
		promptConfig.ConsoleTelemetryWriter = os.Stderr
		app := system.NewStarterSystemApplication(promptConfig)
		output, err := system.RunPromptOnce(app, promptText)
		app.Shutdown()
		if err != nil {
			// A genuine provider/runtime failure: surface it on stderr and exit
			// non-zero so callers can detect it (matches Rust/C++/TS behaviour).
			fmt.Fprintln(os.Stderr, output)
			os.Exit(1)
		}
		fmt.Println(output)
		return
	}

	if flag.Arg(0) == "init" {
		app := system.NewStarterSystemApplication(config)
		output, _ := system.ExecuteStarterSlashCommand(app, "/init "+strings.Join(flag.Args()[1:], " "))
		fmt.Println(output)
		app.Shutdown()
		return
	}

	if flag.Arg(0) == "repl" {
		app := system.NewStarterSystemApplication(config)
		if err := system.RunStarterRepl(app, os.Stdin, os.Stdout); err != nil {
			fmt.Fprintf(os.Stderr, "repl error: %v\n", err)
			app.Shutdown()
			os.Exit(1)
		}
		app.Shutdown()
		return
	}

	rawCommand := strings.TrimSpace(strings.Join(flag.Args(), " "))
	if strings.HasPrefix(rawCommand, "/") {
		app := system.NewStarterSystemApplication(config)
		if output, ok := system.ExecuteStarterSlashCommand(app, rawCommand); ok {
			fmt.Println(output)
			app.Shutdown()
			return
		}
		fmt.Println(app.Sequence.Handle(rawCommand).Output)
		app.Shutdown()
		return
	}

	if *serveAddr != "" {
		store := server.NewSessionStore()
		hs := server.NewHttpServer(*serveAddr, store)
		ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
		defer stop()
		fmt.Printf("emberforge-go HTTP server listening on %s\n", *serveAddr)
		if err := hs.Start(ctx); err != nil {
			fmt.Fprintf(os.Stderr, "server error: %v\n", err)
			os.Exit(1)
		}
		return
	}

	// Default: drop into the interactive REPL when stdin is a terminal,
	// otherwise fall back to the non-interactive demo flow.
	if isInteractiveStdin() {
		app := system.NewStarterSystemApplication(config)
		if err := system.RunStarterRepl(app, os.Stdin, os.Stdout); err != nil {
			fmt.Fprintf(os.Stderr, "repl error: %v\n", err)
			app.Shutdown()
			os.Exit(1)
		}
		app.Shutdown()
		return
	}

	app := system.NewStarterSystemApplication(config)
	outputs := app.RunDemo()
	app.Shutdown()
	report := app.Report()

	fmt.Println("emberforge-go starter")
	fmt.Printf("system: %s\n", report.AppName)
	fmt.Printf("lifecycle: %s\n", report.LifecycleState)
	fmt.Printf("commands: %d\n", report.CommandCount)
	fmt.Printf("tools: %d\n", report.ToolCount)
	fmt.Printf("plugins: %d\n", report.PluginCount)
	fmt.Printf("handled requests: %d\n", report.HandledRequestCount)
	fmt.Printf("plugin valid: %t\n", app.Plugin.Validate())
	fmt.Println(report.ServerDescription)
	fmt.Println(report.LSPSummary)
	fmt.Printf("runtime: %s\n", report.RuntimeAnchor)
	fmt.Printf("turns: %d\n", report.TurnCount)
	for _, output := range outputs {
		fmt.Println(output)
	}
	fmt.Printf("last route: %s\n", report.LastRoute)
	fmt.Printf("last phases: %v\n", report.LastPhaseHistory)
	fmt.Printf("last turn: %s\n", report.LastTurnInput)
}

// isInteractiveStdin reports whether stdin is a character device (a terminal),
// indicating the user can type into an interactive REPL.
func isInteractiveStdin() bool {
	info, err := os.Stdin.Stat()
	if err != nil {
		return false
	}
	return info.Mode()&os.ModeCharDevice != 0
}
