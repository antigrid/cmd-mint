package main

import (
	"os/exec"
	"path/filepath"
	"testing"
)

func TestInvalidShellExitsCode2WhenRunAsCommand(t *testing.T) {
	binary := filepath.Join(t.TempDir(), "cmd-mint")

	build := exec.Command("go", "build", "-o", binary, ".")
	if output, err := build.CombinedOutput(); err != nil {
		t.Fatalf("go build failed: %v\n%s", err, output)
	}

	cmd := exec.Command(binary, "--shell", "powershell")
	output, err := cmd.CombinedOutput()
	if err == nil {
		t.Fatalf("cmd-mint unexpectedly succeeded:\n%s", output)
	}

	exitErr, ok := err.(*exec.ExitError)
	if !ok {
		t.Fatalf("cmd-mint error = %T %v, want *exec.ExitError", err, err)
	}
	if exitErr.ExitCode() != 2 {
		t.Fatalf("cmd-mint exit code = %d, want 2\n%s", exitErr.ExitCode(), output)
	}
}
