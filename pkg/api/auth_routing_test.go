package api

import (
	"os"
	"path/filepath"
	"testing"
)

func clearProviderEnv(t *testing.T) {
	t.Helper()
	for _, key := range []string{
		"ANTHROPIC_API_KEY", "ANTHROPIC_AUTH_TOKEN", "ANTHROPIC_BASE_URL",
		"XAI_API_KEY", "XAI_BASE_URL", "OPENAI_API_KEY",
	} {
		t.Setenv(key, "")
	}
}

func TestResolveAnthropicAuth_PrefersAPIKeyAndCombinesBearer(t *testing.T) {
	clearProviderEnv(t)
	t.Setenv("ANTHROPIC_API_KEY", "key-123")
	t.Setenv("ANTHROPIC_AUTH_TOKEN", "bearer-abc")

	auth, err := resolveAnthropicAuth(EmberSettings{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if auth.APIKey != "key-123" || auth.BearerToken != "bearer-abc" {
		t.Fatalf("got %+v want apiKey=key-123 bearer=bearer-abc", auth)
	}
}

func TestResolveAnthropicAuth_BearerOnly(t *testing.T) {
	clearProviderEnv(t)
	t.Setenv("ANTHROPIC_AUTH_TOKEN", "bearer-only")

	auth, err := resolveAnthropicAuth(EmberSettings{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if auth.APIKey != "" || auth.BearerToken != "bearer-only" {
		t.Fatalf("got %+v want bearer-only", auth)
	}
}

func TestResolveAnthropicAuth_SettingsFallback(t *testing.T) {
	clearProviderEnv(t)
	auth, err := resolveAnthropicAuth(EmberSettings{AnthropicAPIKey: "settings-key"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if auth.APIKey != "settings-key" {
		t.Fatalf("got %+v want settings-key", auth)
	}
}

func TestResolveAnthropicAuth_MissingErrors(t *testing.T) {
	clearProviderEnv(t)
	if _, err := resolveAnthropicAuth(EmberSettings{}); err == nil {
		t.Fatal("expected error when no credentials present")
	}
}

func TestResolveXAIAuth_EnvThenSettings(t *testing.T) {
	clearProviderEnv(t)
	t.Setenv("XAI_API_KEY", "xai-env")
	auth, err := resolveXAIAuth(EmberSettings{XAIAPIKey: "xai-settings"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if auth.APIKey != "xai-env" {
		t.Fatalf("env should win, got %+v", auth)
	}

	clearProviderEnv(t)
	auth, err = resolveXAIAuth(EmberSettings{XAIAPIKey: "xai-settings"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if auth.APIKey != "xai-settings" {
		t.Fatalf("settings fallback failed, got %+v", auth)
	}
}

func TestResolveModelAlias(t *testing.T) {
	cases := map[string]string{
		"opus":      "claude-opus-4-6",
		"sonnet":    "claude-sonnet-4-6",
		"grok":      "grok-3",
		"grok-mini": "grok-3-mini",
		"qwen3:8b":  "qwen3:8b", // unknown passes through unchanged
	}
	for in, want := range cases {
		if got := ResolveModelAlias(in); got != want {
			t.Errorf("ResolveModelAlias(%q)=%q want %q", in, got, want)
		}
	}
}

func TestDetectProviderKind_ByModelAlias(t *testing.T) {
	clearProviderEnv(t)
	if got := DetectProviderKind("opus", EmberSettings{}); got != ProviderKindAnthropic {
		t.Errorf("opus -> %q want anthropic", got)
	}
	if got := DetectProviderKind("grok", EmberSettings{}); got != ProviderKindXAI {
		t.Errorf("grok -> %q want xai", got)
	}
}

func TestDetectProviderKind_DefaultsToOllama(t *testing.T) {
	clearProviderEnv(t)
	if got := DetectProviderKind("some-local-model", EmberSettings{}); got != ProviderKindOllama {
		t.Errorf("unknown with no creds -> %q want ollama", got)
	}
}

func TestDetectProviderKind_CredentialFallback(t *testing.T) {
	clearProviderEnv(t)
	t.Setenv("XAI_API_KEY", "xai-key")
	if got := DetectProviderKind("unknown-model", EmberSettings{}); got != ProviderKindXAI {
		t.Errorf("xai key present -> %q want xai", got)
	}

	clearProviderEnv(t)
	t.Setenv("ANTHROPIC_API_KEY", "anthropic-key")
	if got := DetectProviderKind("unknown-model", EmberSettings{}); got != ProviderKindAnthropic {
		t.Errorf("anthropic key present -> %q want anthropic", got)
	}
}

func TestNewProviderForModel_Routing(t *testing.T) {
	clearProviderEnv(t)
	t.Setenv("ANTHROPIC_API_KEY", "anthropic-key")
	p, err := NewProviderForModel("opus", EmberSettings{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	ap, ok := p.(*AnthropicProvider)
	if !ok {
		t.Fatalf("opus should route to AnthropicProvider, got %T", p)
	}
	if ap.Model != "claude-opus-4-6" {
		t.Errorf("expected canonical model, got %q", ap.Model)
	}

	clearProviderEnv(t)
	t.Setenv("XAI_API_KEY", "xai-key")
	p, err = NewProviderForModel("grok", EmberSettings{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if _, ok := p.(*OpenAICompatProvider); !ok {
		t.Fatalf("grok should route to OpenAICompatProvider, got %T", p)
	}

	clearProviderEnv(t)
	p, err = NewProviderForModel("qwen3:8b", EmberSettings{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if _, ok := p.(*OllamaProvider); !ok {
		t.Fatalf("default should route to OllamaProvider, got %T", p)
	}
}

func TestLoadEmberSettings_MissingFileIsEmpty(t *testing.T) {
	dir := t.TempDir()
	settings, err := LoadEmberSettings(dir)
	if err != nil {
		t.Fatalf("missing file should not error: %v", err)
	}
	if settings != (EmberSettings{}) {
		t.Fatalf("expected empty settings, got %+v", settings)
	}
}

func TestLoadEmberSettings_ParsesFile(t *testing.T) {
	dir := t.TempDir()
	emberDir := filepath.Join(dir, ".ember")
	if err := os.MkdirAll(emberDir, 0o755); err != nil {
		t.Fatalf("setup: %v", err)
	}
	content := `{"anthropicApiKey":"a-key","xaiApiKey":"x-key"}`
	if err := os.WriteFile(filepath.Join(emberDir, "settings.json"), []byte(content), 0o644); err != nil {
		t.Fatalf("setup: %v", err)
	}
	settings, err := LoadEmberSettings(dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if settings.AnthropicAPIKey != "a-key" || settings.XAIAPIKey != "x-key" {
		t.Fatalf("got %+v", settings)
	}
}
