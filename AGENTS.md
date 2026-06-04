# AGENTS.md — emberforge-go

Operating contract for AI agents working in or driving the **Go** port of
emberforge. Keep it factual to this repository; do not assume features from the
Rust reference exist here unless verified in `pkg/`.

## What this is

A terminal coding tool for language-model workflows. It runs against local
models via **Ollama** by default and can use hosted **Anthropic** / **xAI**
providers when credentials are present. This module is a Go implementation
(module path `github.com/PeterPonyu/emberforge-go`, Go 1.22+).

## Install / build

```bash
# Install the `ember` binary onto your PATH via the Go toolchain
go install github.com/PeterPonyu/emberforge-go/cmd/ember@latest

# ...or build from a checkout (produces a local ./ember binary)
go build -o ember ./cmd/ember
```

There is no published OS package; distribution is via `go install` or building
from source. Requires a Go toolchain (CI pins Go 1.22; tested locally on newer).

## Direct loop (one-shot, non-interactive)

The direct loop is a single agent turn that runs and exits — use this when
driving the tool programmatically rather than interactively:

```bash
ember prompt "explain goroutines in one paragraph"
```

- `ember prompt "<text>"` runs **exactly one** turn through the existing
  conversation runtime (`pkg/runtime` `ConversationRuntime.RunTurn`), routed via
  the system dispatcher (`pkg/system/dispatch.go`, `RoutePrompt`), then prints the
  result and exits. Entry point: `cmd/ember/main.go` → `system.RunPromptOnce`
  (`pkg/system/prompt.go`).
- An empty prompt prints `usage: ember prompt "<text>"` and exits with code `2`.
- A turn calls the configured provider. With no local Ollama and no hosted
  credentials, it prints a graceful `[ollama error] ...` string and exits `0`
  (the dispatch/turn wiring still ran). Telemetry lines are written to stdout in
  the default `console` mode; the final line is the turn output.
- There is currently **no** `--output json` mode; output is plain text.

Select the model/provider with the global `--model` flag (before the subcommand):

```bash
ember --model sonnet prompt "..."    # Anthropic, needs ANTHROPIC_API_KEY
ember --model grok-3 prompt "..."    # xAI, needs XAI_API_KEY
ember --model qwen3:8b prompt "..."  # Ollama (local, no key)
```

Other entry points: `ember repl` (interactive), `ember init` (scaffolder),
`ember doctor` (diagnostics), `ember /<command>` (one-shot slash command),
`ember --serve :8080` (HTTP/SSE server), and `ember` with no args (REPL on a TTY,
otherwise a demo flow).

## Providers + required env vars

Provider selection lives in `pkg/api` (`NewProviderForModel` → `DetectProviderKind`):

| Provider | When selected | Credentials |
| --- | --- | --- |
| Ollama (default) | unknown model id and no hosted credentials | none |
| Anthropic | model alias `opus`/`sonnet`/`haiku`, or an Anthropic key is set | `ANTHROPIC_API_KEY` or `ANTHROPIC_AUTH_TOKEN`, or `anthropicApiKey` in `.ember/settings.json` |
| xAI | model alias `grok`/`grok-3`/`grok-mini`/`grok-3-mini`/`grok-2`, or an xAI key is set | `XAI_API_KEY`, or `xaiApiKey` in `.ember/settings.json` |

Hosted-model requests with no matching credentials fall back to Ollama (logged,
non-fatal). Other env vars: `OLLAMA_BASE_URL` (default `http://localhost:11434`),
`OLLAMA_MODEL` / `EMBER_MODEL` (default Ollama model), `EMBER_CONFIG_HOME`
(config dir override). Credentials in `.ember/settings.json` are read from the
**current working directory** as a fallback when env vars are unset.

## Tests

The full suite is what CI runs (`.github/workflows/ci.yml`):

```bash
go mod verify
go vet ./...
go build ./...
go test ./...
```

The direct-loop subcommand is covered by `pkg/system/prompt_test.go`. The parity
replay test (`pkg/server`) skips gracefully when its fixture is absent, so
`go test ./...` passes without extra setup. No network is required for the suite.

## Repository layout

```text
cmd/ember/      CLI entry point (subcommand routing: prompt, repl, init, doctor, --serve)
pkg/api/        Provider routing — Ollama, Anthropic, OpenAI-compat (xAI); auth + model aliases
pkg/commands/   Slash command definitions and registry
pkg/compat/     Compatibility layer and upstream path resolution
pkg/lsp/        Language Server Protocol manager
pkg/plugins/    Plugin system: metadata, validation, hook runner, dispatcher
pkg/runtime/    ConversationRuntime (RunTurn), session state
pkg/server/     HTTP/SSE server + session store; parity replay test
pkg/system/     App lifecycle, config, dispatch, control sequence, turn engine, REPL, prompt one-shot
pkg/telemetry/  Console + JSONL telemetry sinks
pkg/tools/      Built-in tool specs, executor, registry, permissions
```

## Rules for agents

- Reuse the existing runtime (`ConversationRuntime.RunTurn`) and dispatch
  (`SystemDispatcher`); do not build a parallel agent engine.
- Do not commit the prebuilt `./ember` binary, `.omc/`, or other build/junk
  artifacts. Commit source, docs, and tests only.
- Verify before claiming: run the test commands above and paste real output.
- Keep docs truthful — do not advertise providers or flags that aren't wired up.
