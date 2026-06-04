package api

import (
	"encoding/json"
	"fmt"
	"net/http"
	"sort"
	"strings"
	"time"
)

// Model-routing defaults mirror the Rust reference's parse_strategy
// (crates/runtime/src/model_router.rs:113-125). They are NAMED constants so the
// fast/capable/local/cloud choices are never buried literals.
const (
	// autoFastModel serves SIMPLE queries under the "auto" strategy.
	autoFastModel = "qwen2.5:1.5b"
	// autoCapableModel serves MEDIUM/COMPLEX queries under the "auto" strategy.
	autoCapableModel = "qwen3:8b"
	// hybridLocalModel serves SIMPLE/MEDIUM queries under the "hybrid" strategy.
	hybridLocalModel = "qwen3:8b"
	// hybridCloudModel serves COMPLEX queries under the "hybrid" strategy.
	hybridCloudModel = "claude-sonnet-4-6"
)

// Complexity thresholds mirror estimate_complexity
// (crates/runtime/src/model_router.rs:77-98). NAMED so the heuristic is explicit.
const (
	// simpleMaxWords: queries with at most this many words and no code markers
	// are Simple.
	simpleMaxWords = 5
	// complexMinWords: queries longer than this are Complex regardless of markers.
	complexMinWords = 50
)

// ollamaTagsTimeout bounds the /api/tags model-listing request so a slow or
// unreachable Ollama degrades gracefully instead of hanging the CLI. Mirrors the
// Rust reference's 2s blocking-client timeout (model_profiles.rs:155-157).
const ollamaTagsTimeout = 2 * time.Second

// TaskComplexity is the estimated complexity of a user query, mirroring the
// Rust reference's TaskComplexity enum.
type TaskComplexity int

const (
	ComplexitySimple TaskComplexity = iota
	ComplexityMedium
	ComplexityComplex
)

// codeMarkers / multiStepMarkers are the surface-level signals that push a query
// toward Complex, ported verbatim from estimate_complexity.
var (
	codeMarkers      = []string{"```", "refactor", "architect", "implement", "design"}
	multiStepMarkers = []string{"then", "after that", "step by step", "and also", "first", "finally"}
)

// EstimateComplexity classifies a query from surface heuristics, mirroring the
// Rust reference's estimate_complexity exactly: short markerless queries are
// Simple; code/multi-step/long queries are Complex; everything else is Medium.
func EstimateComplexity(query string) TaskComplexity {
	words := len(strings.Fields(query))
	hasCodeMarkers := containsAny(query, codeMarkers)
	hasMultiStep := containsAny(query, multiStepMarkers)

	if words <= simpleMaxWords && !hasCodeMarkers {
		return ComplexitySimple
	}
	if hasCodeMarkers || hasMultiStep || words > complexMinWords {
		return ComplexityComplex
	}
	return ComplexityMedium
}

func containsAny(haystack string, needles []string) bool {
	for _, n := range needles {
		if strings.Contains(haystack, n) {
			return true
		}
	}
	return false
}

// RoutingMode is the model-selection mode of a RoutingStrategy.
type RoutingMode int

const (
	// RoutingFixed always selects FixedModel (or defers to the provider default
	// when FixedModel is empty). This is the zero value and default behavior.
	RoutingFixed RoutingMode = iota
	// RoutingAuto selects FastModel for Simple queries and CapableModel
	// otherwise.
	RoutingAuto
	// RoutingHybrid selects LocalModel for Simple/Medium queries and CloudModel
	// for Complex ones.
	RoutingHybrid
)

// RoutingStrategy selects the model for a turn based on its mode and the query's
// estimated complexity. It mirrors the Rust reference's RoutingStrategy enum
// (Fixed/Auto/Hybrid) as a single struct so the zero value is a no-op Fixed
// strategy that preserves existing behavior.
type RoutingStrategy struct {
	Mode         RoutingMode
	FixedModel   string
	FastModel    string
	CapableModel string
	LocalModel   string
	CloudModel   string
}

// ParseRoutingStrategy maps a model string to a strategy, mirroring the Rust
// reference's parse_strategy: "auto"/"hybrid" select the named multi-model
// strategies; anything else is a Fixed strategy on that model string.
func ParseRoutingStrategy(modelStr string) RoutingStrategy {
	switch strings.ToLower(strings.TrimSpace(modelStr)) {
	case "auto":
		return RoutingStrategy{Mode: RoutingAuto, FastModel: autoFastModel, CapableModel: autoCapableModel}
	case "hybrid":
		return RoutingStrategy{Mode: RoutingHybrid, LocalModel: hybridLocalModel, CloudModel: hybridCloudModel}
	default:
		return RoutingStrategy{Mode: RoutingFixed, FixedModel: strings.TrimSpace(modelStr)}
	}
}

// SelectModel returns the model to use for query under this strategy, mirroring
// the Rust reference's select_model. A Fixed strategy returns FixedModel (which
// may be empty, meaning "defer to the provider's configured model").
func (s RoutingStrategy) SelectModel(query string) string {
	switch s.Mode {
	case RoutingAuto:
		if EstimateComplexity(query) == ComplexitySimple {
			return s.FastModel
		}
		return s.CapableModel
	case RoutingHybrid:
		if EstimateComplexity(query) == ComplexityComplex {
			return s.CloudModel
		}
		return s.LocalModel
	default:
		return s.FixedModel
	}
}

// Describe renders a short human-readable summary of the active strategy for
// command output.
func (s RoutingStrategy) Describe() string {
	switch s.Mode {
	case RoutingAuto:
		return fmt.Sprintf("auto (simple -> %s, complex -> %s)", s.FastModel, s.CapableModel)
	case RoutingHybrid:
		return fmt.Sprintf("hybrid (local -> %s, cloud -> %s)", s.LocalModel, s.CloudModel)
	default:
		if strings.TrimSpace(s.FixedModel) == "" {
			return "fixed (provider default)"
		}
		return "fixed (" + s.FixedModel + ")"
	}
}

// ollamaTagsResponse / ollamaTag decode the GET /api/tags payload (the local
// model catalog). Only the model name is consumed.
type ollamaTagsResponse struct {
	Models []ollamaTag `json:"models"`
}

type ollamaTag struct {
	Name string `json:"name"`
}

// ListLocalOllamaModels queries GET {baseURL}/api/tags and returns the sorted,
// de-duplicated local model tags, mirroring the Rust reference's
// list_ollama_models. baseURL is normalized (a trailing /v1 or slash is
// stripped) so both the native and OpenAI-compatible forms work. An empty
// baseURL falls back to the OLLAMA_BASE_URL env / localhost default. Errors
// (unreachable, non-200, malformed) are returned so callers can degrade
// gracefully.
func ListLocalOllamaModels(baseURL string) ([]string, error) {
	resolved := resolveOllamaBaseURL(baseURL)
	client := &http.Client{Timeout: ollamaTagsTimeout}
	resp, err := client.Get(resolved + "/api/tags")
	if err != nil {
		return nil, fmt.Errorf("ollama: list models: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("ollama: list models: unexpected status %d", resp.StatusCode)
	}
	var decoded ollamaTagsResponse
	if err := json.NewDecoder(resp.Body).Decode(&decoded); err != nil {
		return nil, fmt.Errorf("ollama: list models: decode: %w", err)
	}
	seen := make(map[string]struct{}, len(decoded.Models))
	models := make([]string, 0, len(decoded.Models))
	for _, m := range decoded.Models {
		name := strings.TrimSpace(m.Name)
		if name == "" {
			continue
		}
		if _, ok := seen[name]; ok {
			continue
		}
		seen[name] = struct{}{}
		models = append(models, name)
	}
	sort.Strings(models)
	return models, nil
}
