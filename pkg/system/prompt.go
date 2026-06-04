package system

import "strings"

// RunPromptOnce executes a single non-interactive agent turn for promptText and
// returns the rendered output, mirroring the Rust reference `ember prompt
// "<text>"` (crates/ember-cli CliAction::Prompt -> run_turn_with_output).
//
// It reuses the application's existing ConversationRuntime rather than building a
// separate engine: the input is routed through the system dispatcher
// (pkg/system/dispatch.go), where a plain prompt is classified as RoutePrompt and
// handed to ConversationRuntime.RunTurn exactly once. The control-sequence engine
// bootstraps on first use, records the turn in the session, and emits telemetry,
// so a one-shot run has the same side effects as a single REPL line.
func RunPromptOnce(app *StarterSystemApplication, promptText string) string {
	return app.Sequence.Handle(strings.TrimSpace(promptText)).Output
}
