package tools

import (
	"strings"
	"testing"
)

// TestEncodeToolInput verifies structured tool-call arguments are bridged into
// the single-string input contract each tool's Execute path expects.
func TestEncodeToolInput(t *testing.T) {
	cases := []struct {
		name string
		tool string
		args map[string]any
		want string
	}{
		{"bash", "bash", map[string]any{"command": "ls -la"}, "ls -la"},
		{"read_file", "read_file", map[string]any{"path": "/tmp/x"}, "/tmp/x"},
		{"write_file", "write_file", map[string]any{"path": "a.txt", "content": "body"}, "a.txt:body"},
		{"edit_file", "edit_file", map[string]any{"path": "a.txt", "old_string": "old", "new_string": "new"}, "a.txt\x00old\x00new"},
		{"glob_search", "glob_search", map[string]any{"pattern": "*.go"}, "*.go"},
		{"grep_search no path", "grep_search", map[string]any{"pattern": "needle"}, "needle"},
		{"grep_search with path", "grep_search", map[string]any{"pattern": "needle", "path": "sub"}, "needle\x00sub"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := EncodeToolInput(tc.tool, tc.args); got != tc.want {
				t.Fatalf("EncodeToolInput(%q) = %q, want %q", tc.tool, got, tc.want)
			}
		})
	}
}

// TestEncodeToolInput_NonStringScalar verifies non-string JSON scalars (e.g. an
// integer timeout) are coerced rather than dropped.
func TestEncodeToolInput_CoercesScalars(t *testing.T) {
	got := EncodeToolInput("bash", map[string]any{"command": 42})
	if got != "42" {
		t.Fatalf("got %q, want %q", got, "42")
	}
}

// TestEncodeToolInput_DeferredToolJSON verifies deferred tools (web/notebook/
// agent/skill) receive a compact JSON encoding of their arguments so the
// receipt path stays observable.
func TestEncodeToolInput_DeferredToolJSON(t *testing.T) {
	got := EncodeToolInput("web", map[string]any{"url": "https://example.com"})
	if !strings.Contains(got, "https://example.com") || !strings.HasPrefix(got, "{") {
		t.Fatalf("expected JSON-encoded args, got %q", got)
	}
	if EncodeToolInput("skill", nil) != "" {
		t.Fatalf("expected empty string for nil args")
	}
}
