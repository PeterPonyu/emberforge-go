package system

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// starterEmberJSON is the default project configuration scaffolded by /init.
// It mirrors the Rust STARTER_EMBER_JSON constant.
const starterEmberJSON = "{\n" +
	"  \"permissions\": {\n" +
	"    \"defaultMode\": \"dontAsk\"\n" +
	"  }\n" +
	"}\n"

const gitignoreComment = "# Emberforge local artifacts"

// gitignoreEntries mirror the Rust GITIGNORE_ENTRIES list.
var gitignoreEntries = []string{
	".ember/settings.local.json",
	".ember/sessions/",
	".claw/settings.local.json",
	".claw/sessions/",
}

// InitStatus records what happened to a scaffolded artifact.
type InitStatus string

const (
	InitStatusCreated InitStatus = "created"
	InitStatusUpdated InitStatus = "updated"
	InitStatusSkipped InitStatus = "skipped (already exists)"
)

// InitArtifact is a single file/directory touched by initialization.
type InitArtifact struct {
	Name   string
	Status InitStatus
}

// InitReport summarizes an initialization run.
type InitReport struct {
	ProjectRoot string
	Artifacts   []InitArtifact
}

// Render formats the report the way the Rust InitReport::render does.
func (r InitReport) Render() string {
	lines := []string{
		"Init",
		fmt.Sprintf("  Project          %s", r.ProjectRoot),
	}
	for _, artifact := range r.Artifacts {
		lines = append(lines, fmt.Sprintf("  %-16s %s", artifact.Name, artifact.Status))
	}
	lines = append(lines, "  Next step        Review and tailor the generated guidance")
	return strings.Join(lines, "\n")
}

// repoDetection captures the language/framework markers found in a project.
type repoDetection struct {
	rustWorkspace bool
	rustRoot      bool
	python        bool
	packageJSON   bool
	typescript    bool
	nextjs        bool
	react         bool
	vite          bool
	nest          bool
	srcDir        bool
	testsDir      bool
	rustDir       bool
}

// initTransaction tracks created paths and replaced file contents so a failed
// initialization can be rolled back, mirroring the Rust InitTransaction.
type initTransaction struct {
	createdPaths  []string
	replacedFiles []replacedFile
}

type replacedFile struct {
	path    string
	content string
}

func (t *initTransaction) rollback() {
	for i := len(t.replacedFiles) - 1; i >= 0; i-- {
		_ = os.WriteFile(t.replacedFiles[i].path, []byte(t.replacedFiles[i].content), 0o644)
	}
	for i := len(t.createdPaths) - 1; i >= 0; i-- {
		path := t.createdPaths[i]
		if info, err := os.Stat(path); err == nil && info.IsDir() {
			_ = os.RemoveAll(path)
		} else {
			_ = os.Remove(path)
		}
	}
}

// InitializeRepo scaffolds EMBER.md, .ember.json, .gitignore entries, and the
// .ember/ directory under cwd. It is idempotent: existing files are preserved
// and reported as skipped. On error, partial changes are rolled back.
func InitializeRepo(cwd string) (InitReport, error) {
	var transaction initTransaction
	report, err := initializeRepoInner(cwd, &transaction)
	if err != nil {
		transaction.rollback()
		return InitReport{}, err
	}
	return report, nil
}

func initializeRepoInner(cwd string, transaction *initTransaction) (InitReport, error) {
	artifacts := make([]InitArtifact, 0, 4)

	emberDir := filepath.Join(cwd, ".ember")
	status, err := ensureDir(emberDir, transaction)
	if err != nil {
		return InitReport{}, fmt.Errorf("init: ensure .ember directory: %w", err)
	}
	artifacts = append(artifacts, InitArtifact{Name: ".ember/", Status: status})

	emberJSON := filepath.Join(cwd, ".ember.json")
	status, err = writeFileIfMissing(emberJSON, starterEmberJSON, transaction)
	if err != nil {
		return InitReport{}, fmt.Errorf("init: write .ember.json: %w", err)
	}
	artifacts = append(artifacts, InitArtifact{Name: ".ember.json", Status: status})

	gitignore := filepath.Join(cwd, ".gitignore")
	status, err = ensureGitignoreEntries(gitignore, transaction)
	if err != nil {
		return InitReport{}, fmt.Errorf("init: ensure .gitignore entries: %w", err)
	}
	artifacts = append(artifacts, InitArtifact{Name: ".gitignore", Status: status})

	emberMD := filepath.Join(cwd, "EMBER.md")
	content := RenderInitEmberMD(cwd)
	status, err = writeFileIfMissing(emberMD, content, transaction)
	if err != nil {
		return InitReport{}, fmt.Errorf("init: write EMBER.md: %w", err)
	}
	artifacts = append(artifacts, InitArtifact{Name: "EMBER.md", Status: status})

	return InitReport{ProjectRoot: cwd, Artifacts: artifacts}, nil
}

func ensureDir(path string, transaction *initTransaction) (InitStatus, error) {
	if info, err := os.Stat(path); err == nil && info.IsDir() {
		return InitStatusSkipped, nil
	}
	if err := os.MkdirAll(path, 0o755); err != nil {
		return "", err
	}
	transaction.createdPaths = append(transaction.createdPaths, path)
	return InitStatusCreated, nil
}

func writeFileIfMissing(path, content string, transaction *initTransaction) (InitStatus, error) {
	if _, err := os.Stat(path); err == nil {
		return InitStatusSkipped, nil
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		return "", err
	}
	transaction.createdPaths = append(transaction.createdPaths, path)
	return InitStatusCreated, nil
}

func ensureGitignoreEntries(path string, transaction *initTransaction) (InitStatus, error) {
	existing, err := os.ReadFile(path)
	if err != nil {
		if !os.IsNotExist(err) {
			return "", err
		}
		lines := append([]string{gitignoreComment}, gitignoreEntries...)
		if writeErr := os.WriteFile(path, []byte(strings.Join(lines, "\n")+"\n"), 0o644); writeErr != nil {
			return "", writeErr
		}
		transaction.createdPaths = append(transaction.createdPaths, path)
		return InitStatusCreated, nil
	}

	lines := splitLines(string(existing))
	present := make(map[string]bool, len(lines))
	for _, line := range lines {
		present[line] = true
	}

	changed := false
	if !present[gitignoreComment] {
		lines = append(lines, gitignoreComment)
		present[gitignoreComment] = true
		changed = true
	}
	for _, entry := range gitignoreEntries {
		if !present[entry] {
			lines = append(lines, entry)
			present[entry] = true
			changed = true
		}
	}

	if !changed {
		return InitStatusSkipped, nil
	}

	transaction.replacedFiles = append(transaction.replacedFiles, replacedFile{path: path, content: string(existing)})
	if writeErr := os.WriteFile(path, []byte(strings.Join(lines, "\n")+"\n"), 0o644); writeErr != nil {
		return "", writeErr
	}
	return InitStatusUpdated, nil
}

func splitLines(content string) []string {
	scanner := bufio.NewScanner(strings.NewReader(content))
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	lines := []string{}
	for scanner.Scan() {
		lines = append(lines, scanner.Text())
	}
	return lines
}

// RenderInitEmberMD renders the EMBER.md template, including detected stack,
// verification, repository shape, and framework notes. It mirrors the Rust
// render_init_ember_md function.
func RenderInitEmberMD(cwd string) string {
	detection := detectRepo(cwd)
	lines := []string{
		"# EMBER.md",
		"",
		"This file provides guidance to Emberforge (emberforge.dev) when working with code in this repository.",
		"",
	}

	languages := detectedLanguages(detection)
	frameworks := detectedFrameworks(detection)
	lines = append(lines, "## Detected stack")
	if len(languages) == 0 {
		lines = append(lines, "- No specific language markers were detected yet; document the primary language and verification commands once the project structure settles.")
	} else {
		lines = append(lines, fmt.Sprintf("- Languages: %s.", strings.Join(languages, ", ")))
	}
	if len(frameworks) == 0 {
		lines = append(lines, "- Frameworks: none detected from the supported starter markers.")
	} else {
		lines = append(lines, fmt.Sprintf("- Frameworks/tooling markers: %s.", strings.Join(frameworks, ", ")))
	}
	lines = append(lines, "")

	if verification := verificationLines(cwd, detection); len(verification) > 0 {
		lines = append(lines, "## Verification")
		lines = append(lines, verification...)
		lines = append(lines, "")
	}

	if shape := repositoryShapeLines(detection); len(shape) > 0 {
		lines = append(lines, "## Repository shape")
		lines = append(lines, shape...)
		lines = append(lines, "")
	}

	if notes := frameworkNotes(detection); len(notes) > 0 {
		lines = append(lines, "## Framework notes")
		lines = append(lines, notes...)
		lines = append(lines, "")
	}

	lines = append(lines,
		"## Working agreement",
		"- Prefer small, reviewable changes and keep generated bootstrap files aligned with actual repo workflows.",
		"- Keep shared defaults in `.ember.json`; reserve `.ember/settings.local.json` for machine-local overrides.",
		"- Do not overwrite existing `EMBER.md` content automatically; update it intentionally when repo workflows change.",
		"",
	)

	return strings.Join(lines, "\n")
}

func detectRepo(cwd string) repoDetection {
	packageJSONContents := strings.ToLower(readFileOrEmpty(filepath.Join(cwd, "package.json")))
	return repoDetection{
		rustWorkspace: isFile(filepath.Join(cwd, "Cargo.toml")) && isDir(filepath.Join(cwd, "crates")),
		rustRoot:      isFile(filepath.Join(cwd, "Cargo.toml")),
		python: isFile(filepath.Join(cwd, "pyproject.toml")) ||
			isFile(filepath.Join(cwd, "requirements.txt")) ||
			isFile(filepath.Join(cwd, "setup.py")),
		packageJSON: isFile(filepath.Join(cwd, "package.json")),
		typescript: isFile(filepath.Join(cwd, "tsconfig.json")) ||
			strings.Contains(packageJSONContents, "typescript"),
		nextjs:   strings.Contains(packageJSONContents, "\"next\""),
		react:    strings.Contains(packageJSONContents, "\"react\""),
		vite:     strings.Contains(packageJSONContents, "\"vite\""),
		nest:     strings.Contains(packageJSONContents, "@nestjs"),
		srcDir:   isDir(filepath.Join(cwd, "src")),
		testsDir: isDir(filepath.Join(cwd, "tests")),
		rustDir:  isDir(filepath.Join(cwd, "crates")),
	}
}

func detectedLanguages(detection repoDetection) []string {
	languages := []string{}
	if detection.rustWorkspace || detection.rustRoot {
		languages = append(languages, "Rust")
	}
	if detection.python {
		languages = append(languages, "Python")
	}
	if detection.typescript {
		languages = append(languages, "TypeScript")
	} else if detection.packageJSON {
		languages = append(languages, "JavaScript/Node.js")
	}
	return languages
}

func detectedFrameworks(detection repoDetection) []string {
	frameworks := []string{}
	if detection.nextjs {
		frameworks = append(frameworks, "Next.js")
	}
	if detection.react {
		frameworks = append(frameworks, "React")
	}
	if detection.vite {
		frameworks = append(frameworks, "Vite")
	}
	if detection.nest {
		frameworks = append(frameworks, "NestJS")
	}
	return frameworks
}

func verificationLines(cwd string, detection repoDetection) []string {
	lines := []string{}
	if detection.rustWorkspace || detection.rustRoot {
		lines = append(lines, "- Run Rust verification from the repo root: `cargo fmt`, `cargo clippy --workspace --all-targets -- -D warnings`, `cargo test --workspace`")
	}
	if detection.python {
		if isFile(filepath.Join(cwd, "pyproject.toml")) {
			lines = append(lines, "- Run the Python project checks declared in `pyproject.toml` (for example: `pytest`, `ruff check`, and `mypy` when configured).")
		} else {
			lines = append(lines, "- Run the repo's Python test/lint commands before shipping changes.")
		}
	}
	if detection.packageJSON {
		lines = append(lines, "- Run the JavaScript/TypeScript checks from `package.json` before shipping changes (`npm test`, `npm run lint`, `npm run build`, or the repo equivalent).")
	}
	if detection.testsDir && detection.srcDir {
		lines = append(lines, "- `src/` and `tests/` are both present; update both surfaces together when behavior changes.")
	}
	return lines
}

func repositoryShapeLines(detection repoDetection) []string {
	lines := []string{}
	if detection.rustDir {
		lines = append(lines, "- `crates/` contains the Rust workspace and active CLI/runtime implementation.")
	}
	if detection.srcDir {
		lines = append(lines, "- `src/` contains source files that should stay consistent with generated guidance and tests.")
	}
	if detection.testsDir {
		lines = append(lines, "- `tests/` contains validation surfaces that should be reviewed alongside code changes.")
	}
	return lines
}

func frameworkNotes(detection repoDetection) []string {
	lines := []string{}
	if detection.nextjs {
		lines = append(lines, "- Next.js detected: preserve routing/data-fetching conventions and verify production builds after changing app structure.")
	}
	if detection.react && !detection.nextjs {
		lines = append(lines, "- React detected: keep component behavior covered with focused tests and avoid unnecessary prop/API churn.")
	}
	if detection.vite {
		lines = append(lines, "- Vite detected: validate the production bundle after changing build-sensitive configuration or imports.")
	}
	if detection.nest {
		lines = append(lines, "- NestJS detected: keep module/provider boundaries explicit and verify controller/service wiring after refactors.")
	}
	return lines
}

func readFileOrEmpty(path string) string {
	raw, err := os.ReadFile(path)
	if err != nil {
		return ""
	}
	return string(raw)
}

func isFile(path string) bool {
	info, err := os.Stat(path)
	return err == nil && !info.IsDir()
}

func isDir(path string) bool {
	info, err := os.Stat(path)
	return err == nil && info.IsDir()
}
