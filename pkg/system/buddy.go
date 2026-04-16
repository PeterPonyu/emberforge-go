package system

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

type buddyTemplate struct {
	Name        string
	Species     string
	Personality string
}

type StarterBuddyCompanion struct {
	Name        string
	Species     string
	Personality string
	Muted       bool
}

type StarterBuddyState struct {
	path      string
	nextIndex int
	companion *buddyTemplate
	muted     bool
}

type buddySnapshot struct {
	NextIndex int            `json:"next_index"`
	Companion *buddyTemplate `json:"companion,omitempty"`
	Muted     bool           `json:"muted"`
}

var buddyTemplates = []buddyTemplate{
	{Name: "Waddles", Species: "duck", Personality: "Quirky and easily amused. Leaves rubber duck debugging tips everywhere."},
	{Name: "Goosberry", Species: "goose", Personality: "Assertive and honks at bad code. Takes no prisoners in code reviews."},
	{Name: "Gooey", Species: "blob", Personality: "Adaptable and goes with the flow. Sometimes splits into two when confused."},
	{Name: "Whiskers", Species: "cat", Personality: "Independent and judgmental. Watches you type with mild disdain."},
	{Name: "Ember", Species: "dragon", Personality: "Fiery and passionate about architecture. Hoards good variable names."},
	{Name: "Inky", Species: "octopus", Personality: "Multitasker extraordinaire. Wraps tentacles around every problem at once."},
	{Name: "Hoots", Species: "owl", Personality: "Wise but verbose. Always says \"let me think about that\" for exactly 3 seconds."},
	{Name: "Waddleford", Species: "penguin", Personality: "Cool under pressure. Slides gracefully through merge conflicts."},
	{Name: "Shelly", Species: "turtle", Personality: "Patient and thorough. Believes slow and steady wins the deploy."},
	{Name: "Trailblazer", Species: "snail", Personality: "Methodical and leaves a trail of useful comments. Never rushes."},
	{Name: "Casper", Species: "ghost", Personality: "Ethereal and appears at the worst possible moments with spooky insights."},
	{Name: "Axie", Species: "axolotl", Personality: "Regenerative and cheerful. Recovers from any bug with a smile."},
	{Name: "Chill", Species: "capybara", Personality: "Zen master. Remains calm while everything around is on fire."},
	{Name: "Spike", Species: "cactus", Personality: "Prickly on the outside but full of good intentions. Thrives on neglect."},
	{Name: "Byte", Species: "robot", Personality: "Efficient and literal. Processes feedback in binary."},
	{Name: "Flops", Species: "rabbit", Personality: "Energetic and hops between tasks. Finishes before you start."},
	{Name: "Spore", Species: "mushroom", Personality: "Quietly insightful. Grows on you over time."},
	{Name: "Chonk", Species: "chonk", Personality: "Big, warm, and takes up the whole couch. Prioritizes comfort over elegance."},
}

func resolveBuddyStatePath(explicitPath string) string {
	if strings.TrimSpace(explicitPath) != "" {
		return explicitPath
	}
	if envPath := strings.TrimSpace(os.Getenv("EMBER_BUDDY_STATE_PATH")); envPath != "" {
		return envPath
	}
	if configHome := strings.TrimSpace(os.Getenv("EMBER_CONFIG_HOME")); configHome != "" {
		return filepath.Join(configHome, "buddy-state.json")
	}
	if home, err := os.UserHomeDir(); err == nil && strings.TrimSpace(home) != "" {
		return filepath.Join(home, ".emberforge", "buddy-state.json")
	}
	return filepath.Join(".emberforge", "buddy-state.json")
}

func NewStarterBuddyState(path string) *StarterBuddyState {
	state := &StarterBuddyState{path: resolveBuddyStatePath(path)}
	state.load()
	return state
}

func (s *StarterBuddyState) load() {
	raw, err := os.ReadFile(s.path)
	if err != nil {
		return
	}
	var snapshot buddySnapshot
	if err := json.Unmarshal(raw, &snapshot); err != nil {
		return
	}
	s.nextIndex = snapshot.NextIndex
	s.companion = snapshot.Companion
	s.muted = snapshot.Muted
}

func (s *StarterBuddyState) persist() {
	if err := os.MkdirAll(filepath.Dir(s.path), 0o755); err != nil {
		return
	}
	raw, err := json.MarshalIndent(buddySnapshot{
		NextIndex: s.nextIndex,
		Companion: s.companion,
		Muted:     s.muted,
	}, "", "  ")
	if err != nil {
		return
	}
	_ = os.WriteFile(s.path, append(raw, '\n'), 0o644)
}

func (s *StarterBuddyState) current() (*StarterBuddyCompanion, bool) {
	if s.companion == nil {
		return nil, false
	}
	return &StarterBuddyCompanion{
		Name:        s.companion.Name,
		Species:     s.companion.Species,
		Personality: s.companion.Personality,
		Muted:       s.muted,
	}, true
}

func (s *StarterBuddyState) hatch() (StarterBuddyCompanion, bool) {
	if current, ok := s.current(); ok {
		return *current, false
	}
	template := buddyTemplates[s.nextIndex%len(buddyTemplates)]
	s.nextIndex++
	s.companion = &template
	s.muted = false
	s.persist()
	current, _ := s.current()
	return *current, true
}

func (s *StarterBuddyState) rehatch() StarterBuddyCompanion {
	template := buddyTemplates[s.nextIndex%len(buddyTemplates)]
	s.nextIndex++
	s.companion = &template
	s.muted = false
	s.persist()
	current, _ := s.current()
	return *current
}

func (s *StarterBuddyState) mute() (*StarterBuddyCompanion, bool) {
	if s.companion == nil {
		return nil, false
	}
	s.muted = true
	s.persist()
	return s.current()
}

func (s *StarterBuddyState) unmute() (*StarterBuddyCompanion, bool) {
	if s.companion == nil {
		return nil, false
	}
	s.muted = false
	s.persist()
	return s.current()
}

func renderBuddyCompanion(prefix string, companion StarterBuddyCompanion, note string) string {
	status := "active"
	if companion.Muted {
		status = "muted"
	}
	lines := []string{
		prefix,
		fmt.Sprintf("name: %s", companion.Name),
		fmt.Sprintf("species: %s", strings.ToUpper(companion.Species[:1])+companion.Species[1:]),
		fmt.Sprintf("personality: %s", companion.Personality),
		fmt.Sprintf("status: %s", status),
	}
	if strings.TrimSpace(note) != "" {
		lines = append(lines, note)
	}
	return strings.Join(lines, "\n")
}

func ExecuteBuddyCommand(state *StarterBuddyState, payload string) string {
	trimmed := strings.TrimSpace(payload)
	action := ""
	if trimmed != "" {
		action = strings.Fields(trimmed)[0]
	}

	switch action {
	case "":
		if companion, ok := state.current(); ok {
			return renderBuddyCompanion(
				"[command] buddy",
				*companion,
				"commands: /buddy pet /buddy mute /buddy unmute /buddy hatch /buddy rehatch",
			)
		}
		return strings.Join([]string{
			"[command] buddy",
			"status: no companion",
			"tip: use /buddy hatch to get one",
		}, "\n")
	case "hatch":
		companion, created := state.hatch()
		if !created {
			return renderBuddyCompanion(
				"[command] buddy hatch",
				companion,
				"tip: use /buddy rehatch to roll a new companion",
			)
		}
		return renderBuddyCompanion(
			"[command] buddy hatch",
			companion,
			"note: starter buddy translation from claude-code-src",
		)
	case "rehatch":
		return renderBuddyCompanion(
			"[command] buddy rehatch",
			state.rehatch(),
			"note: previous companion replaced",
		)
	case "pet":
		if companion, ok := state.current(); ok {
			status := "active"
			if companion.Muted {
				status = "muted"
			}
			return strings.Join([]string{
				"[command] buddy pet",
				fmt.Sprintf("reaction: %s purrs happily!", companion.Name),
				fmt.Sprintf("status: %s", status),
			}, "\n")
		}
		return strings.Join([]string{
			"[command] buddy pet",
			"status: no companion",
			"tip: use /buddy hatch to get one",
		}, "\n")
	case "mute":
		if companion, ok := state.mute(); ok {
			return renderBuddyCompanion("[command] buddy mute", *companion, "")
		}
		return strings.Join([]string{
			"[command] buddy mute",
			"status: no companion",
			"tip: use /buddy hatch to get one",
		}, "\n")
	case "unmute":
		if companion, ok := state.unmute(); ok {
			return renderBuddyCompanion("[command] buddy unmute", *companion, "")
		}
		return strings.Join([]string{
			"[command] buddy unmute",
			"status: no companion",
			"tip: use /buddy hatch to get one",
		}, "\n")
	default:
		return strings.Join([]string{
			"[command] buddy",
			fmt.Sprintf("unsupported action: %s", action),
			"commands: /buddy pet /buddy mute /buddy unmute /buddy hatch /buddy rehatch",
		}, "\n")
	}
}
