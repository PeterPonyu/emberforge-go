# Emberforge (Go)

**Local-first terminal tooling for language-model workflows.**

Emberforge is a terminal coding tool that works with local models through Ollama and can use hosted providers when configured. It includes a REPL, tool execution, session management, plugins, and multiple provider backends. This repository contains the Go implementation.

## Quick Start

```bash
# Build from source
go build -o ember ./cmd/ember

# Start the REPL (auto-detects Ollama)
./ember

# Or with a specific model
./ember --model qwen3:8b

# Run diagnostics
./ember doctor

# One-shot prompt
./ember prompt "explain this codebase"
```

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
3. `~/.ember/settings.json` (user settings)

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

# Run
./ember
```

## License

MIT
