package system

import (
	"fmt"
	"os"
	"strings"

	"github.com/PeterPonyu/emberforge-go/pkg/api"
)

// cloudShortcutRows mirrors the Rust reference's MODEL_ALIAS_ROWS
// (ember-cli/main.rs): the hosted-model aliases surfaced under "/model list" so
// users can switch to a cloud model by a short name. Kept as data, not literals
// scattered through formatting.
var cloudShortcutRows = []struct {
	alias string
	model string
}{
	{"opus", "claude-opus-4-6"},
	{"sonnet", "claude-sonnet-4-6"},
	{"haiku", "claude-haiku-4-5-20251213"},
	{"grok", "grok-3"},
	{"grok-mini", "grok-3-mini"},
}

// executeModelCommand implements the `/model` slash command, mirroring the Rust
// reference's model handling (ember-cli/main.rs:1532-1637):
//   - `/model`            -> current model + available choices
//   - `/model list`       -> real local models (GET /api/tags) + cloud shortcuts
//   - `/model auto`       -> install the Auto routing strategy
//   - `/model hybrid`     -> install the Hybrid routing strategy
//   - `/model <name>`     -> switch the active model for subsequent turns
func executeModelCommand(app *StarterSystemApplication, payload string) string {
	current := app.ActiveModel()
	action := strings.TrimSpace(payload)
	lowered := strings.ToLower(action)

	switch {
	case action == "":
		return renderModelReport(current, app.RoutingStrategy())
	case lowered == "list":
		return "[command] model list:\n" + renderAvailableModels(current)
	case lowered == "auto":
		strategy := api.ParseRoutingStrategy("auto")
		app.SetRoutingStrategy(strategy)
		return "[command] model: routing strategy set to " + strategy.Describe()
	case lowered == "hybrid":
		strategy := api.ParseRoutingStrategy("hybrid")
		app.SetRoutingStrategy(strategy)
		return "[command] model: routing strategy set to " + strategy.Describe()
	default:
		previous := current
		// Resolve aliases (opus/sonnet/...) to their canonical ids before switching.
		resolved := api.ResolveModelAlias(action)
		app.SetActiveModel(resolved)
		return fmt.Sprintf("[command] model: switched %s -> %s for subsequent turns", previous, resolved)
	}
}

// RenderModelsCommand renders the live local model catalog for the `ember
// models` subcommand, mirroring the Rust reference's standalone model listing.
// It reuses the same catalog renderer as `/model list`.
func RenderModelsCommand(app *StarterSystemApplication) string {
	return renderAvailableModels(app.ActiveModel())
}

// renderModelReport renders the current model, the active routing strategy, the
// cloud aliases, and the next-step hints, mirroring format_model_report.
func renderModelReport(current string, strategy api.RoutingStrategy) string {
	lines := []string{
		"[command] model:",
		"  current        " + current,
		"  routing        " + strategy.Describe(),
		"aliases",
	}
	for _, row := range cloudShortcutRows {
		lines = append(lines, fmt.Sprintf("  %-10s %s", row.alias, row.model))
	}
	lines = append(lines,
		"next",
		"  /model           Show the current model and available choices",
		"  /model list      List available models",
		"  /model <name>    Switch models for subsequent turns",
		"  /model auto      Route simpler prompts to a faster model",
		"  /model hybrid    Prefer local for lighter work, cloud for harder work",
	)
	return strings.Join(lines, "\n")
}

// renderAvailableModels queries the live local model catalog and renders it with
// the current-model marker, the cloud shortcuts, and the routing shortcuts,
// mirroring format_available_models_report. Ollama being unreachable degrades
// gracefully to a status note rather than an error.
func renderAvailableModels(current string) string {
	baseURL := os.Getenv("OLLAMA_BASE_URL")
	models, err := api.ListLocalOllamaModels(baseURL)

	lines := []string{"Available models"}
	if err != nil {
		lines = append(lines, "  Ollama state     unreachable ("+truncateForSummary(err.Error(), 60)+")")
	} else if len(models) == 0 {
		lines = append(lines, "  Ollama state     reachable, but no local models were reported")
	} else {
		lines = append(lines, fmt.Sprintf("  Ollama state     reachable - %d local model(s) detected", len(models)))
	}

	if len(models) == 0 {
		lines = append(lines, "  Ollama models    none listed")
	} else {
		lines = append(lines, "  Ollama models")
		for _, model := range models {
			lines = append(lines, "    "+modelMarker(model, current)+" "+model)
		}
	}

	lines = append(lines, "Cloud shortcuts")
	for _, row := range cloudShortcutRows {
		lines = append(lines, fmt.Sprintf("  %s %-10s %s", modelMarker(row.model, current), row.alias, row.model))
	}

	lines = append(lines,
		"Routing shortcuts",
		"  - auto       Route simpler prompts to a faster model",
		"  - hybrid     Prefer local for lighter work, cloud for harder work",
	)
	return strings.Join(lines, "\n")
}

// modelMarker returns "*" when model is the current model, else "-", matching the
// reference's current-model indicator.
func modelMarker(model, current string) string {
	if model == current {
		return "*"
	}
	return "-"
}

// truncateForSummary trims a message to at most n characters with an ellipsis,
// mirroring the reference's truncate_for_summary used in status lines.
func truncateForSummary(s string, n int) string {
	runes := []rune(s)
	if len(runes) <= n {
		return s
	}
	if n <= 1 {
		return string(runes[:n])
	}
	return string(runes[:n-1]) + "…"
}
