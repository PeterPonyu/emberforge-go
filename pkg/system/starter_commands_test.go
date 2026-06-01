package system

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestExecuteStarterSlashCommandTable(t *testing.T) {
	cases := []struct {
		name     string
		input    string
		wantOK   bool
		contains []string
	}{
		{name: "help", input: "/help", wantOK: true, contains: []string{"available commands:", "/init"}},
		{name: "status", input: "/status", wantOK: true, contains: []string{"[command] status:", "lifecycle="}},
		{name: "doctor", input: "/doctor", wantOK: true, contains: []string{"emberforge-go doctor"}},
		{name: "model_list", input: "/model list", wantOK: true, contains: []string{"[command] model list:"}},
		{name: "compact", input: "/compact", wantOK: true, contains: []string{"[command] compact:"}},
		{name: "review", input: "/review api", wantOK: true, contains: []string{"[command] review", "scope: api"}},
		{name: "commit", input: "/commit ship it", wantOK: true, contains: []string{"[command] commit", "summary: ship it"}},
		{name: "pr", input: "/pr notes", wantOK: true, contains: []string{"[command] pr", "context: notes"}},
		{name: "unknown", input: "/nope", wantOK: false},
		{name: "not_slash", input: "hello", wantOK: false},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			app := NewStarterSystemApplication(DefaultStarterSystemConfig())
			defer app.Shutdown()

			output, ok := ExecuteStarterSlashCommand(app, tc.input)
			if ok != tc.wantOK {
				t.Fatalf("expected ok=%t for %q, got %t (output=%q)", tc.wantOK, tc.input, ok, output)
			}
			for _, want := range tc.contains {
				if !strings.Contains(output, want) {
					t.Fatalf("expected output for %q to contain %q, got:\n%s", tc.input, want, output)
				}
			}
		})
	}
}

func TestExecuteStarterSlashCommandHelp(t *testing.T) {
	app := NewStarterSystemApplication(DefaultStarterSystemConfig())
	defer app.Shutdown()

	output, ok := ExecuteStarterSlashCommand(app, "/help")
	if !ok {
		t.Fatal("expected /help to be handled")
	}
	if !strings.Contains(output, "/questions [ask <task-id> <text>|pending|answer <question-id> <text>]") {
		t.Fatalf("expected help to include questions hint, got:\n%s", output)
	}
	if !strings.Contains(output, "/tasks [create prompt <text>|list|show <task-id>|stop <task-id>]") {
		t.Fatalf("expected help to include tasks hint, got:\n%s", output)
	}
	if !strings.Contains(output, "/doctor [quick|status]") {
		t.Fatalf("expected help to include doctor hint, got:\n%s", output)
	}
	if !strings.Contains(output, "/buddy [hatch|rehatch|pet|mute|unmute]") {
		t.Fatalf("expected help to include buddy hint, got:\n%s", output)
	}
	if !strings.Contains(output, "/pr [context]") {
		t.Fatalf("expected help to include pr hint, got:\n%s", output)
	}
}

func TestExecuteStarterSlashCommandHelpListsInit(t *testing.T) {
	app := NewStarterSystemApplication(DefaultStarterSystemConfig())
	defer app.Shutdown()

	output, ok := ExecuteStarterSlashCommand(app, "/help")
	if !ok {
		t.Fatal("expected /help to be handled")
	}
	if !strings.Contains(output, "/init [path] -- Scaffold EMBER.md, .ember.json, and .gitignore entries") {
		t.Fatalf("expected help to include init hint, got:\n%s", output)
	}
}

func TestExecuteStarterSlashCommandInit(t *testing.T) {
	root := t.TempDir()

	output, ok := dispatchInTempApp(t, "/init "+root)
	if !ok {
		t.Fatal("expected /init to be handled")
	}

	for _, expected := range []string{
		"Init",
		"Project          " + root,
		".ember/          created",
		".ember.json      created",
		".gitignore       created",
		"EMBER.md         created",
		"Next step        Review and tailor the generated guidance",
	} {
		if !strings.Contains(output, expected) {
			t.Fatalf("expected init output to contain %q, got:\n%s", expected, output)
		}
	}

	if !isDir(filepath.Join(root, ".ember")) {
		t.Fatalf("expected .ember directory to be created")
	}
	emberJSON, err := os.ReadFile(filepath.Join(root, ".ember.json"))
	if err != nil {
		t.Fatalf("read .ember.json: %v", err)
	}
	if string(emberJSON) != starterEmberJSON {
		t.Fatalf("unexpected .ember.json contents:\n%s", emberJSON)
	}
	gitignore, err := os.ReadFile(filepath.Join(root, ".gitignore"))
	if err != nil {
		t.Fatalf("read .gitignore: %v", err)
	}
	for _, entry := range gitignoreEntries {
		if !strings.Contains(string(gitignore), entry) {
			t.Fatalf("expected .gitignore to contain %q, got:\n%s", entry, gitignore)
		}
	}
}

func TestExecuteStarterSlashCommandInitIsIdempotent(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "EMBER.md"), []byte("custom guidance\n"), 0o644); err != nil {
		t.Fatalf("seed EMBER.md: %v", err)
	}

	first, ok := dispatchInTempApp(t, "/init "+root)
	if !ok {
		t.Fatal("expected /init to be handled")
	}
	if !strings.Contains(first, "EMBER.md         skipped (already exists)") {
		t.Fatalf("expected EMBER.md to be skipped, got:\n%s", first)
	}

	second, _ := dispatchInTempApp(t, "/init "+root)
	for _, name := range []string{".ember/", ".ember.json", ".gitignore", "EMBER.md"} {
		if !strings.Contains(second, name) || !strings.Contains(second, "skipped (already exists)") {
			t.Fatalf("expected second init to skip %q, got:\n%s", name, second)
		}
	}

	preserved, err := os.ReadFile(filepath.Join(root, "EMBER.md"))
	if err != nil {
		t.Fatalf("read EMBER.md: %v", err)
	}
	if string(preserved) != "custom guidance\n" {
		t.Fatalf("expected existing EMBER.md to be preserved, got:\n%s", preserved)
	}
}

func dispatchInTempApp(t *testing.T, command string) (string, bool) {
	t.Helper()
	app := NewStarterSystemApplication(DefaultStarterSystemConfig())
	t.Cleanup(app.Shutdown)
	return ExecuteStarterSlashCommand(app, command)
}

func TestExecuteStarterSlashCommandDoctor(t *testing.T) {
	app := NewStarterSystemApplication(DefaultStarterSystemConfig())
	defer app.Shutdown()

	output, ok := ExecuteStarterSlashCommand(app, "/doctor")
	if !ok {
		t.Fatal("expected /doctor to be handled")
	}
	if !strings.Contains(output, "emberforge-go doctor") {
		t.Fatalf("unexpected doctor output:\n%s", output)
	}
}

func TestExecuteStarterSlashCommandDoctorStatus(t *testing.T) {
	app := NewStarterSystemApplication(DefaultStarterSystemConfig())
	defer app.Shutdown()

	output, ok := ExecuteStarterSlashCommand(app, "/doctor status")
	if !ok {
		t.Fatal("expected /doctor status to be handled")
	}
	if !strings.Contains(output, "emberforge-go doctor status") {
		t.Fatalf("unexpected doctor status output:\n%s", output)
	}
	if !strings.Contains(output, "last_route: none") {
		t.Fatalf("unexpected doctor status output:\n%s", output)
	}
}

func TestExecuteStarterSlashCommandModelAndPayloadCommands(t *testing.T) {
	app := NewStarterSystemApplication(DefaultStarterSystemConfig())
	defer app.Shutdown()

	modelOutput, ok := ExecuteStarterSlashCommand(app, "/model list")
	if !ok || !strings.Contains(modelOutput, "model list:") {
		t.Fatalf("unexpected model list output: %q", modelOutput)
	}

	reviewOutput, ok := ExecuteStarterSlashCommand(app, "/review workspace")
	if !ok || !strings.Contains(reviewOutput, "[command] review") || !strings.Contains(reviewOutput, "scope: workspace") {
		t.Fatalf("unexpected review output: %q", reviewOutput)
	}

	prOutput, ok := ExecuteStarterSlashCommand(app, "/pr release notes")
	if !ok || !strings.Contains(prOutput, "[command] pr") || !strings.Contains(prOutput, "context: release notes") {
		t.Fatalf("unexpected pr output: %q", prOutput)
	}
}

func TestExecuteStarterSlashCommandBuddyLifecycle(t *testing.T) {
	t.Setenv("EMBER_BUDDY_STATE_PATH", filepath.Join(t.TempDir(), "buddy-state.json"))

	app := NewStarterSystemApplication(DefaultStarterSystemConfig())
	defer app.Shutdown()

	output, ok := ExecuteStarterSlashCommand(app, "/buddy")
	if !ok || !strings.Contains(output, "status: no companion") {
		t.Fatalf("unexpected /buddy output: %q", output)
	}

	output, ok = ExecuteStarterSlashCommand(app, "/buddy hatch")
	if !ok || !strings.Contains(output, "name: Waddles") || !strings.Contains(output, "species: Duck") {
		t.Fatalf("unexpected /buddy hatch output: %q", output)
	}

	output, ok = ExecuteStarterSlashCommand(app, "/buddy hatch")
	if !ok || !strings.Contains(output, "status: companion already active") || !strings.Contains(output, "/buddy rehatch") {
		t.Fatalf("unexpected second /buddy hatch output: %q", output)
	}

	output, ok = ExecuteStarterSlashCommand(app, "/buddy mute")
	if !ok || !strings.Contains(output, "status: muted") || !strings.Contains(output, "hide quietly") {
		t.Fatalf("unexpected /buddy mute output: %q", output)
	}

	output, ok = ExecuteStarterSlashCommand(app, "/buddy mute")
	if !ok || !strings.Contains(output, "status: already muted") {
		t.Fatalf("unexpected second /buddy mute output: %q", output)
	}

	output, ok = ExecuteStarterSlashCommand(app, "/buddy pet")
	if !ok || !strings.Contains(output, "reaction: Waddles purrs happily!") {
		t.Fatalf("unexpected /buddy pet output: %q", output)
	}

	output, ok = ExecuteStarterSlashCommand(app, "/buddy unmute")
	if !ok || !strings.Contains(output, "status: active") || !strings.Contains(output, "welcome back") {
		t.Fatalf("unexpected /buddy unmute output: %q", output)
	}

	output, ok = ExecuteStarterSlashCommand(app, "/buddy unmute")
	if !ok || !strings.Contains(output, "status: already active") {
		t.Fatalf("unexpected second /buddy unmute output: %q", output)
	}

	output, ok = ExecuteStarterSlashCommand(app, "/buddy rehatch")
	if !ok || !strings.Contains(output, "name: Goosberry") || !strings.Contains(output, "species: Goose") {
		t.Fatalf("unexpected /buddy rehatch output: %q", output)
	}

	app2 := NewStarterSystemApplication(DefaultStarterSystemConfig())
	defer app2.Shutdown()

	output, ok = ExecuteStarterSlashCommand(app2, "/buddy")
	if !ok || !strings.Contains(output, "name: Goosberry") || !strings.Contains(output, "species: Goose") {
		t.Fatalf("expected persisted buddy state, got: %q", output)
	}
}

func TestExecuteStarterSlashCommandTaskQuestionResumeFlow(t *testing.T) {
	t.Setenv("EMBER_TASK_STATE_PATH", filepath.Join(t.TempDir(), "task-question-state.json"))

	app := NewStarterSystemApplication(DefaultStarterSystemConfig())
	defer app.Shutdown()

	output, ok := ExecuteStarterSlashCommand(app, "/tasks create prompt investigate auth flow")
	if !ok || !strings.Contains(output, "task_id: task-1") || !strings.Contains(output, "status: in_progress") {
		t.Fatalf("unexpected task create output: %q", output)
	}

	output, ok = ExecuteStarterSlashCommand(app, "/questions ask task-1 Which tenant should we target first?")
	if !ok || !strings.Contains(output, "question_id: question-1") || !strings.Contains(output, "status: waiting_for_user") {
		t.Fatalf("unexpected question ask output: %q", output)
	}

	app2 := NewStarterSystemApplication(DefaultStarterSystemConfig())
	defer app2.Shutdown()

	output, ok = ExecuteStarterSlashCommand(app2, "/questions pending")
	if !ok || !strings.Contains(output, "question-1 -> task-1") {
		t.Fatalf("unexpected pending questions output: %q", output)
	}

	output, ok = ExecuteStarterSlashCommand(app2, "/tasks show task-1")
	if !ok || !strings.Contains(output, "status: waiting_for_user") {
		t.Fatalf("unexpected task show output: %q", output)
	}

	output, ok = ExecuteStarterSlashCommand(app2, "/questions answer question-1 Start with the billing tenant")
	if !ok || !strings.Contains(output, "task_status: completed") {
		t.Fatalf("unexpected question answer output: %q", output)
	}

	output, ok = ExecuteStarterSlashCommand(app2, "/tasks show task-1")
	if !ok || !strings.Contains(output, "status: completed") || !strings.Contains(output, "answer: Start with the billing tenant") {
		t.Fatalf("unexpected completed task output: %q", output)
	}

	transcriptRaw, err := os.ReadFile(filepath.Join(filepath.Dir(os.Getenv("EMBER_TASK_STATE_PATH")), "task-question-transcript.jsonl"))
	if err != nil {
		t.Fatalf("read transcript: %v", err)
	}
	transcript := string(transcriptRaw)
	for _, expected := range []string{
		`"id":"task-question-runtime"`,
		`"type":"task_state"`,
		`"type":"question_state"`,
		`"status":"waiting_for_user"`,
		`"status":"completed"`,
	} {
		if !strings.Contains(transcript, expected) {
			t.Fatalf("expected transcript to contain %q, got:\n%s", expected, transcript)
		}
	}
}
