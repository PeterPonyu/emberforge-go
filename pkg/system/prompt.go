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
//
// It returns the rendered output along with the real error from the turn (nil on
// success) so the caller can exit non-zero on a genuine provider failure without
// inspecting the rendered text.
func RunPromptOnce(app *StarterSystemApplication, promptText string) (string, error) {
	record := app.Sequence.Handle(strings.TrimSpace(promptText))
	return record.Output, record.Err
}
