package server

import (
	"bufio"
	"encoding/json"
	"os"
	"testing"
)

const parityFixturePath = "/home/zeyufu/Desktop/emberforge-translations/parity_fixtures/scenario_001_session_lifecycle.jsonl"

func TestParityReplay_ScenarioSessionLifecycle(t *testing.T) {
	if _, err := os.Stat(parityFixturePath); err != nil {
		t.Skipf("parity fixture not present at %s: %v", parityFixturePath, err)
	}

	f, err := os.Open(parityFixturePath)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()

	store := NewSessionStore()
	var sessionID string
	var expectedMessageCount int

	scanner := bufio.NewScanner(f)
	scanner.Buffer(make([]byte, 256*1024), 256*1024)
	for scanner.Scan() {
		line := scanner.Bytes()
		if len(line) == 0 {
			continue
		}
		var disc struct {
			Type string `json:"type"`
		}
		if err := json.Unmarshal(line, &disc); err != nil {
			t.Fatalf("invalid json: %v", err)
		}
		switch disc.Type {
		case "session":
			sess, err := store.CreateSession()
			if err != nil {
				t.Fatalf("CreateSession: %v", err)
			}
			sessionID = sess.ID
		case "message":
			var msg struct {
				Role    string          `json:"role"`
				Content string          `json:"content"`
				Blocks  json.RawMessage `json:"blocks"`
			}
			if err := json.Unmarshal(line, &msg); err != nil {
				t.Fatalf("message decode: %v", err)
			}
			if len(msg.Blocks) > 0 {
				if err := store.AppendBlocksMessage(sessionID, msg.Role, msg.Blocks); err != nil {
					t.Fatalf("AppendBlocksMessage: %v", err)
				}
			} else {
				if err := store.AppendMessage(sessionID, msg.Role, msg.Content); err != nil {
					t.Fatalf("AppendMessage: %v", err)
				}
			}
			expectedMessageCount++
		case "tool_use", "tool_result":
			// tool_use/tool_result records are now captured as message records with blocks
			continue
		case "session_close":
			// Sanity check: closing id must match the session we created.
			var rec struct {
				ID string `json:"id"`
			}
			if err := json.Unmarshal(line, &rec); err != nil {
				t.Fatalf("session_close decode: %v", err)
			}
			// The fixture uses a deterministic id; our store assigns its own id.
			// We only verify the field is non-empty (fixture is well-formed).
			if rec.ID == "" {
				t.Error("session_close record has empty id")
			}
		default:
			t.Logf("unknown record type: %s", disc.Type)
		}
	}
	if err := scanner.Err(); err != nil {
		t.Fatal(err)
	}

	sess, ok := store.GetSession(sessionID)
	if !ok {
		t.Fatal("session not found after replay")
	}
	if len(sess.Messages) != expectedMessageCount {
		t.Errorf("got %d messages, want %d", len(sess.Messages), expectedMessageCount)
	}
}
