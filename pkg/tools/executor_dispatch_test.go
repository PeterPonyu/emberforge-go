package tools

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestExecuteWithPermission_GatesByMode(t *testing.T) {
	dir := t.TempDir()
	exec := NewRealToolExecutor(dir)
	registry := NewToolRegistry(nil)

	// read_file requires read-only: allowed in every mode.
	path := filepath.Join(dir, "f.txt")
	if err := os.WriteFile(path, []byte("body"), 0o644); err != nil {
		t.Fatalf("setup: %v", err)
	}
	out, ran := exec.ExecuteWithPermission(registry, "read_file", path, PermissionReadOnly)
	if !ran || out != "body" {
		t.Fatalf("read_file should run in read-only mode: ran=%v out=%q", ran, out)
	}

	// write_file requires workspace-write: denied in read-only mode.
	out, ran = exec.ExecuteWithPermission(registry, "write_file", path+":new", PermissionReadOnly)
	if ran {
		t.Fatalf("write_file must be denied in read-only mode: %q", out)
	}
	if !strings.Contains(out, "permission denied") {
		t.Errorf("expected permission denied message, got %q", out)
	}

	// bash requires danger-full-access: denied in workspace-write mode.
	out, ran = exec.ExecuteWithPermission(registry, "bash", "echo hi", PermissionWorkspaceWrite)
	if ran {
		t.Fatalf("bash must be denied in workspace-write mode: %q", out)
	}

	// bash permitted with full access.
	out, ran = exec.ExecuteWithPermission(registry, "bash", "echo hi", PermissionDangerFullAccess)
	if !ran || strings.TrimSpace(out) != "hi" {
		t.Fatalf("bash should run with full access: ran=%v out=%q", ran, out)
	}
}

func TestExecuteWithPermission_UnknownTool(t *testing.T) {
	exec := NewRealToolExecutor(t.TempDir())
	out, ran := exec.ExecuteWithPermission(NewToolRegistry(nil), "nope", "", PermissionDangerFullAccess)
	if ran {
		t.Fatal("unknown tool must not run")
	}
	if !strings.Contains(out, "unknown tool") {
		t.Errorf("expected unknown tool message, got %q", out)
	}
}

func TestExecute_EditFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "code.txt")
	if err := os.WriteFile(path, []byte("alpha beta gamma"), 0o644); err != nil {
		t.Fatalf("setup: %v", err)
	}
	exec := NewRealToolExecutor(dir)

	out := exec.Execute("edit_file", path+"\x00beta\x00BETA")
	if strings.Contains(out, "error") {
		t.Fatalf("unexpected error: %s", out)
	}
	data, _ := os.ReadFile(path)
	if string(data) != "alpha BETA gamma" {
		t.Errorf("got %q want %q", string(data), "alpha BETA gamma")
	}

	if out := exec.Execute("edit_file", path+"\x00missing\x00x"); !strings.Contains(out, "not found") {
		t.Errorf("expected not-found error, got %q", out)
	}
}

func TestExecute_GlobSearch(t *testing.T) {
	dir := t.TempDir()
	for _, name := range []string{"a.go", "b.go", "c.txt"} {
		if err := os.WriteFile(filepath.Join(dir, name), []byte("x"), 0o644); err != nil {
			t.Fatalf("setup: %v", err)
		}
	}
	exec := NewRealToolExecutor(dir)
	out := exec.Execute("glob_search", "*.go")
	if !strings.Contains(out, "a.go") || !strings.Contains(out, "b.go") {
		t.Errorf("glob should find go files, got %q", out)
	}
	if strings.Contains(out, "c.txt") {
		t.Errorf("glob should not match c.txt, got %q", out)
	}
}

func TestExecute_GrepSearch(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "f.txt"), []byte("one\nneedle here\nthree"), 0o644); err != nil {
		t.Fatalf("setup: %v", err)
	}
	exec := NewRealToolExecutor(dir)
	out := exec.Execute("grep_search", "needle")
	if !strings.Contains(out, "needle here") || !strings.Contains(out, ":2:") {
		t.Errorf("grep should report matching line with number, got %q", out)
	}
	if out := exec.Execute("grep_search", "absent-pattern"); !strings.Contains(out, "no matches") {
		t.Errorf("expected no matches, got %q", out)
	}
}

func TestExecute_DeferredToolsReturnReceipt(t *testing.T) {
	exec := NewRealToolExecutor(t.TempDir())
	for _, name := range []string{"web", "notebook", "agent", "skill"} {
		out := exec.Execute(name, "payload")
		if !strings.Contains(out, name) || !strings.Contains(out, "payload") {
			t.Errorf("tool %q receipt malformed: %q", name, out)
		}
	}
}
