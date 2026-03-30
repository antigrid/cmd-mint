package cli

import (
	"bytes"
	"strings"
	"testing"

	"cmd-mint/internal/version"
)

func TestHelp(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	code := Run([]string{"--help"}, &stdout, &stderr, version.BuildInfo{})

	if code != ExitOK {
		t.Fatalf("Run(--help) exit code = %d, want %d", code, ExitOK)
	}
	if !strings.Contains(stdout.String(), "Usage:") {
		t.Fatalf("help output missing usage: %q", stdout.String())
	}
	if stderr.Len() != 0 {
		t.Fatalf("stderr = %q, want empty", stderr.String())
	}
}

func TestVersion(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	build := version.BuildInfo{
		Version: "1.2.3",
		Commit:  "abc123",
		Date:    "2026-06-01",
	}
	code := Run([]string{"--version"}, &stdout, &stderr, build)

	if code != ExitOK {
		t.Fatalf("Run(--version) exit code = %d, want %d", code, ExitOK)
	}
	want := "cmd-mint version 1.2.3 commit abc123 built 2026-06-01\n"
	if stdout.String() != want {
		t.Fatalf("version output = %q, want %q", stdout.String(), want)
	}
	if stderr.Len() != 0 {
		t.Fatalf("stderr = %q, want empty", stderr.String())
	}
}

func TestUnknownFlag(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	code := Run([]string{"--history-file", "history"}, &stdout, &stderr, version.BuildInfo{})

	if code != ExitInvalidArgs {
		t.Fatalf("Run(unknown flag) exit code = %d, want %d", code, ExitInvalidArgs)
	}
	if stdout.Len() != 0 {
		t.Fatalf("stdout = %q, want empty", stdout.String())
	}
}
