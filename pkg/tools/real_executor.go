package tools

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
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
	case "bash":
		return e.bash(input)
	default:
		return fmt.Sprintf("[real tool] unknown tool: %s", toolName)
	}
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
