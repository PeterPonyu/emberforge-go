# Emberforge (Go)

**A local-first coding forge for serious developers.**

Emberforge is an interactive coding assistant that runs in your terminal, powered by local LLMs via Ollama. It provides a rich REPL with tool execution, session management, plugins, and multi-provider support. This is the Go implementation of the Emberforge coding assistant.

## Quick Start

```bash
# Build from source
go build -o ember ./cmd/ember

# Start the REPL (auto-detects Ollama)
./ember

# Or with a specific model
./ember --model qwen3:8b

# One-shot prompt
./ember prompt "explain this codebase"
```

## Features

- **Local-first**: Runs with Ollama -- no API keys needed for local models
- **Cloud fallback**: Anthropic Claude, xAI Grok when API keys are configured
- **Smart routing**: Select models by task complexity
- **Rich slash commands**: `/help`, `/status`, `/model`, `/compact`, `/review`, `/commit`, and more
- **Built-in tools**: bash, file ops, search, and more
- **Session persistence**: Save, resume, export conversations
- **Plugin system**: Extend with custom tools and hooks
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
| **Anthropic** | Claude Opus 4.6, Sonnet 4.6, Haiku 4.5 | `ANTHROPIC_API_KEY` |
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
