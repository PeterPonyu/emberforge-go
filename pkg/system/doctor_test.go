package system

import (
	"strings"
	"testing"
)

func TestBuildDoctorReport(t *testing.T) {
	t.Setenv("OLLAMA_BASE_URL", "http://localhost:11434")
	t.Setenv("OLLAMA_MODEL", "qwen3:8b")
	t.Setenv("ANTHROPIC_API_KEY", "")
	t.Setenv("XAI_API_KEY", "token")

	report := BuildDoctorReport(StarterSystemReport{
		CommandCount:      11,
		ToolCount:         3,
		PluginCount:       1,
		ServerDescription: "server: disabled",
		LSPSummary:        "lsp: idle",
		LifecycleState:    "ready",
	})

	for _, expected := range []string{
		"emberforge-go doctor",
		"commands: 11",
		"xai_api_key: present",
		"anthropic_api_key: missing",
	} {
		if !strings.Contains(report, expected) {
			t.Fatalf("expected report to contain %q, got:\n%s", expected, report)
		}
	}
}
