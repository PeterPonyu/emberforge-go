package system

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestInitializeRepoCreatesExpectedFilesAndGitignoreEntries(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "crates"), 0o755); err != nil {
		t.Fatalf("create crates dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(root, "Cargo.toml"), []byte("[workspace]\n"), 0o644); err != nil {
		t.Fatalf("write Cargo.toml: %v", err)
	}

	report, err := InitializeRepo(root)
	if err != nil {
		t.Fatalf("init: %v", err)
	}
	rendered := report.Render()
	for _, name := range []string{".ember/", ".ember.json", ".gitignore", "EMBER.md"} {
		if !strings.Contains(rendered, name) {
			t.Fatalf("expected report to mention %q, got:\n%s", name, rendered)
		}
	}

	if !isDir(filepath.Join(root, ".ember")) {
		t.Fatal("expected .ember directory")
	}
	emberJSON, err := os.ReadFile(filepath.Join(root, ".ember.json"))
	if err != nil {
		t.Fatalf("read .ember.json: %v", err)
	}
	if string(emberJSON) != starterEmberJSON {
		t.Fatalf("unexpected .ember.json:\n%s", emberJSON)
	}

	emberMD, err := os.ReadFile(filepath.Join(root, "EMBER.md"))
	if err != nil {
		t.Fatalf("read EMBER.md: %v", err)
	}
	md := string(emberMD)
	if !strings.Contains(md, "Languages: Rust.") {
		t.Fatalf("expected Rust language detection, got:\n%s", md)
	}
	if !strings.Contains(md, "cargo clippy --workspace --all-targets -- -D warnings") {
		t.Fatalf("expected rust verification line, got:\n%s", md)
	}
}

func TestInitializeRepoIsIdempotentAndPreservesExistingFiles(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "EMBER.md"), []byte("custom guidance\n"), 0o644); err != nil {
		t.Fatalf("seed EMBER.md: %v", err)
	}
	if err := os.WriteFile(filepath.Join(root, ".gitignore"), []byte(".ember/settings.local.json\n"), 0o644); err != nil {
		t.Fatalf("seed .gitignore: %v", err)
	}

	first, err := InitializeRepo(root)
	if err != nil {
		t.Fatalf("first init: %v", err)
	}
	if !strings.Contains(first.Render(), "EMBER.md") || !strings.Contains(first.Render(), "skipped") {
		t.Fatalf("expected EMBER.md skipped on first run, got:\n%s", first.Render())
	}

	second, err := InitializeRepo(root)
	if err != nil {
		t.Fatalf("second init: %v", err)
	}
	rendered := second.Render()
	for _, name := range []string{".ember/", ".ember.json", ".gitignore", "EMBER.md"} {
		if !strings.Contains(rendered, name) || !strings.Contains(rendered, "skipped") {
			t.Fatalf("expected %q skipped on second run, got:\n%s", name, rendered)
		}
	}

	preserved, err := os.ReadFile(filepath.Join(root, "EMBER.md"))
	if err != nil {
		t.Fatalf("read EMBER.md: %v", err)
	}
	if string(preserved) != "custom guidance\n" {
		t.Fatalf("expected EMBER.md preserved, got:\n%s", preserved)
	}
}

func TestInitializeRepoUpdatesPartialGitignore(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, ".gitignore"), []byte("node_modules/\n.ember/sessions/\n"), 0o644); err != nil {
		t.Fatalf("seed .gitignore: %v", err)
	}

	report, err := InitializeRepo(root)
	if err != nil {
		t.Fatalf("init: %v", err)
	}
	if !strings.Contains(report.Render(), ".gitignore       updated") {
		t.Fatalf("expected .gitignore to be updated, got:\n%s", report.Render())
	}

	gitignore, err := os.ReadFile(filepath.Join(root, ".gitignore"))
	if err != nil {
		t.Fatalf("read .gitignore: %v", err)
	}
	contents := string(gitignore)
	for _, expected := range []string{"node_modules/", gitignoreComment, ".ember/settings.local.json", ".claw/sessions/"} {
		if !strings.Contains(contents, expected) {
			t.Fatalf("expected .gitignore to contain %q, got:\n%s", expected, contents)
		}
	}
	// Existing entry must not be duplicated.
	if strings.Count(contents, ".ember/sessions/") != 1 {
		t.Fatalf("expected .ember/sessions/ once, got:\n%s", contents)
	}
}

func TestRenderInitEmberMDDetectsPythonAndNextjs(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "pyproject.toml"), []byte("[project]\nname = \"demo\"\n"), 0o644); err != nil {
		t.Fatalf("seed pyproject: %v", err)
	}
	if err := os.WriteFile(filepath.Join(root, "package.json"), []byte(`{"dependencies":{"next":"14.0.0","react":"18.0.0"},"devDependencies":{"typescript":"5.0.0"}}`), 0o644); err != nil {
		t.Fatalf("seed package.json: %v", err)
	}

	rendered := RenderInitEmberMD(root)
	for _, expected := range []string{
		"Languages: Python, TypeScript.",
		"Frameworks/tooling markers: Next.js, React.",
		"pyproject.toml",
		"Next.js detected",
	} {
		if !strings.Contains(rendered, expected) {
			t.Fatalf("expected EMBER.md to contain %q, got:\n%s", expected, rendered)
		}
	}
}
