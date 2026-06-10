package main_test

import (
	"os/exec"
	"path/filepath"
	"testing"
)

// TestBuildAndSmokeHelp compiles the ember binary into a temp directory and
// invokes it with -help, verifying that the binary builds and exits cleanly.
// Go's flag package prints usage and exits with code 2 for -help — that is the
// expected outcome when no -help flag is registered explicitly.
//
// Run with -short to skip the build step in fast test runs:
//
//	go test -short ./cmd/ember/...
func TestBuildAndSmokeHelp(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping binary build smoke test in short mode")
	}

	dir := t.TempDir()
	bin := filepath.Join(dir, "ember")

	build := exec.Command("go", "build", "-o", bin, ".")
	build.Dir = "."
	if out, err := build.CombinedOutput(); err != nil {
		t.Fatalf("go build failed: %v\n%s", err, out)
	}

	cmd := exec.Command(bin, "-help")
	out, err := cmd.CombinedOutput()
	if err == nil {
		// exit 0 is also fine if the binary handles -help explicitly
		return
	}
	if exitErr, ok := err.(*exec.ExitError); ok && exitErr.ExitCode() == 2 {
		// flag.Parse exits 2 for -help — expected
		return
	}
	t.Fatalf("ember -help: unexpected error: %v\noutput: %s", err, out)
}
