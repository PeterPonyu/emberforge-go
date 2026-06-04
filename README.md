# Emberforge (Go)

**Local-first terminal tooling for language-model workflows.**

Emberforge is a terminal coding tool that works with local models through Ollama and can use hosted providers when configured. It includes tool execution, session management, plugins, and multiple provider backends. This repository contains the Go implementation.

> **Note:** The Go port is a work in progress. The one-shot `prompt` command
> (the non-interactive "direct loop" from the Rust reference), the interactive
> REPL (`repl`, or the default when launched from a terminal), and the `init`
> scaffolder are all wired up. The binary supports `prompt`, `repl`, `init`,
> `doctor`, slash commands, an HTTP/SSE server (`--serve`), and a non-interactive
> demo run (see [Quick Start](#quick-start)).

## Quick Start

```bash
# Install with the Go toolchain (puts the `ember` binary on your PATH)
go install github.com/PeterPonyu/emberforge-go/cmd/ember@latest

# ...or build from source (produces a local ./ember binary)
go build -o ember ./cmd/ember

# Run one non-interactive agent turn (the "direct loop") and exit
./ember prompt "explain goroutines in one paragraph"

# Run diagnostics
./ember doctor

# Run a slash command directly
./ember /help
./ember /status

# Start the HTTP/SSE server
./ember --serve :8080

# Drop into the interactive REPL (also the default when stdin is a terminal)
./ember repl

# Scaffold project files (EMBER.md, .ember.json, .gitignore entries)
./ember init

# Append structured telemetry to $EMBER_CONFIG_HOME/telemetry/<session>.jsonl
./ember --telemetry jsonl repl

# Run the demo flow (used when no command is given and stdin is not a terminal)
./ember

# Select a model for the Ollama provider (applies to the runs above)
./ember --model qwen3:8b
```

> `./ember prompt "<text>"` runs a single non-interactive agent turn through the
> existing conversation runtime (provider call + tool dispatch), prints the
> result, and exits — the Go equivalent of the Rust reference `ember prompt`. A
> turn talks to the configured provider; with no local Ollama running (and no
> hosted credentials), it prints a graceful provider-error string and still exits
> cleanly. Running `./ember` with no command drops into the interactive REPL when
> stdin is a terminal, and otherwise runs the non-interactive demo flow.

## Features

- **Local-first**: Runs with Ollama -- no API keys needed for local models
- **Hosted providers**: Anthropic Claude and xAI Grok when API keys are configured
- **Task-based model selection**: Select models by task complexity
- **Slash commands**: `/help`, `/status`, `/doctor`, `/model`, `/questions`, `/tasks`, `/buddy`, `/compact`, `/review`, `/commit`, `/pr`, and more
- **Tools**: bash, file ops, search, and more
- **Sessions**: Save, resume, export conversations
- **Plugin system**: Includes plugin metadata and validation scaffolding
- **MCP integration**: Connect to Model Context Protocol servers
- **Telemetry**: Session tracing and usage analytics

## Architecture

```text
cmd/
  ember/          CLI entry point

pkg/
  api/            API client -- Ollama, Anthropic, OpenAI-compat provider routing
  commands/       Slash command definitions and registry
  compat/         Compatibility layer and path resolution
  lsp/            Language Server Protocol integration
  plugins/        Plugin system with metadata and validation
  runtime/        Conversation runtime, session state, turn management
  server/         HTTP server infrastructure
  system/         Application lifecycle, config, dispatch, control sequences
  telemetry/      Session tracing, analytics events
  tools/          Built-in tool specs, executor, and registry
```

## Providers

Emberforge routes each turn to one of three provider backends. The provider is
selected per model in `pkg/api` (`NewProviderForModel` → `DetectProviderKind`):

| Provider | Models / aliases | Auth |
| --- | --- | --- |
| **Ollama** (local, default) | any non-aliased model id (e.g. `qwen3:8b`, `llama3`, `gemma3`, `mistral`, `deepseek-r1`, `phi4`) | None needed |
| **Anthropic** | `opus`, `sonnet`, `haiku` (alias to `claude-opus-4-6`, `claude-sonnet-4-6`, `claude-haiku-4-5-20251213`) | `ANTHROPIC_API_KEY` or `ANTHROPIC_AUTH_TOKEN` (or `anthropicApiKey` in `.ember/settings.json`) |
| **xAI** | `grok`, `grok-3`, `grok-mini`/`grok-3-mini`, `grok-2` | `XAI_API_KEY` (or `xaiApiKey` in `.ember/settings.json`) |

Selection rules:

- A **known model alias** routes directly to its provider (e.g. `--model sonnet`
  → Anthropic, `--model grok-3` → xAI).
- For an **unknown model id**, the first available credential wins: Anthropic
  (if an Anthropic key/token is set), then xAI, otherwise **Ollama** (the
  no-credentials default).
- Hosted providers require credentials. If a hosted model is requested with no
  matching credentials, the app logs the reason and **falls back to Ollama** so it
  never aborts on provider setup.

Pick a model for any command (including the direct loop) with `--model`:

```bash
./ember --model sonnet prompt "summarize this repo"   # Anthropic (needs ANTHROPIC_API_KEY)
./ember --model grok-3 prompt "summarize this repo"   # xAI (needs XAI_API_KEY)
./ember --model qwen3:8b prompt "summarize this repo" # Ollama (local, no key)
```

## Configuration

Credential settings are read from `.ember/settings.json` in the current
working directory (the `anthropicApiKey` and `xaiApiKey` fields are consulted
as a fallback when the matching environment variables are unset). The `init`
command scaffolds a project `.ember.json` alongside `EMBER.md` and `.gitignore`
entries.

User-level settings (e.g. a `~/.emberforge/settings.json`) are not yet
supported: the binary reads no settings file from your home directory. The
`~/.emberforge/` directory is used only for state and telemetry (buddy state,
task-question state, and telemetry logs).

Environment variables:

- `EMBER_CONFIG_HOME` -- override config directory
- `OLLAMA_BASE_URL` -- custom Ollama endpoint (default: `http://localhost:11434`)
- `OLLAMA_MODEL` / `EMBER_MODEL` -- default Ollama model when `--model` is unset
- `ANTHROPIC_API_KEY` / `ANTHROPIC_AUTH_TOKEN` -- Anthropic API credentials
- `XAI_API_KEY` -- xAI API credentials

## Development

```bash
# Build
go build -o ember ./cmd/ember

# Run tests
go test ./...

# Run the demo flow
./ember

# Run diagnostics
./ember doctor
```

### Parity replay test fixture

The parity replay test (`pkg/server/parity_replay_test.go`) exercises a recorded
session-lifecycle scenario. It looks for a fixture at
`pkg/server/testdata/scenario_001_session_lifecycle.jsonl`, or at the path given
by the `EMBERFORGE_PARITY_FIXTURE` environment variable:

```bash
# Point the parity replay test at a custom fixture path
EMBERFORGE_PARITY_FIXTURE=/path/to/scenario.jsonl go test ./pkg/server/...
```

When no fixture is present at either location, the test skips gracefully, so a
plain `go test ./...` run will not fail on a missing fixture.

## License

MIT
