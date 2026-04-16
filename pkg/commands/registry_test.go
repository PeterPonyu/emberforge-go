package commands

import "testing"

func TestGetCommandsExposesTranslatedCommandSurface(t *testing.T) {
	commands := GetCommands()
	got := make([]string, 0, len(commands))
	for _, command := range commands {
		got = append(got, command.Name)
	}

	want := []string{"help", "status", "doctor", "model", "questions", "tasks", "buddy", "compact", "review", "commit", "pr"}
	if len(got) != len(want) {
		t.Fatalf("expected %d commands, got %d (%v)", len(want), len(got), got)
	}
	for index, expected := range want {
		if got[index] != expected {
			t.Fatalf("expected command %d to be %q, got %q", index, expected, got[index])
		}
	}
	if commands[2].ArgumentHint != "[quick|status]" {
		t.Fatalf("expected doctor hint to be recorded, got %q", commands[2].ArgumentHint)
	}
	if commands[4].Category != CommandCategorySession {
		t.Fatalf("expected questions category session, got %q", commands[4].Category)
	}
	if commands[5].Category != CommandCategoryAutomation {
		t.Fatalf("expected tasks category automation, got %q", commands[5].Category)
	}
	if commands[6].ArgumentHint != "[hatch|rehatch|pet|mute|unmute]" {
		t.Fatalf("expected buddy hint to be recorded, got %q", commands[6].ArgumentHint)
	}
	if commands[8].Category != CommandCategoryGit {
		t.Fatalf("expected review category git, got %q", commands[8].Category)
	}
}
