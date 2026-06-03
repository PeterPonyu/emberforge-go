# Emberforge (Go)

**Local-first terminal tooling for language-model workflows.**

Emberforge is a terminal coding tool that works with local models through Ollama and can use hosted providers when configured. It includes tool execution, session management, plugins, and multiple provider backends. This repository contains the Go implementation.

> **Note:** The Go port is a work in progress. The interactive REPL (`repl`, or the default when launched from a terminal) and the `init` scaffolder are now wired up; the one-shot `prompt` command from the Rust reference is still pending. The binary supports `repl`, `init`, `doctor`, slash commands, an HTTP/SSE server (`--serve`), and a non-interactive demo run (see [Quick Start](#quick-start)).

## Quick Start

```bash
# Build from source
go build -o ember ./cmd/ember

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

> Running `./ember` with no command drops into the interactive REPL when stdin
> is a terminal, and otherwise runs the non-interactive demo flow. The one-shot
> `./ember prompt "..."` command from the Rust reference is still planned; see
> issue #9 for tracking.

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

## Model Support

| Provider | Models | Auth |
| --- | --- | --- |
| **Ollama** (local) | qwen3, llama3, gemma3, mistral, deepseek-r1, phi4, and many more | None needed |
| **Anthropic** | Claude Opus, Sonnet, and Haiku families | `ANTHROPIC_API_KEY` |
| **xAI** | Grok 3, Grok 3 Mini | `XAI_API_KEY` |

## Configuration

Emberforge reads configuration from (in order of priority):

1. `.ember.json` (project config)
2. `.ember/settings.json` (project settings)
3. `~/.emberforge/settings.json` (user settings)

Environment variables:

- `EMBER_CONFIG_HOME` -- override config directory
- `OLLAMA_BASE_URL` -- custom Ollama endpoint (default: `http://localhost:11434`)
- `ANTHROPIC_API_KEY` -- Anthropic API credentials
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

## License

MIT
