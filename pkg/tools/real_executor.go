package tools

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"time"
)

const (
	maxReadFileBytes = 10 * 1024 * 1024 // 10 MB
	bashTimeout      = 30 * time.Second
)

type RealToolExecutor struct {
	workspaceRoot string
}

func NewRealToolExecutor(root string) *RealToolExecutor {
	return &RealToolExecutor{workspaceRoot: root}
}

func (e *RealToolExecutor) Execute(toolName string, input string) string {
	switch toolName {
	case "read_file":
		return e.readFile(input)
	case "write_file":
		return e.writeFile(input)
	case "edit_file":
		return e.editFile(input)
	case "glob_search":
		return e.globSearch(input)
	case "grep_search":
		return e.grepSearch(input)
	case "bash":
		return e.bash(input)
	case "web", "notebook", "agent", "skill":
		// These tools are structurally defined and permission-gated in the
		// registry; their full execution is deferred. Return a structured
		// receipt so the dispatch path is observable end-to-end.
		return fmt.Sprintf("[%s] not yet executable in go runtime; input=%s", toolName, input)
	default:
		return fmt.Sprintf("[real tool] unknown tool: %s", toolName)
	}
}

// ExecuteWithPermission gates Execute behind the registry-declared permission
// for toolName: the tool only runs when activeMode satisfies the tool's
// required permission, mirroring the Rust mode-based permission check. Unknown
// tools are reported without running. The boolean reports whether the tool was
// permitted to run.
func (e *RealToolExecutor) ExecuteWithPermission(registry ToolRegistry, toolName, input string, activeMode PermissionMode) (string, bool) {
	spec, ok := registry.Find(toolName)
	if !ok {
		return fmt.Sprintf("[permission error] unknown tool: %s", toolName), false
	}
	if !activeMode.Allows(spec.RequiredPermission) {
		return fmt.Sprintf(
			"[permission denied] tool %q requires %s; current mode is %s",
			toolName, spec.RequiredPermission, activeMode,
		), false
	}
	return e.Execute(toolName, input), true
}

// resolveWithinWorkspace absolutizes path and verifies it stays within the
// configured workspace root (an empty root falls back to the current working
// directory). It returns the cleaned absolute path, or an error whose message
// matches the boundary error shared by read_file and write_file.
//
// The check is symlink-safe: both the workspace root and the target are
// resolved with filepath.EvalSymlinks before comparison. Because the target
// may not exist yet (write_file can create new files), the nearest existing
// ancestor of the target is resolved and the remaining not-yet-existing
// suffix is recomposed onto it. This prevents an in-workspace symlink from
// pointing at an outside target and slipping past a string-only prefix check.
// Comparison is done on cleaned path boundaries to avoid prefix false
// positives such as "/ws" matching "/ws-other".
func (e *RealToolExecutor) resolveWithinWorkspace(path string) (string, error) {
	absPath, err := filepath.Abs(path)
	if err != nil {
		return "", fmt.Errorf("abs path: %w", err)
	}

	root := e.workspaceRoot
	if root == "" {
		cwd, err := os.Getwd()
		if err != nil {
			return "", fmt.Errorf("getwd: %w", err)
		}
		root = cwd
	}
	absRoot, err := filepath.Abs(root)
	if err != nil {
		return "", fmt.Errorf("abs root: %w", err)
	}

	// Resolve the real workspace root so that comparisons are made against
	// canonical paths (e.g. when /tmp is itself a symlink).
	realRoot, err := filepath.EvalSymlinks(absRoot)
	if err != nil {
		return "", fmt.Errorf("resolve workspace root: %w", err)
	}

	// Resolve the real target path. The target may not exist, so walk up to
	// the nearest existing ancestor, resolve symlinks there, then re-append
	// the not-yet-existing suffix.
	realPath, err := resolveExistingPrefix(absPath)
	if err != nil {
		return "", fmt.Errorf("resolve path: %w", err)
	}

	if !withinRoot(realPath, realRoot) {
		return "", fmt.Errorf("path %q is outside workspace %q", absPath, absRoot)
	}
	return realPath, nil
}

// resolveExistingPrefix resolves symlinks for the longest existing prefix of
// absPath and recomposes the remaining (non-existent) trailing components, so
// a canonical absolute path is returned even when the leaf does not yet exist.
func resolveExistingPrefix(absPath string) (string, error) {
	cur := filepath.Clean(absPath)
	var suffix []string
	for {
		resolved, err := filepath.EvalSymlinks(cur)
		if err == nil {
			if len(suffix) == 0 {
				return resolved, nil
			}
			parts := append([]string{resolved}, suffix...)
			return filepath.Join(parts...), nil
		}
		if !os.IsNotExist(err) {
			return "", err
		}
		parent := filepath.Dir(cur)
		if parent == cur {
			// Reached the filesystem root without finding an existing path.
			return "", err
		}
		suffix = append([]string{filepath.Base(cur)}, suffix...)
		cur = parent
	}
}

// withinRoot reports whether the cleaned path p is the root itself or nested
// strictly inside root, using a separator-terminated prefix to avoid false
// positives like "/ws" matching "/ws-other".
func withinRoot(p, root string) bool {
	return p == root || strings.HasPrefix(p, root+string(filepath.Separator))
}

func (e *RealToolExecutor) readFile(path string) string {
	absPath, err := e.resolveWithinWorkspace(path)
	if err != nil {
		return fmt.Sprintf("[read_file error] %s", err.Error())
	}

	info, err := os.Stat(absPath)
	if err != nil {
		return fmt.Sprintf("[read_file error] stat: %s", err.Error())
	}
	if info.Size() > maxReadFileBytes {
		return fmt.Sprintf("[read_file error] file too large: %d bytes (max %d)", info.Size(), maxReadFileBytes)
	}
	data, err := os.ReadFile(absPath)
	if err != nil {
		return fmt.Sprintf("[read_file error] read: %s", err.Error())
	}
	return string(data)
}

func (e *RealToolExecutor) writeFile(input string) string {
	// input format: "path:content"
	sep := strings.SplitN(input, ":", 2)
	if len(sep) != 2 {
		return "[write_file error] input must be 'path:content'"
	}
	path, content := sep[0], sep[1]

	absPath, err := e.resolveWithinWorkspace(path)
	if err != nil {
		return fmt.Sprintf("[write_file error] %s", err.Error())
	}

	if err := os.WriteFile(absPath, []byte(content), 0644); err != nil {
		return fmt.Sprintf("[write_file error] write: %s", err.Error())
	}
	return fmt.Sprintf("[write_file] wrote %d bytes to %s", len(content), absPath)
}

// editFile performs an exact string replacement in a workspace file. Input
// format: "path\x00old\x00new" (NUL-separated) to allow arbitrary content;
// the old string must be present, else an error is returned.
func (e *RealToolExecutor) editFile(input string) string {
	parts := strings.SplitN(input, "\x00", 3)
	if len(parts) != 3 {
		return "[edit_file error] input must be 'path\\x00old_string\\x00new_string'"
	}
	path, oldString, newString := parts[0], parts[1], parts[2]

	absPath, err := e.resolveWithinWorkspace(path)
	if err != nil {
		return fmt.Sprintf("[edit_file error] %s", err.Error())
	}
	data, err := os.ReadFile(absPath)
	if err != nil {
		return fmt.Sprintf("[edit_file error] read: %s", err.Error())
	}
	content := string(data)
	if !strings.Contains(content, oldString) {
		return "[edit_file error] old_string not found in file"
	}
	updated := strings.Replace(content, oldString, newString, 1)
	if err := os.WriteFile(absPath, []byte(updated), 0644); err != nil {
		return fmt.Sprintf("[edit_file error] write: %s", err.Error())
	}
	return fmt.Sprintf("[edit_file] updated %s", absPath)
}

// workspaceRootDir returns the configured workspace root as an absolute path,
// falling back to the current working directory when no root is set. Unlike
// resolveWithinWorkspace it interprets relative subpaths against the root
// rather than the process cwd, which is what the directory-walking tools need.
func (e *RealToolExecutor) workspaceRootDir(sub string) (string, error) {
	root := e.workspaceRoot
	if root == "" {
		cwd, err := os.Getwd()
		if err != nil {
			return "", fmt.Errorf("getwd: %w", err)
		}
		root = cwd
	}
	absRoot, err := filepath.Abs(root)
	if err != nil {
		return "", fmt.Errorf("abs root: %w", err)
	}
	if sub == "" || sub == "." {
		return absRoot, nil
	}
	joined := filepath.Clean(filepath.Join(absRoot, sub))
	if !withinRoot(joined, absRoot) {
		return "", fmt.Errorf("path %q is outside workspace %q", joined, absRoot)
	}
	return joined, nil
}

// globSearch lists workspace files matching a shell glob pattern. The pattern
// is resolved relative to the workspace root and results are confined to it.
func (e *RealToolExecutor) globSearch(pattern string) string {
	root, err := e.workspaceRootDir(".")
	if err != nil {
		return fmt.Sprintf("[glob_search error] %s", err.Error())
	}
	matches, err := filepath.Glob(filepath.Join(root, pattern))
	if err != nil {
		return fmt.Sprintf("[glob_search error] %s", err.Error())
	}
	var kept []string
	for _, m := range matches {
		if withinRoot(filepath.Clean(m), root) {
			kept = append(kept, m)
		}
	}
	if len(kept) == 0 {
		return "[glob_search] no matches"
	}
	return strings.Join(kept, "\n")
}

// grepSearch walks the workspace and returns "path:lineno:line" for every line
// matching the regex pattern. Input format: "pattern" or "pattern\x00subdir".
func (e *RealToolExecutor) grepSearch(input string) string {
	pattern := input
	sub := "."
	if parts := strings.SplitN(input, "\x00", 2); len(parts) == 2 {
		pattern, sub = parts[0], parts[1]
	}

	re, err := regexp.Compile(pattern)
	if err != nil {
		return fmt.Sprintf("[grep_search error] invalid pattern: %s", err.Error())
	}
	root, err := e.workspaceRootDir(sub)
	if err != nil {
		return fmt.Sprintf("[grep_search error] %s", err.Error())
	}

	var results []string
	walkErr := filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() {
			return nil
		}
		file, err := os.Open(path)
		if err != nil {
			return nil
		}
		defer file.Close()
		scanner := bufio.NewScanner(file)
		scanner.Buffer(make([]byte, 256*1024), 256*1024)
		lineNo := 0
		for scanner.Scan() {
			lineNo++
			line := scanner.Text()
			if re.MatchString(line) {
				results = append(results, fmt.Sprintf("%s:%d:%s", path, lineNo, line))
			}
		}
		return nil
	})
	if walkErr != nil {
		return fmt.Sprintf("[grep_search error] %s", walkErr.Error())
	}
	if len(results) == 0 {
		return "[grep_search] no matches"
	}
	return strings.Join(results, "\n")
}

func (e *RealToolExecutor) bash(command string) string {
	ctx, cancel := context.WithTimeout(context.Background(), bashTimeout)
	defer cancel()
	cmd := exec.CommandContext(ctx, "sh", "-c", command)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Sprintf("[bash error] %s\n%s", err.Error(), string(out))
	}
	return string(out)
}
