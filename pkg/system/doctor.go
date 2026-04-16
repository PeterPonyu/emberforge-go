package system

import (
	"fmt"
	"os"
	"strings"
)

func envPresence(value string) string {
	if strings.TrimSpace(value) == "" {
		return "missing"
	}
	return "present"
}

func BuildDoctorReport(report StarterSystemReport) string {
	model := os.Getenv("OLLAMA_MODEL")
	if strings.TrimSpace(model) == "" {
		model = os.Getenv("EMBER_MODEL")
	}
	if strings.TrimSpace(model) == "" {
		model = "qwen3:8b"
	}

	baseURL := os.Getenv("OLLAMA_BASE_URL")
	if strings.TrimSpace(baseURL) == "" {
		baseURL = "http://localhost:11434"
	}

	lines := []string{
		"emberforge-go doctor",
		"provider: ollama",
		fmt.Sprintf("base_url: %s", baseURL),
		fmt.Sprintf("model: %s", model),
		fmt.Sprintf("anthropic_api_key: %s", envPresence(os.Getenv("ANTHROPIC_API_KEY"))),
		fmt.Sprintf("xai_api_key: %s", envPresence(os.Getenv("XAI_API_KEY"))),
		fmt.Sprintf("commands: %d", report.CommandCount),
		fmt.Sprintf("tools: %d", report.ToolCount),
		fmt.Sprintf("plugins: %d", report.PluginCount),
		fmt.Sprintf("server: %s", report.ServerDescription),
		fmt.Sprintf("lsp: %s", report.LSPSummary),
		fmt.Sprintf("lifecycle: %s", report.LifecycleState),
	}
	return strings.Join(lines, "\n")
}
