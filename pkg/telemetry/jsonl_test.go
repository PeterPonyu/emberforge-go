package telemetry

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestJsonlTelemetrySinkAppendsRecords(t *testing.T) {
	path := filepath.Join(t.TempDir(), "nested", "telemetry.jsonl")

	sink, err := NewJsonlTelemetrySink("session-abc", path)
	if err != nil {
		t.Fatalf("create sink: %v", err)
	}

	sink.Record(Event{Name: "turn_started", Details: "hello"})
	sink.Record(Event{Name: "turn_completed"})
	if err := sink.Close(); err != nil {
		t.Fatalf("close sink: %v", err)
	}

	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read log: %v", err)
	}
	lines := strings.Split(strings.TrimSpace(string(raw)), "\n")
	if len(lines) != 2 {
		t.Fatalf("expected 2 jsonl lines, got %d:\n%s", len(lines), raw)
	}

	if !strings.Contains(lines[0], `"session_id":"session-abc"`) ||
		!strings.Contains(lines[0], `"seq":0`) ||
		!strings.Contains(lines[0], `"type":"turn_started"`) ||
		!strings.Contains(lines[0], `"details":"hello"`) ||
		!strings.Contains(lines[0], `"timestamp_ms":`) {
		t.Fatalf("unexpected first record: %s", lines[0])
	}
	if !strings.Contains(lines[1], `"seq":1`) ||
		!strings.Contains(lines[1], `"type":"turn_completed"`) {
		t.Fatalf("unexpected second record: %s", lines[1])
	}
	// Detail-free events must omit the attributes map.
	if strings.Contains(lines[1], "attributes") {
		t.Fatalf("expected detail-free record to omit attributes: %s", lines[1])
	}
}

func TestJsonlTelemetrySinkAppendsToExistingFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "telemetry.jsonl")
	if err := os.WriteFile(path, []byte(`{"existing":true}`+"\n"), 0o644); err != nil {
		t.Fatalf("seed file: %v", err)
	}

	sink, err := NewJsonlTelemetrySink("session-xyz", path)
	if err != nil {
		t.Fatalf("create sink: %v", err)
	}
	sink.Record(Event{Name: "appended"})
	if err := sink.Close(); err != nil {
		t.Fatalf("close sink: %v", err)
	}

	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read log: %v", err)
	}
	contents := string(raw)
	if !strings.Contains(contents, `{"existing":true}`) {
		t.Fatalf("expected existing telemetry to be preserved, got:\n%s", contents)
	}
	if !strings.Contains(contents, `"type":"appended"`) {
		t.Fatalf("expected appended record, got:\n%s", contents)
	}
}

func TestDefaultJsonlPathHonorsConfigHome(t *testing.T) {
	t.Setenv("EMBER_CONFIG_HOME", "/tmp/ember-config")
	got := DefaultJsonlPath("session-1")
	want := filepath.Join("/tmp/ember-config", "telemetry", "session-1.jsonl")
	if got != want {
		t.Fatalf("expected %q, got %q", want, got)
	}
}

var _ TelemetrySink = (*JsonlTelemetrySink)(nil)
