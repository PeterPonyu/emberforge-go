package server

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"sync"
	"time"
)

type Session struct {
	ID        string    `json:"id"`
	CreatedAt time.Time `json:"created_at"`
	Messages  []Message `json:"messages"`
}

type Message struct {
	Role      string    `json:"role"`
	Content   string    `json:"content"`
	Timestamp time.Time `json:"timestamp"`
}

// SessionSummary is the lightweight view returned by GET /sessions.
type SessionSummary struct {
	ID           string    `json:"id"`
	MessageCount int       `json:"message_count"`
	CreatedAt    time.Time `json:"created_at"`
}

type SessionStore struct {
	mu          sync.RWMutex
	sessions    map[string]*Session
	subscribers map[string][]chan []byte
}

func NewSessionStore() *SessionStore {
	return &SessionStore{
		sessions:    make(map[string]*Session),
		subscribers: make(map[string][]chan []byte),
	}
}

// newSessionID returns a 32-char hex string from 16 random bytes.
func newSessionID() (string, error) {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}

func (ss *SessionStore) CreateSession() (*Session, error) {
	id, err := newSessionID()
	if err != nil {
		return nil, fmt.Errorf("session_store: generate id: %w", err)
	}
	s := &Session{
		ID:        id,
		CreatedAt: time.Now().UTC(),
		Messages:  []Message{},
	}
	ss.mu.Lock()
	ss.sessions[id] = s
	ss.mu.Unlock()
	return s, nil
}

func (ss *SessionStore) GetSession(id string) (*Session, bool) {
	ss.mu.RLock()
	defer ss.mu.RUnlock()
	s, ok := ss.sessions[id]
	return s, ok
}

func (ss *SessionStore) ListSessions() []SessionSummary {
	ss.mu.RLock()
	defer ss.mu.RUnlock()
	out := make([]SessionSummary, 0, len(ss.sessions))
	for _, s := range ss.sessions {
		out = append(out, SessionSummary{
			ID:           s.ID,
			MessageCount: len(s.Messages),
			CreatedAt:    s.CreatedAt,
		})
	}
	return out
}

func (ss *SessionStore) AppendMessage(id, role, content string) error {
	ss.mu.Lock()
	defer ss.mu.Unlock()
	s, ok := ss.sessions[id]
	if !ok {
		return fmt.Errorf("session_store: session %q not found", id)
	}
	s.Messages = append(s.Messages, Message{
		Role:      role,
		Content:   content,
		Timestamp: time.Now().UTC(),
	})
	return nil
}

// Subscribe registers a new channel for SSE events on session id.
// The caller must invoke the returned unsubscribe function when done.
func (ss *SessionStore) Subscribe(id string) (<-chan []byte, func()) {
	ch := make(chan []byte, 64)
	ss.mu.Lock()
	ss.subscribers[id] = append(ss.subscribers[id], ch)
	ss.mu.Unlock()

	unsubscribe := func() {
		ss.mu.Lock()
		defer ss.mu.Unlock()
		subs := ss.subscribers[id]
		for i, c := range subs {
			if c == ch {
				ss.subscribers[id] = append(subs[:i], subs[i+1:]...)
				break
			}
		}
	}
	return ch, unsubscribe
}

// Broadcast sends event bytes to all subscribers of session id.
// Non-blocking: slow subscribers are skipped.
func (ss *SessionStore) Broadcast(id string, event []byte) {
	ss.mu.RLock()
	subs := ss.subscribers[id]
	// copy slice to avoid holding the lock during sends
	targets := make([]chan []byte, len(subs))
	copy(targets, subs)
	ss.mu.RUnlock()

	for _, ch := range targets {
		select {
		case ch <- event:
		default:
		}
	}
}

func (ss *SessionStore) broadcastMessage(id string, msg Message) {
	b, err := json.Marshal(msg)
	if err != nil {
		return
	}
	ss.Broadcast(id, b)
}
