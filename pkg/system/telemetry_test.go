package system

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestJsonlTelemetryPathRecordsDispatchEvents(t *testing.T) {
	configHome := t.TempDir()
	t.Setenv("EMBER_CONFIG_HOME", configHome)
	t.Setenv("EMBER_TASK_STATE_PATH", filepath.Join(t.TempDir(), "task-state.json"))
	t.Setenv("EMBER_BUDDY_STATE_PATH", filepath.Join(t.TempDir(), "buddy-state.json"))

	config := DefaultStarterSystemConfig()
	config.TelemetryMode = TelemetryModeJSONL

	app := NewStarterSystemApplication(config)

	// Dispatch a command through the full sequence so telemetry events fire.
	if _, handled := ExecuteStarterSlashCommand(app, "/status"); !handled {
		t.Fatal("expected /status to be handled")
	}
	app.Sequence.Handle("hello from telemetry test")
	app.Shutdown()

	telemetryDir := filepath.Join(configHome, "telemetry")
	entries, err := os.ReadDir(telemetryDir)
	if err != nil {
		t.Fatalf("read telemetry dir: %v", err)
	}
	if len(entries) != 1 {
		t.Fatalf("expected exactly one telemetry log, got %d", len(entries))
	}
	logName := entries[0].Name()
	if !strings.HasSuffix(logName, ".jsonl") {
		t.Fatalf("expected .jsonl telemetry log, got %q", logName)
	}

	raw, err := os.ReadFile(filepath.Join(telemetryDir, logName))
	if err != nil {
		t.Fatalf("read telemetry log: %v", err)
	}
	contents := string(raw)
	for _, expected := range []string{
		`"session_id":"` + app.SessionID + `"`,
		`"seq":0`,
		`"timestamp_ms":`,
		`"type":"bootstrap_completed"`,
		`"type":"turn_started"`,
		`"type":"sequence_persisted"`,
		`"type":"shutdown_completed"`,
		`"details":`,
	} {
		if !strings.Contains(contents, expected) {
			t.Fatalf("expected telemetry log to contain %q, got:\n%s", expected, contents)
		}
	}
}
