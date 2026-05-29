package plugins

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestAllHookEventsHas17Variants(t *testing.T) {
	if got := len(AllHookEvents()); got != 17 {
		t.Fatalf("expected 17 hook events, got %d", got)
	}
}

func TestHookEventStringRoundTrip(t *testing.T) {
	for _, event := range AllHookEvents() {
		name := event.String()
		parsed, ok := ParseHookEvent(name)
		if !ok {
			t.Fatalf("ParseHookEvent(%q) failed", name)
		}
		if parsed != event {
			t.Errorf("round trip mismatch: %v -> %q -> %v", event, name, parsed)
		}
	}
}

func TestHookEventWireNames(t *testing.T) {
	cases := map[HookEvent]string{
		PreToolUse:   "PreToolUse",
		SessionStart: "SessionStart",
		FileChanged:  "FileChanged",
		PluginUnload: "PluginUnload",
	}
	for event, want := range cases {
		if got := event.String(); got != want {
			t.Errorf("event %d: want %q, got %q", int(event), want, got)
		}
	}
}

func TestIsToolEvent(t *testing.T) {
	tool := []HookEvent{PreToolUse, PostToolUse, ToolError}
	for _, e := range tool {
		if !e.IsToolEvent() {
			t.Errorf("%v should be a tool event", e)
		}
	}
	if SessionStart.IsToolEvent() {
		t.Errorf("SessionStart should not be a tool event")
	}
}

func TestHookMatchRule(t *testing.T) {
	tests := []struct {
		name      string
		rule      HookMatchRule
		toolName  string
		toolInput string
		want      bool
	}{
		{"empty matches all", HookMatchRule{}, "bash", "rm -rf /", true},
		{"tool name match", HookMatchRule{ToolNames: []string{"bash"}}, "bash", "ls", true},
		{"tool name mismatch", HookMatchRule{ToolNames: []string{"bash"}}, "read_file", "ls", false},
		{"command prefix glob", HookMatchRule{Commands: []string{"rm *"}}, "bash", "sudo rm -rf x", true},
		{"command substring", HookMatchRule{Commands: []string{"git push"}}, "bash", "run git push now", true},
		{"command no match", HookMatchRule{Commands: []string{"rm *"}}, "bash", "ls -la", false},
		{"case insensitive", HookMatchRule{Commands: []string{"RM *"}}, "bash", "Rm file", true},
		{"both fields", HookMatchRule{ToolNames: []string{"bash"}, Commands: []string{"rm *"}}, "bash", "rm x", true},
		{"tool ok command fail", HookMatchRule{ToolNames: []string{"bash"}, Commands: []string{"rm *"}}, "bash", "ls", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.rule.Matches(tt.toolName, tt.toolInput); got != tt.want {
				t.Errorf("Matches(%q, %q) = %v, want %v", tt.toolName, tt.toolInput, got, tt.want)
			}
		})
	}
}

func TestCommandExecutorExitCodeSemantics(t *testing.T) {
	ctx := context.Background()
	exec := CommandExecutor{}

	tests := []struct {
		name    string
		run     string
		want    HookOutcome
		wantMsg string
	}{
		{"exit 0 allow", "printf 'ok'; exit 0", OutcomeAllow, "ok"},
		{"exit 2 deny", "printf 'blocked'; exit 2", OutcomeDeny, "blocked"},
		{"exit 1 warn", "printf 'noisy'; exit 1", OutcomeWarn, ""},
		{"exit 0 no output", "true", OutcomeAllow, ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			def := HookDefinition{Event: PreToolUse, Type: BackendCommand, Run: tt.run}
			outcome, msg := exec.Execute(ctx, def, PreToolUse, HookContext{ToolName: "bash", ToolInput: "{}"})
			if outcome != tt.want {
				t.Fatalf("outcome = %v, want %v (msg=%q)", outcome, tt.want, msg)
			}
			if tt.wantMsg != "" && msg != tt.wantMsg {
				t.Errorf("message = %q, want %q", msg, tt.wantMsg)
			}
		})
	}
}

func TestHTTPExecutorStatusSemantics(t *testing.T) {
	ctx := context.Background()
	exec := HTTPExecutor{}

	tests := []struct {
		name   string
		status int
		body   string
		want   HookOutcome
	}{
		{"200 allow", http.StatusOK, "fine", OutcomeAllow},
		{"403 deny", http.StatusForbidden, "nope", OutcomeDeny},
		{"500 warn", http.StatusInternalServerError, "boom", OutcomeWarn},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.Header.Get("Content-Type") != "application/json" {
					t.Errorf("missing json content type")
				}
				w.WriteHeader(tt.status)
				_, _ = w.Write([]byte(tt.body))
			}))
			defer srv.Close()

			def := HookDefinition{Event: PreToolUse, Type: BackendHTTP, URL: srv.URL}
			outcome, _ := exec.Execute(ctx, def, PreToolUse, HookContext{ToolName: "bash", ToolInput: "{}"})
			if outcome != tt.want {
				t.Errorf("outcome = %v, want %v", outcome, tt.want)
			}
		})
	}
}

func TestDispatcherDenyStopsLoop(t *testing.T) {
	ctx := context.Background()
	defs := []HookDefinition{
		{Event: PreToolUse, Type: BackendCommand, Run: "printf 'first'; exit 0"},
		{Event: PreToolUse, Type: BackendCommand, Run: "printf 'deny'; exit 2"},
		{Event: PreToolUse, Type: BackendCommand, Run: "printf 'never'; exit 0"},
	}
	d := NewHookDispatcher(defs)
	result := d.RunPreToolUse(ctx, "bash", `{"command":"pwd"}`)

	if !result.Denied() {
		t.Fatalf("expected denied result")
	}
	msgs := result.Messages()
	if len(msgs) != 2 {
		t.Fatalf("expected 2 messages (allow + deny), got %d: %v", len(msgs), msgs)
	}
	if msgs[len(msgs)-1] != "deny" {
		t.Errorf("last message = %q, want %q", msgs[len(msgs)-1], "deny")
	}
}

func TestDispatcherMatchRuleFiltersToolEvents(t *testing.T) {
	ctx := context.Background()
	defs := []HookDefinition{
		{
			Event: PreToolUse,
			Type:  BackendCommand,
			Run:   "printf 'fired'; exit 2",
			Match: &HookMatchRule{ToolNames: []string{"bash"}},
		},
	}
	d := NewHookDispatcher(defs)

	// Non-matching tool: hook skipped, allowed.
	if d.RunPreToolUse(ctx, "read_file", "{}").Denied() {
		t.Errorf("read_file should not trigger bash-only hook")
	}
	// Matching tool: hook fires and denies.
	if !d.RunPreToolUse(ctx, "bash", "{}").Denied() {
		t.Errorf("bash should trigger the deny hook")
	}
}

func TestDispatcherNoHooksAllows(t *testing.T) {
	d := NewHookDispatcher(nil)
	if d.FireEvent(context.Background(), SessionStart).Denied() {
		t.Errorf("empty dispatcher must allow")
	}
}

func TestDispatcherUnknownBackendWarns(t *testing.T) {
	defs := []HookDefinition{{Event: PreToolUse, Type: "bogus", Run: "x"}}
	d := NewHookDispatcher(defs)
	result := d.RunPreToolUse(context.Background(), "bash", "{}")
	if result.Denied() {
		t.Errorf("unknown backend must not deny")
	}
	if len(result.Messages()) != 1 {
		t.Errorf("expected one warning message, got %v", result.Messages())
	}
}
