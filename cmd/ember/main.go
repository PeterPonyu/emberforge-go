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
	flag.Parse()

	if strings.TrimSpace(*model) != "" {
		os.Setenv("OLLAMA_MODEL", strings.TrimSpace(*model))
		os.Setenv("EMBER_MODEL", strings.TrimSpace(*model))
	}

	if flag.Arg(0) == "doctor" {
		app := system.NewStarterSystemApplication(system.DefaultStarterSystemConfig())
		fmt.Println(system.BuildDoctorReport(app.Report()))
		app.Shutdown()
		return
	}

	rawCommand := strings.TrimSpace(strings.Join(flag.Args(), " "))
	if strings.HasPrefix(rawCommand, "/") {
		app := system.NewStarterSystemApplication(system.DefaultStarterSystemConfig())
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

	// Default: demo mode (original behaviour).
	app := system.NewStarterSystemApplication(system.DefaultStarterSystemConfig())
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
	fmt.Printf("rust anchor: %s\n", report.RustAnchor)
	fmt.Printf("turns: %d\n", report.TurnCount)
	for _, output := range outputs {
		fmt.Println(output)
	}
	fmt.Printf("last route: %s\n", report.LastRoute)
	fmt.Printf("last phases: %v\n", report.LastPhaseHistory)
	fmt.Printf("last turn: %s\n", report.LastTurnInput)
}
