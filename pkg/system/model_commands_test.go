package system

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/PeterPonyu/emberforge-go/pkg/api"
)

// TestModelListParsesLiveCatalog verifies `/model list` queries Ollama's
// /api/tags endpoint and renders the real local model names plus cloud shortcuts.
func TestModelListParsesLiveCatalog(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/tags" {
			t.Fatalf("unexpected path %s", r.URL.Path)
		}
		fmt.Fprintln(w, `{"models":[{"name":"qwen3:8b"},{"name":"qwen2.5:1.5b"}]}`)
	}))
	defer srv.Close()
	t.Setenv("OLLAMA_BASE_URL", srv.URL)

	app := NewStarterSystemApplication(DefaultStarterSystemConfig())
	defer app.Shutdown()

	out, ok := ExecuteStarterSlashCommand(app, "/model list")
	if !ok {
		t.Fatal("expected /model list to be handled")
	}
	for _, want := range []string{"[command] model list:", "qwen3:8b", "qwen2.5:1.5b", "Cloud shortcuts", "opus", "Routing shortcuts"} {
		if !strings.Contains(out, want) {
			t.Fatalf("expected /model list output to contain %q, got:\n%s", want, out)
		}
	}
}

// TestModelListGracefulWhenOllamaDown verifies `/model list` degrades gracefully
// (status note + cloud shortcuts) when the catalog endpoint is unreachable.
func TestModelListGracefulWhenOllamaDown(t *testing.T) {
	// Point at a closed server so the request fails fast.
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	srv.Close()
	t.Setenv("OLLAMA_BASE_URL", srv.URL)

	app := NewStarterSystemApplication(DefaultStarterSystemConfig())
	defer app.Shutdown()

	out, ok := ExecuteStarterSlashCommand(app, "/model list")
	if !ok {
		t.Fatal("expected /model list to be handled")
	}
	if !strings.Contains(out, "unreachable") {
		t.Fatalf("expected unreachable note, got:\n%s", out)
	}
	if !strings.Contains(out, "Cloud shortcuts") {
		t.Fatalf("expected cloud shortcuts even when offline, got:\n%s", out)
	}
}

// TestModelAutoInstallsRoutingStrategy verifies `/model auto` installs the Auto
// routing strategy on the runtime so later turns route by complexity.
func TestModelAutoInstallsRoutingStrategy(t *testing.T) {
	app := NewStarterSystemApplication(DefaultStarterSystemConfig())
	defer app.Shutdown()

	out, ok := ExecuteStarterSlashCommand(app, "/model auto")
	if !ok || !strings.Contains(out, "auto") {
		t.Fatalf("unexpected /model auto output: %q", out)
	}
	if app.RoutingStrategy().Mode != api.RoutingAuto {
		t.Fatalf("expected Auto routing strategy, got %v", app.RoutingStrategy().Mode)
	}
	// The installed strategy routes trivial vs complex prompts to different models.
	if app.RoutingStrategy().SelectModel("hi") == app.RoutingStrategy().SelectModel("implement a parser") {
		t.Fatalf("auto strategy did not differentiate prompts")
	}
}

func TestModelHybridInstallsRoutingStrategy(t *testing.T) {
	app := NewStarterSystemApplication(DefaultStarterSystemConfig())
	defer app.Shutdown()

	if _, ok := ExecuteStarterSlashCommand(app, "/model hybrid"); !ok {
		t.Fatal("expected /model hybrid to be handled")
	}
	if app.RoutingStrategy().Mode != api.RoutingHybrid {
		t.Fatalf("expected Hybrid routing strategy, got %v", app.RoutingStrategy().Mode)
	}
}

// TestModelSwitchUpdatesActiveModel verifies `/model <name>` switches the active
// model for subsequent turns and resets routing to fixed.
func TestModelSwitchUpdatesActiveModel(t *testing.T) {
	// Register cleanup of the env the switch mutates.
	t.Setenv("OLLAMA_MODEL", "")
	t.Setenv("EMBER_MODEL", "")

	app := NewStarterSystemApplication(DefaultStarterSystemConfig())
	defer app.Shutdown()

	out, ok := ExecuteStarterSlashCommand(app, "/model llama3:8b")
	if !ok {
		t.Fatal("expected /model <name> to be handled")
	}
	if !strings.Contains(out, "switched") || !strings.Contains(out, "llama3:8b") {
		t.Fatalf("unexpected switch output: %q", out)
	}
	if app.ActiveModel() != "llama3:8b" {
		t.Fatalf("active model = %q, want llama3:8b", app.ActiveModel())
	}
	if app.RoutingStrategy().Mode != api.RoutingFixed || app.RoutingStrategy().FixedModel != "llama3:8b" {
		t.Fatalf("expected fixed strategy on llama3:8b, got %+v", app.RoutingStrategy())
	}
}

// TestModelAliasSwitchResolvesCanonical verifies an alias (e.g. "sonnet") is
// resolved to its canonical id when switching.
func TestModelAliasSwitchResolvesCanonical(t *testing.T) {
	t.Setenv("OLLAMA_MODEL", "")
	t.Setenv("EMBER_MODEL", "")
	t.Setenv("ANTHROPIC_API_KEY", "test-key") // so the anthropic provider resolves

	app := NewStarterSystemApplication(DefaultStarterSystemConfig())
	defer app.Shutdown()

	out, ok := ExecuteStarterSlashCommand(app, "/model sonnet")
	if !ok {
		t.Fatal("expected /model sonnet to be handled")
	}
	if !strings.Contains(out, "claude-sonnet-4-6") {
		t.Fatalf("expected alias resolution to claude-sonnet-4-6, got: %q", out)
	}
}
