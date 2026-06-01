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

		fmt.Fprintln(writer, dispatchReplLine(app, line))
	}
}

// dispatchReplLine routes a single REPL line: starter slash commands take
// priority, then the control-sequence engine handles prompts and tool calls.
func dispatchReplLine(app *StarterSystemApplication, line string) string {
	if strings.HasPrefix(line, "/") {
		if output, ok := ExecuteStarterSlashCommand(app, line); ok {
			return output
		}
	}
	return app.Sequence.Handle(line).Output
}
