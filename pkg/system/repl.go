package system

import (
	"bufio"
	"fmt"
	"io"
	"strings"
)

// ReplPrompt is the prompt rendered before each REPL input line.
const ReplPrompt = "ember> "

// RunStarterRepl runs an interactive read-eval-print loop against app. It reads
// lines from reader, dispatches each through the slash-command handler (falling
// back to the control-sequence engine for prompts and tool calls), and writes
// rendered output to writer. Session messages accumulate in the runtime session
// across turns.
//
// The loop exits gracefully on EOF (Ctrl-D) or when the user enters /exit or
// /quit, returning any non-EOF read error wrapped for the caller.
func RunStarterRepl(app *StarterSystemApplication, reader io.Reader, writer io.Writer) error {
	app.Sequence.Bootstrap()

	// Surface assistant text deltas to the terminal as they arrive during a
	// prompt turn's agentic loop, instead of buffering the whole response.
	app.Runtime.Stream = writer

	fmt.Fprintf(writer, "emberforge-go REPL (session %s)\n", app.SessionID)
	fmt.Fprintln(writer, "Type a prompt, /help for commands, or /exit (Ctrl-D) to quit.")

	scanner := bufio.NewScanner(reader)
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)

	for {
		fmt.Fprint(writer, ReplPrompt)
		if !scanner.Scan() {
			if err := scanner.Err(); err != nil {
				fmt.Fprintln(writer)
				return fmt.Errorf("repl: read input: %w", err)
			}
			// EOF / Ctrl-D: graceful exit.
			fmt.Fprintln(writer)
			fmt.Fprintln(writer, "[repl] goodbye")
			return nil
		}

		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		if line == "/exit" || line == "/quit" {
			fmt.Fprintln(writer, "[repl] goodbye")
			return nil
		}

		output, streamed := dispatchReplLine(app, line)
		if streamed {
			// The assistant text already streamed to writer during the turn;
			// emit only a trailing newline to terminate the line.
			fmt.Fprintln(writer)
		} else {
			fmt.Fprintln(writer, output)
		}
	}
}

// dispatchReplLine routes a single REPL line: starter slash commands take
// priority, then the control-sequence engine handles prompts and tool calls.
// The bool reports whether the output was already streamed to the runtime's
// writer (true only for prompt turns under an active streaming chat provider),
// so the caller can avoid re-printing it.
func dispatchReplLine(app *StarterSystemApplication, line string) (string, bool) {
	if strings.HasPrefix(line, "/") {
		if output, ok := ExecuteStarterSlashCommand(app, line); ok {
			return output, false
		}
	}
	record := app.Sequence.Handle(line)
	if record.Route == RoutePrompt && app.Runtime.StreamsOutput() {
		return "", true
	}
	return record.Output, false
}
