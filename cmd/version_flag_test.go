package cmd

import (
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func runCLIFromRepoRoot(t *testing.T, args ...string) (string, error) {
	t.Helper()

	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatalf("failed to determine caller path")
	}
	repoRoot := filepath.Dir(filepath.Dir(file))

	cmdArgs := append([]string{"run", "."}, args...)
	cmd := exec.Command("go", cmdArgs...)
	cmd.Dir = repoRoot
	output, err := cmd.CombinedOutput()
	return string(output), err
}

func TestVersionLongFlag(t *testing.T) {
	output, err := runCLIFromRepoRoot(t, "--version")
	if err != nil {
		t.Fatalf("expected success, got %v: %s", err, output)
	}

	if !strings.Contains(output, "notion-cli version") {
		t.Fatalf("expected version output, got: %s", output)
	}
}

func TestVersionShortFlag(t *testing.T) {
	output, err := runCLIFromRepoRoot(t, "-v")
	if err != nil {
		t.Fatalf("expected success, got %v: %s", err, output)
	}

	if !strings.Contains(output, "notion-cli version") {
		t.Fatalf("expected version output, got: %s", output)
	}
}
