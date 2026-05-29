package tools

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRealToolExecutor_ReadFileRoundTrip(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "hello.txt")
	content := "hello world"
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatalf("setup: %v", err)
	}

	exec := NewRealToolExecutor(dir)
	got := exec.Execute("read_file", path)
	if got != content {
		t.Errorf("expected %q, got %q", content, got)
	}
}

func TestRealToolExecutor_ReadFileRejectsOutsideWorkspace(t *testing.T) {
	workspace := t.TempDir()

	// A secret file living entirely outside the workspace root.
	outsideDir := t.TempDir()
	outsidePath := filepath.Join(outsideDir, "secret.txt")
	if err := os.WriteFile(outsidePath, []byte("outside-secret"), 0644); err != nil {
		t.Fatalf("setup: %v", err)
	}

	// A legitimate in-workspace file.
	insidePath := filepath.Join(workspace, "inside.txt")
	if err := os.WriteFile(insidePath, []byte("inside-ok"), 0644); err != nil {
		t.Fatalf("setup: %v", err)
	}

	exec := NewRealToolExecutor(workspace)

	tests := []struct {
		name      string
		path      string
		wantError bool
		wantBody  string
	}{
		{
			name:      "absolute outside path rejected",
			path:      outsidePath,
			wantError: true,
		},
		{
			name:      "traversal-shaped outside path rejected",
			path:      filepath.Join(workspace, "..", filepath.Base(outsideDir), "secret.txt"),
			wantError: true,
		},
		{
			name:      "in-workspace read succeeds",
			path:      insidePath,
			wantError: false,
			wantBody:  "inside-ok",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := exec.Execute("read_file", tt.path)
			if tt.wantError {
				if !strings.Contains(got, "[read_file error]") || !strings.Contains(got, "outside workspace") {
					t.Errorf("expected outside-workspace rejection, got %q", got)
				}
				if strings.Contains(got, "outside-secret") {
					t.Errorf("read_file leaked outside file contents: %q", got)
				}
				return
			}
			if got != tt.wantBody {
				t.Errorf("expected %q, got %q", tt.wantBody, got)
			}
		})
	}
}

func TestRealToolExecutor_WriteFileCreatesFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "out.txt")
	content := "written by test"

	exec := NewRealToolExecutor(dir)
	result := exec.Execute("write_file", path+":"+content)
	if strings.Contains(result, "error") {
		t.Fatalf("unexpected error: %s", result)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("file not created: %v", err)
	}
	if string(data) != content {
		t.Errorf("expected %q, got %q", content, string(data))
	}
}

func TestRealToolExecutor_BashEchoReturnsOutput(t *testing.T) {
	dir := t.TempDir()
	exec := NewRealToolExecutor(dir)
	got := exec.Execute("bash", "echo hello")
	got = strings.TrimSpace(got)
	if got != "hello" {
		t.Errorf("expected %q, got %q", "hello", got)
	}
}
