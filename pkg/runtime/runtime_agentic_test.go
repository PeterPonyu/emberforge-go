package runtime

import (
	"errors"
	"io"
	"strings"
	"testing"

	"github.com/PeterPonyu/emberforge-go/pkg/api"
	"github.com/PeterPonyu/emberforge-go/pkg/telemetry"
	"github.com/PeterPonyu/emberforge-go/pkg/tools"
)

// scriptedChatProvider is a mock api.ChatProvider that records every ChatRequest
// it receives and returns scripted ChatResponses in order. When alwaysToolCall
// is set it returns the first scripted response on every call, simulating a
// model that never stops requesting tools (to exercise the iteration bound).
type scriptedChatProvider struct {
	responses      []api.ChatResponse
	alwaysToolCall bool
	calls          []api.ChatRequest
	// streamText, when non-empty, is written to each request's Stream sink to
	// simulate incremental token streaming through the runtime.
	streamText string
}

func (s *scriptedChatProvider) SendMessage(api.MessageRequest) (api.MessageResponse, error) {
	return api.MessageResponse{}, errors.New("scriptedChatProvider: SendMessage not used in agentic loop")
}

func (s *scriptedChatProvider) Chat(request api.ChatRequest) (api.ChatResponse, error) {
	s.calls = append(s.calls, request)
	if s.streamText != "" && request.Stream != nil {
		io.WriteString(request.Stream, s.streamText)
	}
	if s.alwaysToolCall {
		return s.responses[0], nil
	}
	idx := len(s.calls) - 1
	if idx >= len(s.responses) {
		idx = len(s.responses) - 1
	}
	return s.responses[idx], nil
}

func newAgenticRuntime(t *testing.T, p api.Provider) *ConversationRuntime {
	t.Helper()
	sink := telemetry.ConsoleTelemetrySink{Writer: io.Discard}
	rt := NewConversationRuntime(p, tools.NewRealToolExecutor(t.TempDir()), sink)
	return rt
}

// TestRunAgenticLoop_ExecutesToolThenCompletes is the core agentic-loop test: the
// model requests a tool on turn 1 and returns final text on turn 2. It asserts
// (a) the request carried the tools array, (b) the tool actually executed and its
// real output was appended as a tool-role message fed back to the model, and
// (c) the loop terminated with the model's final text.
func TestRunAgenticLoop_ExecutesToolThenCompletes(t *testing.T) {
	provider := &scriptedChatProvider{
		responses: []api.ChatResponse{
			{
				Text: "let me run that",
				ToolCalls: []api.ToolCall{
					{Name: "bash", Arguments: map[string]any{"command": "echo loop-test-marker"}},
				},
			},
			{Text: "the directory listing is complete"},
		},
	}
	rt := newAgenticRuntime(t, provider)

	output, err := rt.RunTurnResult("list the files")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if output != "the directory listing is complete" {
		t.Fatalf("final output = %q, want the turn-2 text", output)
	}

	// The loop must have made exactly two model calls: turn 1 (tool request) and
	// turn 2 (final text after the tool result was fed back).
	if len(provider.calls) != 2 {
		t.Fatalf("expected 2 model calls, got %d", len(provider.calls))
	}

	// (a) The request must advertise the existing tool registry as the tools
	// array, including bash with its schema.
	firstTools := provider.calls[0].Tools
	if len(firstTools) == 0 {
		t.Fatal("expected the request to carry the tools array, got none")
	}
	var bash *api.ToolDefinition
	for i := range firstTools {
		if firstTools[i].Name == "bash" {
			bash = &firstTools[i]
			break
		}
	}
	if bash == nil {
		t.Fatalf("tools array missing bash; got %d tools", len(firstTools))
	}
	if bash.Parameters == nil {
		t.Fatal("bash tool definition missing its input schema (parameters)")
	}

	// (b) The second model call must include the tool result appended as a
	// `tool` role message, carrying the REAL echo output (proving the executor
	// ran rather than the call being faked or string-scraped).
	secondMessages := provider.calls[1].Messages
	var toolMsg *api.ChatMessage
	for i := range secondMessages {
		if secondMessages[i].Role == "tool" {
			toolMsg = &secondMessages[i]
			break
		}
	}
	if toolMsg == nil {
		t.Fatal("second request missing the appended tool-role message")
	}
	if toolMsg.ToolName != "bash" {
		t.Fatalf("tool message tool_name = %q, want bash", toolMsg.ToolName)
	}
	if !strings.Contains(toolMsg.Content, "loop-test-marker") {
		t.Fatalf("tool message did not carry real echo output; content=%q", toolMsg.Content)
	}

	// The assistant's turn-1 message (with the tool call) must also have been
	// threaded back into the conversation before the tool result.
	var sawAssistantToolCall bool
	for _, m := range secondMessages {
		if m.Role == "assistant" && len(m.ToolCalls) == 1 && m.ToolCalls[0].Name == "bash" {
			sawAssistantToolCall = true
		}
	}
	if !sawAssistantToolCall {
		t.Fatal("expected the assistant tool-call turn to be appended to the conversation")
	}
}

// TestRunAgenticLoop_MaxIterationsBoundsRunaway verifies that a model which never
// stops requesting tools is bounded by maxIterations rather than looping forever.
func TestRunAgenticLoop_MaxIterationsBoundsRunaway(t *testing.T) {
	provider := &scriptedChatProvider{
		alwaysToolCall: true,
		responses: []api.ChatResponse{
			{
				Text: "still going",
				ToolCalls: []api.ToolCall{
					{Name: "bash", Arguments: map[string]any{"command": "true"}},
				},
			},
		},
	}
	rt := newAgenticRuntime(t, provider)
	rt.MaxIterations = 3

	output, err := rt.RunTurnResult("loop forever")
	if err == nil {
		t.Fatal("expected an error when the iteration bound is exceeded")
	}
	if !strings.Contains(err.Error(), "maximum number of iterations") {
		t.Fatalf("error = %v, want max-iterations message", err)
	}
	if !strings.Contains(output, "maximum number of iterations") {
		t.Fatalf("output = %q, want rendered max-iterations message", output)
	}
	// The model is called once per iteration up to the bound, then the loop
	// aborts before the next call: exactly MaxIterations calls.
	if len(provider.calls) != 3 {
		t.Fatalf("expected exactly 3 model calls under the bound, got %d", len(provider.calls))
	}
}

// TestRunAgenticLoop_RespectsPermissionGating verifies the loop runs tools
// through the permission gate: in read-only mode a bash call is denied (not
// executed) and the denial is fed back as the tool result.
func TestRunAgenticLoop_RespectsPermissionGating(t *testing.T) {
	provider := &scriptedChatProvider{
		responses: []api.ChatResponse{
			{ToolCalls: []api.ToolCall{{Name: "bash", Arguments: map[string]any{"command": "echo should-not-run"}}}},
			{Text: "acknowledged the denial"},
		},
	}
	rt := newAgenticRuntime(t, provider)
	rt.PermissionMode = tools.PermissionReadOnly

	if _, err := rt.RunTurnResult("run bash"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	toolMsg := firstToolMessage(t, provider.calls[1].Messages)
	if !strings.Contains(toolMsg.Content, "permission denied") {
		t.Fatalf("expected permission denial fed back, got %q", toolMsg.Content)
	}
	if strings.Contains(toolMsg.Content, "should-not-run") {
		t.Fatalf("bash must not have executed in read-only mode; got %q", toolMsg.Content)
	}
}

// TestRunAgenticLoop_StreamsAssistantText verifies the runtime threads its Stream
// sink into the provider so assistant text is surfaced incrementally.
func TestRunAgenticLoop_StreamsAssistantText(t *testing.T) {
	provider := &scriptedChatProvider{
		responses:  []api.ChatResponse{{Text: "streamed answer"}},
		streamText: "streamed answer",
	}
	rt := newAgenticRuntime(t, provider)
	var sink strings.Builder
	rt.Stream = &sink

	if !rt.StreamsOutput() {
		t.Fatal("expected StreamsOutput to be true for a chat provider with a stream sink")
	}
	if _, err := rt.RunTurnResult("hi"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if sink.String() != "streamed answer" {
		t.Fatalf("stream sink = %q, want %q", sink.String(), "streamed answer")
	}
}

func firstToolMessage(t *testing.T, messages []api.ChatMessage) api.ChatMessage {
	t.Helper()
	for _, m := range messages {
		if m.Role == "tool" {
			return m
		}
	}
	t.Fatal("no tool-role message found")
	return api.ChatMessage{}
}
