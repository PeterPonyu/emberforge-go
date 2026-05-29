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
// matches the boundary error shared by read_file and write_file. Comparison is
// done on cleaned path boundaries to avoid prefix false positives such as
// "/ws" matching "/ws-other".
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

	if !strings.HasPrefix(absPath, absRoot+string(filepath.Separator)) && absPath != absRoot {
		return "", fmt.Errorf("path %q is outside workspace %q", absPath, absRoot)
	}
	return absPath, nil
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
