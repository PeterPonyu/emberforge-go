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
