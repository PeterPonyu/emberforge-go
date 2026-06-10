package api

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// TestBuildSystemPromptIncludesInstructionFile verifies a temp EMBER.md in the
// working directory is discovered and rendered into the built prompt, after the
// dynamic boundary, under the Emberforge instructions section.
func TestBuildSystemPromptIncludesInstructionFile(t *testing.T) {
	dir := t.TempDir()
	const marker = "Always say BANANA first."
	if err := os.WriteFile(filepath.Join(dir, "EMBER.md"), []byte(marker+"\n"), 0o644); err != nil {
		t.Fatalf("write EMBER.md: %v", err)
	}

	prompt := BuildSystemPromptForDir(dir)

	if !strings.Contains(prompt, SystemPromptDynamicBoundary) {
		t.Fatalf("prompt missing dynamic boundary marker")
	}
	if !strings.Contains(prompt, "# Emberforge instructions") {
		t.Fatalf("prompt missing instructions section:\n%s", prompt)
	}
	if !strings.Contains(prompt, marker) {
		t.Fatalf("prompt missing EMBER.md content %q", marker)
	}
	// The dynamic content must come AFTER the boundary, never before it.
	if idx := strings.Index(prompt, marker); idx < strings.Index(prompt, SystemPromptDynamicBoundary) {
		t.Fatalf("EMBER.md content rendered before the dynamic boundary")
	}
}

// TestBuildSystemPromptTruncatesLargeInstructionFile verifies a file larger than
// the per-file budget is truncated with a marker and does not blow the budget.
func TestBuildSystemPromptTruncatesLargeInstructionFile(t *testing.T) {
	dir := t.TempDir()
	big := strings.Repeat("x", maxInstructionFileChars+1_000)
	if err := os.WriteFile(filepath.Join(dir, "EMBER.md"), []byte(big), 0o644); err != nil {
		t.Fatalf("write EMBER.md: %v", err)
	}

	prompt := BuildSystemPromptForDir(dir)
	if !strings.Contains(prompt, "[truncated]") {
		t.Fatalf("expected truncation marker for oversized instruction file")
	}
	// The rendered file body must not exceed the per-file budget (plus marker).
	section := prompt[strings.Index(prompt, "# Emberforge instructions"):]
	xs := strings.Count(section, "x")
	if xs > maxInstructionFileChars {
		t.Fatalf("instruction body exceeded budget: %d > %d", xs, maxInstructionFileChars)
	}
}

// TestBuildSystemPromptIncludesGitStatusInRepo verifies the git status snapshot
// is injected when the working directory is a git repository.
func TestBuildSystemPromptIncludesGitStatusInRepo(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not available")
	}
	dir := t.TempDir()
	if out, err := exec.Command("git", "-C", dir, "init", "--quiet").CombinedOutput(); err != nil {
		t.Skipf("git init failed (%v): %s", err, out)
	}
	if err := os.WriteFile(filepath.Join(dir, "EMBER.md"), []byte("rules\n"), 0o644); err != nil {
		t.Fatalf("write EMBER.md: %v", err)
	}

	prompt := BuildSystemPromptForDir(dir)
	if !strings.Contains(prompt, "Git status snapshot:") {
		t.Fatalf("expected git status snapshot in repo, got:\n%s", prompt)
	}
	if !strings.Contains(prompt, "EMBER.md") {
		t.Fatalf("expected untracked EMBER.md in git status snapshot")
	}
}

// TestBuildSystemPromptGracefulWithoutGitRepo verifies prompt construction in a
// non-repository directory succeeds and simply omits the git snapshot.
func TestBuildSystemPromptGracefulWithoutGitRepo(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "EMBER.md"), []byte("rules\n"), 0o644); err != nil {
		t.Fatalf("write EMBER.md: %v", err)
	}

	prompt := BuildSystemPromptForDir(dir)
	// EMBER.md still appears; the build never fails on a missing repo.
	if !strings.Contains(prompt, "# Emberforge instructions") {
		t.Fatalf("expected instructions section even without a git repo")
	}
	if strings.Contains(prompt, "Git status snapshot:") {
		t.Fatalf("did not expect a git status snapshot outside a repo:\n%s", prompt)
	}
}

// TestBuildSystemPromptRendersConfigSection verifies the runtime config section
// is always present and reports discovered .ember settings files.
func TestBuildSystemPromptRendersConfigSection(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, ".ember.json"), []byte("{}\n"), 0o644); err != nil {
		t.Fatalf("write .ember.json: %v", err)
	}
	prompt := BuildSystemPromptForDir(dir)
	if !strings.Contains(prompt, "# Runtime config") {
		t.Fatalf("expected runtime config section")
	}
	if !strings.Contains(prompt, ".ember.json") {
		t.Fatalf("expected discovered .ember.json in config section:\n%s", prompt)
	}
}
