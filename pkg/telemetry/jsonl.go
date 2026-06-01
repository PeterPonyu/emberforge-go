package telemetry

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

// jsonlRecord is the on-disk shape of a single telemetry line. It mirrors the
// Rust JsonlTelemetrySink record (session id, sequence, timestamp, event type,
// and attributes).
type jsonlRecord struct {
	SessionID   string            `json:"session_id"`
	Sequence    uint64            `json:"seq"`
	TimestampMS int64             `json:"timestamp_ms"`
	Type        string            `json:"type"`
	Attributes  map[string]string `json:"attributes,omitempty"`
}

// JsonlTelemetrySink appends telemetry events as newline-delimited JSON to a
// log file. It is safe for concurrent use. Each record carries a per-sink
// monotonic sequence number, a millisecond timestamp, the event type (taken
// from Event.Name), and any structured attributes (Event.Details is recorded
// under the "details" key).
type JsonlTelemetrySink struct {
	sessionID string
	path      string
	mu        sync.Mutex
	file      *os.File
	sequence  atomic.Uint64
}

// NewJsonlTelemetrySink opens (or creates) the JSONL telemetry log at path,
// creating any missing parent directories. The file is opened in append mode
// so existing telemetry is preserved.
func NewJsonlTelemetrySink(sessionID, path string) (*JsonlTelemetrySink, error) {
	if dir := filepath.Dir(path); dir != "" {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return nil, fmt.Errorf("telemetry: create log directory %q: %w", dir, err)
		}
	}
	file, err := os.OpenFile(path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		return nil, fmt.Errorf("telemetry: open log file %q: %w", path, err)
	}
	return &JsonlTelemetrySink{
		sessionID: sessionID,
		path:      path,
		file:      file,
	}, nil
}

// Path returns the absolute log path this sink writes to.
func (s *JsonlTelemetrySink) Path() string {
	return s.path
}

// SessionID returns the session identifier stamped onto every record.
func (s *JsonlTelemetrySink) SessionID() string {
	return s.sessionID
}

// Record appends event as a JSONL line. Encoding or write failures are
// swallowed so telemetry never disrupts the primary workflow, mirroring the
// best-effort semantics of the Rust sink.
func (s *JsonlTelemetrySink) Record(event Event) {
	record := jsonlRecord{
		SessionID:   s.sessionID,
		Sequence:    s.sequence.Add(1) - 1,
		TimestampMS: time.Now().UnixMilli(),
		Type:        event.Name,
	}
	if strings.TrimSpace(event.Details) != "" {
		record.Attributes = map[string]string{"details": event.Details}
	}

	line, err := json.Marshal(record)
	if err != nil {
		return
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	_, _ = s.file.Write(append(line, '\n'))
}

// Close flushes and releases the underlying log file.
func (s *JsonlTelemetrySink) Close() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.file == nil {
		return nil
	}
	err := s.file.Close()
	s.file = nil
	if err != nil {
		return fmt.Errorf("telemetry: close log file %q: %w", s.path, err)
	}
	return nil
}

// DefaultJsonlPath resolves the telemetry log path under
// $EMBER_CONFIG_HOME/telemetry/<sessionID>.jsonl, falling back to the user home
// (~/.emberforge/telemetry) and finally a relative .emberforge directory.
func DefaultJsonlPath(sessionID string) string {
	name := sessionID + ".jsonl"
	if configHome := strings.TrimSpace(os.Getenv("EMBER_CONFIG_HOME")); configHome != "" {
		return filepath.Join(configHome, "telemetry", name)
	}
	if home, err := os.UserHomeDir(); err == nil && strings.TrimSpace(home) != "" {
		return filepath.Join(home, ".emberforge", "telemetry", name)
	}
	return filepath.Join(".emberforge", "telemetry", name)
}
