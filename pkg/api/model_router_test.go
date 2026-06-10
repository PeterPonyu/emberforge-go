package api

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestEstimateComplexity(t *testing.T) {
	cases := []struct {
		query string
		want  TaskComplexity
	}{
		{"hello", ComplexitySimple},
		{"hi", ComplexitySimple},
		{"what time is it", ComplexitySimple},
		{"what files are in the src directory", ComplexityMedium},
		{"refactor the authentication module to use JWT", ComplexityComplex},
		{"implement a REST API with pagination", ComplexityComplex},
		{"first read the config, then update the database, finally restart", ComplexityComplex},
	}
	for _, tc := range cases {
		if got := EstimateComplexity(tc.query); got != tc.want {
			t.Errorf("EstimateComplexity(%q) = %v, want %v", tc.query, got, tc.want)
		}
	}
}

func TestParseRoutingStrategy(t *testing.T) {
	if s := ParseRoutingStrategy("auto"); s.Mode != RoutingAuto || s.FastModel != autoFastModel || s.CapableModel != autoCapableModel {
		t.Fatalf("auto strategy mismatch: %+v", s)
	}
	if s := ParseRoutingStrategy("hybrid"); s.Mode != RoutingHybrid || s.LocalModel != hybridLocalModel || s.CloudModel != hybridCloudModel {
		t.Fatalf("hybrid strategy mismatch: %+v", s)
	}
	if s := ParseRoutingStrategy("qwen3:8b"); s.Mode != RoutingFixed || s.FixedModel != "qwen3:8b" {
		t.Fatalf("fixed strategy mismatch: %+v", s)
	}
}

// TestSelectModelAutoRoutesByComplexity is the core Item C unit guarantee:
// `/model auto` picks the fast model for a trivial prompt and the capable model
// for a coding prompt.
func TestSelectModelAutoRoutesByComplexity(t *testing.T) {
	s := ParseRoutingStrategy("auto")
	if got := s.SelectModel("hi"); got != autoFastModel {
		t.Fatalf("trivial prompt routed to %q, want fast model %q", got, autoFastModel)
	}
	if got := s.SelectModel("refactor the auth module to support OAuth"); got != autoCapableModel {
		t.Fatalf("coding prompt routed to %q, want capable model %q", got, autoCapableModel)
	}
	// Trivial and complex prompts must resolve to DIFFERENT models.
	if s.SelectModel("hi") == s.SelectModel("implement a parser") {
		t.Fatalf("auto routing did not differentiate trivial vs complex prompts")
	}
}

func TestSelectModelHybridRoutesComplexToCloud(t *testing.T) {
	s := ParseRoutingStrategy("hybrid")
	if got := s.SelectModel("hello"); got != hybridLocalModel {
		t.Fatalf("simple prompt routed to %q, want local %q", got, hybridLocalModel)
	}
	if got := s.SelectModel("design and implement a distributed cache"); got != hybridCloudModel {
		t.Fatalf("complex prompt routed to %q, want cloud %q", got, hybridCloudModel)
	}
}

func TestSelectModelFixedAndZeroValue(t *testing.T) {
	if got := ParseRoutingStrategy("llama3:8b").SelectModel("anything"); got != "llama3:8b" {
		t.Fatalf("fixed model = %q, want llama3:8b", got)
	}
	// Zero value (default) returns empty -> defer to provider's configured model.
	var zero RoutingStrategy
	if got := zero.SelectModel("hello"); got != "" {
		t.Fatalf("zero-value strategy returned %q, want empty", got)
	}
}

// TestListLocalOllamaModelsParsesTags verifies the /api/tags catalog is parsed,
// sorted, and de-duplicated.
func TestListLocalOllamaModelsParsesTags(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/tags" {
			t.Fatalf("unexpected path %s", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprintln(w, `{"models":[{"name":"qwen3:8b"},{"name":"qwen2.5:1.5b"},{"name":"qwen3:8b"},{"name":"llama3:8b"}]}`)
	}))
	defer srv.Close()

	models, err := ListLocalOllamaModels(srv.URL)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	want := []string{"llama3:8b", "qwen2.5:1.5b", "qwen3:8b"}
	if len(models) != len(want) {
		t.Fatalf("got %v, want %v", models, want)
	}
	for i := range want {
		if models[i] != want[i] {
			t.Fatalf("models[%d] = %q, want %q (full: %v)", i, models[i], want[i], models)
		}
	}
}

func TestListLocalOllamaModelsHandlesV1Suffix(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/tags" {
			t.Fatalf("unexpected path %s (v1 suffix not normalized)", r.URL.Path)
		}
		fmt.Fprintln(w, `{"models":[{"name":"qwen3:8b"}]}`)
	}))
	defer srv.Close()

	models, err := ListLocalOllamaModels(srv.URL + "/v1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(models) != 1 || models[0] != "qwen3:8b" {
		t.Fatalf("got %v, want [qwen3:8b]", models)
	}
}

func TestListLocalOllamaModelsErrorsOnNon200(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "boom", http.StatusInternalServerError)
	}))
	defer srv.Close()
	if _, err := ListLocalOllamaModels(srv.URL); err == nil {
		t.Fatal("expected error on non-200, got nil")
	}
}
