# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/).

## [Unreleased]

### Added
- Transport-level HTTP timeouts (dial 10 s, TLS 10 s, response-header 60 s) for
  all provider HTTP clients; blanket `Client.Timeout` is intentionally omitted
  so long-running streaming responses are not cancelled mid-stream.
- `LICENSE` (MIT, 2026 PeterPonyu).
- CLI build smoke test (`cmd/ember/smoke_test.go`) that compiles the binary and
  exercises `-help`.

### Changed
- `.gitignore`: added `.omc/` and `/ember` (locally built binary).
- `README.md`: documented `pkg/lsp` as a one-method stub (not a working LSP
  integration) and added a parity-status note for the decision ledger.

## [0.1.0] - 2026-05-31

### Added
- Initial Go port: `prompt`, `repl`, `init`, `doctor`, slash commands,
  `--serve` HTTP/SSE server, and non-interactive demo flow.
- Ollama provider with streaming, thinking-separation, and configurable
  `num_predict` bound.
- Anthropic and xAI (OpenAI-compat) hosted providers with credential routing.
- Dynamic prompt-context sizing, model listing/routing, and multi-turn tool loop.
