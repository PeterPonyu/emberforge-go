package api

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// ProviderKind identifies the hosted (or local) backend selected for a model.
// It mirrors the Rust ProviderKind enum.
type ProviderKind string

const (
	ProviderKindAnthropic ProviderKind = "anthropic"
	ProviderKindXAI       ProviderKind = "xai"
	ProviderKindOllama    ProviderKind = "ollama"
)

// EmberSettings holds the subset of .ember/settings.json credential fields the
// provider-detection layer consults as a fallback when env vars are absent.
type EmberSettings struct {
	AnthropicAPIKey string `json:"anthropicApiKey"`
	XAIAPIKey       string `json:"xaiApiKey"`
}

// LoadEmberSettings reads .ember/settings.json from dir (defaulting to the
// current working directory). A missing file yields an empty, non-error result
// so detection degrades gracefully to env-only resolution.
func LoadEmberSettings(dir string) (EmberSettings, error) {
	if dir == "" {
		cwd, err := os.Getwd()
		if err != nil {
			return EmberSettings{}, fmt.Errorf("ember settings: getwd: %w", err)
		}
		dir = cwd
	}
	path := filepath.Join(dir, ".ember", "settings.json")
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return EmberSettings{}, nil
		}
		return EmberSettings{}, fmt.Errorf("ember settings: read %s: %w", path, err)
	}
	var settings EmberSettings
	if err := json.Unmarshal(data, &settings); err != nil {
		return EmberSettings{}, fmt.Errorf("ember settings: parse %s: %w", path, err)
	}
	return settings, nil
}

// modelAlias maps a normalized (lowercased, trimmed) model alias to its
// canonical model id and owning provider, mirroring the Rust MODEL_REGISTRY +
// resolve_model_alias behavior for the providers ported here.
type modelAlias struct {
	canonical string
	provider  ProviderKind
}

var modelRegistry = map[string]modelAlias{
	"opus":                      {canonical: "claude-opus-4-6", provider: ProviderKindAnthropic},
	"sonnet":                    {canonical: "claude-sonnet-4-6", provider: ProviderKindAnthropic},
	"haiku":                     {canonical: "claude-haiku-4-5-20251213", provider: ProviderKindAnthropic},
	"claude-opus-4-6":           {canonical: "claude-opus-4-6", provider: ProviderKindAnthropic},
	"claude-sonnet-4-6":         {canonical: "claude-sonnet-4-6", provider: ProviderKindAnthropic},
	"claude-haiku-4-5-20251213": {canonical: "claude-haiku-4-5-20251213", provider: ProviderKindAnthropic},
	"grok":                      {canonical: "grok-3", provider: ProviderKindXAI},
	"grok-3":                    {canonical: "grok-3", provider: ProviderKindXAI},
	"grok-mini":                 {canonical: "grok-3-mini", provider: ProviderKindXAI},
	"grok-3-mini":               {canonical: "grok-3-mini", provider: ProviderKindXAI},
	"grok-2":                    {canonical: "grok-2", provider: ProviderKindXAI},
}

// ResolveModelAlias returns the canonical model id for a known alias, or the
// trimmed input unchanged when no alias matches.
func ResolveModelAlias(model string) string {
	trimmed := strings.TrimSpace(model)
	if alias, ok := modelRegistry[strings.ToLower(trimmed)]; ok {
		return alias.canonical
	}
	return trimmed
}

// DetectProviderKind resolves which provider should serve model. Known model
// aliases route directly; otherwise the available credentials (env first, then
// .ember settings) decide, mirroring the Rust detect_provider_kind fallback.
// Ollama is the default when no hosted credentials are present.
func DetectProviderKind(model string, settings EmberSettings) ProviderKind {
	if alias, ok := modelRegistry[strings.ToLower(strings.TrimSpace(model))]; ok {
		return alias.provider
	}
	if readEnvNonEmpty("ANTHROPIC_API_KEY") != "" ||
		readEnvNonEmpty("ANTHROPIC_AUTH_TOKEN") != "" ||
		strings.TrimSpace(settings.AnthropicAPIKey) != "" {
		return ProviderKindAnthropic
	}
	if readEnvNonEmpty("XAI_API_KEY") != "" || strings.TrimSpace(settings.XAIAPIKey) != "" {
		return ProviderKindXAI
	}
	return ProviderKindOllama
}

// NewProviderForModel builds the Provider implementation that should serve
// model. The selected provider's credentials are resolved from env and the
// optional .ember settings fallback. Ollama (the default) needs no credentials.
func NewProviderForModel(model string, settings EmberSettings) (Provider, error) {
	resolved := ResolveModelAlias(model)
	switch DetectProviderKind(model, settings) {
	case ProviderKindAnthropic:
		auth, err := resolveAnthropicAuth(settings)
		if err != nil {
			return nil, err
		}
		return NewAnthropicProvider(auth, resolved), nil
	case ProviderKindXAI:
		auth, err := resolveXAIAuth(settings)
		if err != nil {
			return nil, err
		}
		return NewOpenAICompatProvider(XAIConfig(), auth, resolved), nil
	default:
		return NewOllamaProvider("", resolved), nil
	}
}
