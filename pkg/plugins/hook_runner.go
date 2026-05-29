package plugins

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"runtime"
	"strings"
	"time"
)

// defaultHookTimeout is applied when a HookDefinition leaves TimeoutSecs unset.
// It matches the Rust reference default of 30 seconds.
const defaultHookTimeout = 30 * time.Second

// HookOutcome is the tri-state result of running a single hook, derived from
// the contract's exit-code semantics: 0 = allow, 2 = deny, anything else =
// warn.
type HookOutcome int

const (
	// OutcomeAllow permits the tool call to proceed; an optional message may
	// accompany it (captured stdout / response body).
	OutcomeAllow HookOutcome = iota
	// OutcomeDeny blocks the tool call.
	OutcomeDeny
	// OutcomeWarn allows the tool call but records a diagnostic message.
	OutcomeWarn
)

// HookBackend selects how a hook is executed.
type HookBackend string

const (
	// BackendCommand runs a shell command (os/exec). Exit code drives the
	// outcome.
	BackendCommand HookBackend = "command"
	// BackendHTTP POSTs the JSON payload to a URL. HTTP status drives the
	// outcome.
	BackendHTTP HookBackend = "http"
)

// HookDefinition is a structured, settings.json-style hook declaration. It
// mirrors HookDefinition in the Rust reference (crates/runtime/src/hooks.rs).
type HookDefinition struct {
	// Event selects which lifecycle/tool event fires this hook.
	Event HookEvent `json:"event"`
	// Type selects the execution backend ("command" or "http").
	Type HookBackend `json:"type"`
	// Run is the shell command to execute (Type == BackendCommand).
	Run string `json:"run,omitempty"`
	// URL is the endpoint to POST to (Type == BackendHTTP).
	URL string `json:"url,omitempty"`
	// Headers are extra HTTP headers for the HTTP backend.
	Headers map[string]string `json:"headers,omitempty"`
	// Match optionally filters which tool calls trigger this hook. Only
	// meaningful for tool events.
	Match *HookMatchRule `json:"match,omitempty"`
	// TimeoutSecs bounds execution. Zero means defaultHookTimeout.
	TimeoutSecs uint32 `json:"timeout_secs,omitempty"`
}

// timeout returns the effective execution timeout for the hook.
func (h HookDefinition) timeout() time.Duration {
	if h.TimeoutSecs == 0 {
		return defaultHookTimeout
	}
	return time.Duration(h.TimeoutSecs) * time.Second
}

// HookContext carries the data threaded into a hook payload and environment.
type HookContext struct {
	ToolName  string
	ToolInput string
	// ToolOutput is non-nil only for post-execution / error events.
	ToolOutput *string
	IsError    bool
}

// hookPayload is the JSON document handed to command stdin and HTTP bodies.
// Field layout matches the cross-port contract §4.4 payload structure.
type hookPayload struct {
	HookEventName     string          `json:"hook_event_name"`
	ToolName          string          `json:"tool_name"`
	ToolInput         json.RawMessage `json:"tool_input"`
	ToolInputJSON     string          `json:"tool_input_json"`
	ToolOutput        *string         `json:"tool_output"`
	ToolResultIsError bool            `json:"tool_result_is_error"`
}

// buildPayload renders the JSON payload for an event + context. tool_input is
// embedded as parsed JSON when the input is valid JSON, otherwise wrapped as
// {"raw": <input>} to match parse_tool_input in the Rust reference.
func buildPayload(event HookEvent, ctx HookContext) ([]byte, error) {
	var toolInput json.RawMessage
	if json.Valid([]byte(ctx.ToolInput)) && ctx.ToolInput != "" {
		toolInput = json.RawMessage(ctx.ToolInput)
	} else {
		wrapped, err := json.Marshal(map[string]string{"raw": ctx.ToolInput})
		if err != nil {
			return nil, fmt.Errorf("wrap raw tool input: %w", err)
		}
		toolInput = wrapped
	}

	payload := hookPayload{
		HookEventName:     event.String(),
		ToolName:          ctx.ToolName,
		ToolInput:         toolInput,
		ToolInputJSON:     ctx.ToolInput,
		ToolOutput:        ctx.ToolOutput,
		ToolResultIsError: ctx.IsError,
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("marshal hook payload: %w", err)
	}
	return body, nil
}

// HookExecutor runs a single hook definition for a given event/context and
// reports the resulting outcome and message.
type HookExecutor interface {
	Execute(ctx context.Context, def HookDefinition, event HookEvent, hctx HookContext) (HookOutcome, string)
}

// CommandExecutor runs hooks via the system shell using os/exec. It implements
// the contract's exit-code semantics: 0 allow, 2 deny, anything else warn.
type CommandExecutor struct{}

// Execute runs the command backend. The JSON payload is piped on stdin and key
// fields are also exposed as HOOK_* environment variables for shell scripts.
func (CommandExecutor) Execute(ctx context.Context, def HookDefinition, event HookEvent, hctx HookContext) (HookOutcome, string) {
	payload, err := buildPayload(event, hctx)
	if err != nil {
		return OutcomeWarn, fmt.Sprintf("%s hook payload build failed: %v", event, err)
	}

	cmd := shellCommand(ctx, def.Run)
	cmd.Stdin = bytes.NewReader(payload)
	cmd.Env = append(os.Environ(),
		"HOOK_EVENT="+event.String(),
		"HOOK_TOOL_NAME="+hctx.ToolName,
		"HOOK_TOOL_INPUT="+hctx.ToolInput,
		"HOOK_TOOL_IS_ERROR="+boolEnv(hctx.IsError),
	)
	if hctx.ToolOutput != nil {
		cmd.Env = append(cmd.Env, "HOOK_TOOL_OUTPUT="+*hctx.ToolOutput)
	}

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	runErr := cmd.Run()
	out := strings.TrimSpace(stdout.String())
	errOut := strings.TrimSpace(stderr.String())

	if runErr == nil {
		return OutcomeAllow, out
	}

	exitErr, ok := runErr.(*exec.ExitError)
	if !ok {
		// Failed to start, timeout, or signal: warn but allow.
		return OutcomeWarn, fmt.Sprintf("%s hook `%s` failed to run for `%s`: %v", event, def.Run, hctx.ToolName, runErr)
	}

	switch code := exitErr.ExitCode(); code {
	case 2:
		if out == "" {
			out = fmt.Sprintf("%s hook denied tool `%s`", event, hctx.ToolName)
		}
		return OutcomeDeny, out
	default:
		return OutcomeWarn, formatWarning(def.Run, code, out, errOut)
	}
}

// HTTPExecutor runs hooks via an HTTP POST of the JSON payload. To preserve the
// contract's tri-state allow/deny/warn semantics across the HTTP boundary it
// maps status codes as: 2xx allow, 403 deny, anything else warn. The response
// body (trimmed) is returned as the message.
type HTTPExecutor struct {
	// Client is the HTTP client used for requests. When nil a default client
	// honouring the hook timeout is constructed per call.
	Client *http.Client
}

// Execute runs the HTTP backend.
func (h HTTPExecutor) Execute(ctx context.Context, def HookDefinition, event HookEvent, hctx HookContext) (HookOutcome, string) {
	payload, err := buildPayload(event, hctx)
	if err != nil {
		return OutcomeWarn, fmt.Sprintf("%s hook payload build failed: %v", event, err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, def.URL, bytes.NewReader(payload))
	if err != nil {
		return OutcomeWarn, fmt.Sprintf("%s hook request build failed for %s: %v", event, def.URL, err)
	}
	req.Header.Set("Content-Type", "application/json")
	for k, v := range def.Headers {
		req.Header.Set(k, v)
	}

	client := h.Client
	if client == nil {
		client = &http.Client{Timeout: def.timeout()}
	}

	resp, err := client.Do(req)
	if err != nil {
		return OutcomeWarn, fmt.Sprintf("%s hook POST to %s failed: %v", event, def.URL, err)
	}
	defer resp.Body.Close()

	var body bytes.Buffer
	// Bound the body we read to keep messages reasonable.
	_, _ = body.ReadFrom(&limitedReader{r: resp.Body, n: 64 * 1024})
	message := strings.TrimSpace(body.String())

	switch {
	case resp.StatusCode >= 200 && resp.StatusCode < 300:
		return OutcomeAllow, message
	case resp.StatusCode == http.StatusForbidden:
		if message == "" {
			message = fmt.Sprintf("%s hook denied tool `%s`", event, hctx.ToolName)
		}
		return OutcomeDeny, message
	default:
		return OutcomeWarn, fmt.Sprintf("%s hook %s returned status %d; allowing tool execution to continue: %s",
			event, def.URL, resp.StatusCode, message)
	}
}

// executorFor selects the backend executor for a hook definition.
func executorFor(def HookDefinition) (HookExecutor, error) {
	switch def.Type {
	case BackendCommand:
		return CommandExecutor{}, nil
	case BackendHTTP:
		return HTTPExecutor{}, nil
	default:
		return nil, fmt.Errorf("unknown hook backend %q", def.Type)
	}
}

// shellCommand builds the platform-appropriate shell invocation for a command
// string, mirroring shell_command in the Rust reference. On Windows it uses
// `cmd /C`; elsewhere `sh -lc` (or `sh <file>` for an existing script path).
func shellCommand(ctx context.Context, command string) *exec.Cmd {
	if runtime.GOOS == "windows" {
		return exec.CommandContext(ctx, "cmd", "/C", command)
	}
	if info, err := os.Stat(command); err == nil && !info.IsDir() {
		return exec.CommandContext(ctx, "sh", command)
	}
	return exec.CommandContext(ctx, "sh", "-lc", command)
}

func boolEnv(b bool) string {
	if b {
		return "1"
	}
	return "0"
}

func formatWarning(command string, code int, stdout, stderr string) string {
	msg := fmt.Sprintf("Hook `%s` exited with status %d; allowing tool execution to continue", command, code)
	switch {
	case stdout != "":
		msg += ": " + stdout
	case stderr != "":
		msg += ": " + stderr
	}
	return msg
}

// limitedReader caps how many bytes are read from an HTTP response body without
// pulling in io.LimitReader semantics that swallow the underlying error.
type limitedReader struct {
	r interface{ Read([]byte) (int, error) }
	n int64
}

func (l *limitedReader) Read(p []byte) (int, error) {
	if l.n <= 0 {
		return 0, errLimitReached
	}
	if int64(len(p)) > l.n {
		p = p[:l.n]
	}
	n, err := l.r.Read(p)
	l.n -= int64(n)
	return n, err
}

// errLimitReached signals the body limit was hit; bytes.Buffer.ReadFrom treats
// any non-EOF error as terminal, which is the behaviour we want.
var errLimitReached = fmt.Errorf("hook response body limit reached")
