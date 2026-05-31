package server

import (
	"path/filepath"
	"testing"
)

func TestParityFixturePathDefaultIsPortable(t *testing.T) {
	t.Setenv("EMBERFORGE_PARITY_FIXTURE", "")

	path := parityFixturePath()
	if filepath.IsAbs(path) {
		t.Fatalf("default parity fixture path must be repository-relative, got %q", path)
	}
}

func TestParityFixturePathCanBeOverridden(t *testing.T) {
	t.Setenv("EMBERFORGE_PARITY_FIXTURE", "/tmp/emberforge-parity.jsonl")

	if got := parityFixturePath(); got != "/tmp/emberforge-parity.jsonl" {
		t.Fatalf("got %q", got)
	}
}
