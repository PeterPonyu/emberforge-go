package api

import (
	"fmt"
	"os"
	"strings"
)

// AuthSource describes the credential material resolved for a hosted provider.
// It mirrors the Rust runtime's AuthSource enum (api_key / bearer_token /
// both / none) so request construction can attach the correct headers.
type AuthSource struct {
	APIKey      string
	BearerToken string
}

// HasCredentials reports whether any usable credential is present.
func (a AuthSource) HasCredentials() bool {
	return a.APIKey != "" || a.BearerToken != ""
}

// readEnvNonEmpty returns the trimmed value of key when it is set to a
// non-empty value, mirroring the Rust read_env_non_empty helper.
func readEnvNonEmpty(key string) string {
	return strings.TrimSpace(os.Getenv(key))
}

// resolveAnthropicAuth mirrors the Rust resolve_startup_auth_source precedence
// for the Anthropic ("Claw") provider: ANTHROPIC_API_KEY (optionally combined
// with ANTHROPIC_AUTH_TOKEN as a bearer), then ANTHROPIC_AUTH_TOKEN alone. The
// optional .ember settings provide a fallback when no env credentials exist.
func resolveAnthropicAuth(settings EmberSettings) (AuthSource, error) {
	apiKey := readEnvNonEmpty("ANTHROPIC_API_KEY")
	authToken := readEnvNonEmpty("ANTHROPIC_AUTH_TOKEN")

	if apiKey != "" {
		return AuthSource{APIKey: apiKey, BearerToken: authToken}, nil
	}
	if authToken != "" {
		return AuthSource{BearerToken: authToken}, nil
	}
	if key := strings.TrimSpace(settings.AnthropicAPIKey); key != "" {
		return AuthSource{APIKey: key}, nil
	}
	return AuthSource{}, fmt.Errorf("anthropic: missing credentials (set ANTHROPIC_API_KEY or ANTHROPIC_AUTH_TOKEN)")
}

// resolveXAIAuth mirrors the precedence for the xAI provider: XAI_API_KEY env
// first, then the .ember settings fallback.
func resolveXAIAuth(settings EmberSettings) (AuthSource, error) {
	if apiKey := readEnvNonEmpty("XAI_API_KEY"); apiKey != "" {
		return AuthSource{APIKey: apiKey}, nil
	}
	if key := strings.TrimSpace(settings.XAIAPIKey); key != "" {
		return AuthSource{APIKey: key}, nil
	}
	return AuthSource{}, fmt.Errorf("xai: missing credentials (set XAI_API_KEY)")
}
