package main

import (
	"fmt"

	"github.com/zeyufu/emberforge-translations/emberforge-go/pkg/system"
)

func main() {
	app := system.NewStarterSystemApplication(system.DefaultStarterSystemConfig())
	outputs := app.RunDemo()
	report := app.Report()

	fmt.Println("emberforge-go starter")
	fmt.Printf("system: %s\n", report.AppName)
	fmt.Printf("commands: %d\n", report.CommandCount)
	fmt.Printf("tools: %d\n", report.ToolCount)
	fmt.Printf("plugins: %d\n", report.PluginCount)
	fmt.Printf("plugin valid: %t\n", app.Plugin.Validate())
	fmt.Println(report.ServerDescription)
	fmt.Println(report.LSPSummary)
	fmt.Printf("rust anchor: %s\n", report.RustAnchor)
	fmt.Printf("turns: %d\n", report.TurnCount)
	for _, output := range outputs {
		fmt.Println(output)
	}
	fmt.Printf("last turn: %s\n", report.LastTurnInput)
}
