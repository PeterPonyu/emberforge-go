package server

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"
)

type HttpServer struct {
	addr      string
	store     *SessionStore
	mux       *http.ServeMux
	startTime time.Time
}

func NewHttpServer(addr string, store *SessionStore) *HttpServer {
	s := &HttpServer{
		addr:  addr,
		store: store,
		mux:   http.NewServeMux(),
	}
	s.setupRoutes()
	return s
}

func (s *HttpServer) setupRoutes() {
	s.mux.HandleFunc("/health", s.handleHealth)
	s.mux.HandleFunc("/sessions", s.handleSessions)
	// catch-all for /sessions/{id} and /sessions/{id}/message and /sessions/{id}/events
	s.mux.HandleFunc("/sessions/", s.handleSessionsSubpath)
}

// Start runs the HTTP server and blocks until ctx is cancelled, then shuts down gracefully.
func (s *HttpServer) Start(ctx context.Context) error {
	s.startTime = time.Now()
	srv := &http.Server{
		Addr:    s.addr,
		Handler: s.mux,
	}
	go func() {
		<-ctx.Done()
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = srv.Shutdown(shutdownCtx)
	}()
	if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		return err
	}
	return nil
}

func writeJSON(w http.ResponseWriter, status int, v interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func (s *HttpServer) handleHealth(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	uptimeMs := time.Since(s.startTime).Milliseconds()
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"status":    "ok",
		"version":   "0.1",
		"uptime_ms": uptimeMs,
	})
}

func (s *HttpServer) handleSessions(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		summaries := s.store.ListSessions()
		if summaries == nil {
			summaries = []SessionSummary{}
		}
		writeJSON(w, http.StatusOK, summaries)
	case http.MethodPost:
		sess, err := s.store.CreateSession()
		if err != nil {
			http.Error(w, "failed to create session", http.StatusInternalServerError)
			return
		}
		writeJSON(w, http.StatusCreated, map[string]interface{}{
			"id":         sess.ID,
			"created_at": sess.CreatedAt.Format(time.RFC3339),
		})
	default:
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}

// handleSessionsSubpath dispatches /sessions/{id}, /sessions/{id}/message,
// and /sessions/{id}/events.
func (s *HttpServer) handleSessionsSubpath(w http.ResponseWriter, r *http.Request) {
	tail := strings.TrimPrefix(r.URL.Path, "/sessions/")

	if strings.HasSuffix(tail, "/events") {
		id := strings.TrimSuffix(tail, "/events")
		s.handleSSE(w, r, id)
		return
	}
	if strings.HasSuffix(tail, "/message") {
		id := strings.TrimSuffix(tail, "/message")
		s.handleAppendMessage(w, r, id)
		return
	}
	// Plain /sessions/{id}
	s.handleGetSession(w, r, tail)
}

func (s *HttpServer) handleGetSession(w http.ResponseWriter, r *http.Request, id string) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	sess, ok := s.store.GetSession(id)
	if !ok {
		http.NotFound(w, r)
		return
	}
	writeJSON(w, http.StatusOK, sess)
}

func (s *HttpServer) handleAppendMessage(w http.ResponseWriter, r *http.Request, id string) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	var body struct {
		Role    string `json:"role"`
		Content string `json:"content"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		http.Error(w, "invalid JSON body", http.StatusBadRequest)
		return
	}
	if err := s.store.AppendMessage(id, body.Role, body.Content); err != nil {
		http.NotFound(w, r)
		return
	}
	sess, _ := s.store.GetSession(id)
	var msg Message
	if sess != nil && len(sess.Messages) > 0 {
		msg = sess.Messages[len(sess.Messages)-1]
	}
	s.store.broadcastMessage(id, msg)
	w.WriteHeader(http.StatusAccepted)
}

func (s *HttpServer) handleSSE(w http.ResponseWriter, r *http.Request, id string) {
	_, ok := s.store.GetSession(id)
	if !ok {
		http.NotFound(w, r)
		return
	}
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "streaming unsupported", http.StatusInternalServerError)
		return
	}
	ch, unsubscribe := s.store.Subscribe(id)
	defer unsubscribe()
	for {
		select {
		case <-r.Context().Done():
			return
		case ev := <-ch:
			fmt.Fprintf(w, "data: %s\n\n", ev)
			flusher.Flush()
		}
	}
}
