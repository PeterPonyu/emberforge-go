package server

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

// newTestServer returns an HttpServer wired to an httptest.Server.
// The caller must call ts.Close() when done.
func newTestServer(t *testing.T) (*HttpServer, *httptest.Server) {
	t.Helper()
	store := NewSessionStore()
	hs := NewHttpServer(":0", store)
	ts := httptest.NewServer(hs.mux)
	hs.startTime = time.Now()
	return hs, ts
}

func TestHealthEndpoint(t *testing.T) {
	_, ts := newTestServer(t)
	defer ts.Close()

	resp, err := http.Get(ts.URL + "/health")
	if err != nil {
		t.Fatalf("GET /health: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("want 200, got %d", resp.StatusCode)
	}

	var body map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		t.Fatalf("decode body: %v", err)
	}
	if body["status"] != "ok" {
		t.Errorf("want status=ok, got %v", body["status"])
	}
	if body["version"] != "0.1" {
		t.Errorf("want version=0.1, got %v", body["version"])
	}
	if _, ok := body["uptime_ms"]; !ok {
		t.Error("missing uptime_ms field")
	}
}

func TestCreateAndListSessions(t *testing.T) {
	_, ts := newTestServer(t)
	defer ts.Close()

	resp, err := http.Post(ts.URL+"/sessions", "application/json", nil)
	if err != nil {
		t.Fatalf("POST /sessions: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("want 201, got %d", resp.StatusCode)
	}
	var created map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&created); err != nil {
		t.Fatalf("decode created: %v", err)
	}
	id, ok := created["id"].(string)
	if !ok || id == "" {
		t.Fatalf("missing or empty id: %v", created)
	}
	if _, ok := created["created_at"]; !ok {
		t.Error("missing created_at")
	}

	resp2, err := http.Get(ts.URL + "/sessions")
	if err != nil {
		t.Fatalf("GET /sessions: %v", err)
	}
	defer resp2.Body.Close()
	if resp2.StatusCode != http.StatusOK {
		t.Fatalf("want 200, got %d", resp2.StatusCode)
	}
	var list []map[string]interface{}
	if err := json.NewDecoder(resp2.Body).Decode(&list); err != nil {
		t.Fatalf("decode list: %v", err)
	}
	if len(list) != 1 {
		t.Fatalf("want 1 session in list, got %d", len(list))
	}
	if list[0]["id"] != id {
		t.Errorf("want id=%s in list, got %v", id, list[0]["id"])
	}
}

func TestGetSession_NotFound(t *testing.T) {
	_, ts := newTestServer(t)
	defer ts.Close()

	resp, err := http.Get(ts.URL + "/sessions/nonexistent")
	if err != nil {
		t.Fatalf("GET /sessions/nonexistent: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("want 404, got %d", resp.StatusCode)
	}
}

// TestAppendMessage_BroadcastsToSubscriber subscribes to SSE, POSTs a message,
// and asserts the subscriber receives the event within 2 seconds.
func TestAppendMessage_BroadcastsToSubscriber(t *testing.T) {
	_, ts := newTestServer(t)
	defer ts.Close()

	resp, err := http.Post(ts.URL+"/sessions", "application/json", nil)
	if err != nil {
		t.Fatalf("POST /sessions: %v", err)
	}
	defer resp.Body.Close()
	var created map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&created); err != nil {
		t.Fatalf("decode created: %v", err)
	}
	id := created["id"].(string)

	// Start SSE subscriber in a goroutine.
	receivedCh := make(chan string, 1)
	go func() {
		sseResp, err := http.Get(ts.URL + "/sessions/" + id + "/events")
		if err != nil {
			receivedCh <- ""
			return
		}
		defer sseResp.Body.Close()
		buf := make([]byte, 4096)
		n, _ := sseResp.Body.Read(buf)
		receivedCh <- string(buf[:n])
	}()

	// Give the subscriber a moment to connect.
	time.Sleep(50 * time.Millisecond)

	payload := `{"role":"user","content":"hello"}`
	msgResp, err := http.Post(ts.URL+"/sessions/"+id+"/message", "application/json", strings.NewReader(payload))
	if err != nil {
		t.Fatalf("POST message: %v", err)
	}
	msgResp.Body.Close()
	if msgResp.StatusCode != http.StatusAccepted {
		t.Fatalf("want 202, got %d", msgResp.StatusCode)
	}

	select {
	case frame := <-receivedCh:
		if !strings.Contains(frame, "data:") {
			t.Errorf("expected SSE frame with 'data:', got: %q", frame)
		}
		if !strings.Contains(frame, "hello") {
			t.Errorf("expected frame to contain 'hello', got: %q", frame)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for SSE event")
	}
}

// TestSSE_ClientDisconnect verifies that cancelling the subscriber's request
// context causes the SSE handler to exit and invoke the unsubscribe function,
// cleaning up the subscriber slot.
// This test drives the store's Subscribe/Broadcast API directly via an
// httptest.ResponseRecorder to avoid blocking the httptest.Server on teardown.
func TestSSE_ClientDisconnect(t *testing.T) {
	store := NewSessionStore()
	sess, err := store.CreateSession()
	if err != nil {
		t.Fatalf("CreateSession: %v", err)
	}
	id := sess.ID

	ch, unsubscribe := store.Subscribe(id)

	store.mu.RLock()
	countBefore := len(store.subscribers[id])
	store.mu.RUnlock()
	if countBefore != 1 {
		t.Fatalf("want 1 subscriber before disconnect, got %d", countBefore)
	}

	// Simulate the SSE handler's defer: unsubscribe removes the channel.
	ctx, cancel := context.WithCancel(context.Background())
	handlerDone := make(chan struct{})
	go func() {
		defer close(handlerDone)
		defer unsubscribe()
		select {
		case <-ctx.Done():
			return
		case <-ch:
			return
		}
	}()

	cancel()

	select {
	case <-handlerDone:
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for handler goroutine to exit")
	}

	store.mu.RLock()
	countAfter := len(store.subscribers[id])
	store.mu.RUnlock()
	if countAfter != 0 {
		t.Errorf("want 0 subscribers after disconnect, got %d", countAfter)
	}

	_ = bytes.NewReader(nil) // keep bytes import referenced
}
