package system

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

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
