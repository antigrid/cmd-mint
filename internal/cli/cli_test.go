package cli

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"cmd-mint/internal/model"
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

	code := Run([]string{"--not-a-real-flag"}, &stdout, &stderr, version.BuildInfo{})

	if code != ExitInvalidArgs {
		t.Fatalf("Run(unknown flag) exit code = %d, want %d", code, ExitInvalidArgs)
	}
	if stdout.Len() != 0 {
		t.Fatalf("stdout = %q, want empty", stdout.String())
	}
}

func TestRepeatableHistoryFilesArePreserved(t *testing.T) {
	dir := t.TempDir()
	first := writeTempHistoryFile(t, dir, "zsh.history")
	second := writeTempHistoryFile(t, dir, "bash.history")

	result, err := parseFlags([]string{
		"--history-file", first,
		"--history-file", second,
	}, &bytes.Buffer{})
	if err != nil {
		t.Fatalf("parseFlags returned error: %v", err)
	}

	want := []string{first, second}
	if strings.Join(result.Options.HistoryFiles, "\x00") != strings.Join(want, "\x00") {
		t.Fatalf("HistoryFiles = %#v, want %#v", result.Options.HistoryFiles, want)
	}
}

func TestParseFlagsDefaults(t *testing.T) {
	result, err := parseFlags(nil, &bytes.Buffer{})
	if err != nil {
		t.Fatalf("parseFlags returned error: %v", err)
	}

	if result.Options.Shell != model.ShellAuto {
		t.Fatalf("Shell = %q, want %q", result.Options.Shell, model.ShellAuto)
	}
	if result.Options.MaxAliases != 25 {
		t.Fatalf("MaxAliases = %d, want 25", result.Options.MaxAliases)
	}
	if result.Options.MinFrequency != 3 {
		t.Fatalf("MinFrequency = %d, want 3", result.Options.MinFrequency)
	}
}

func TestValidationRejectsInvalidShell(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	code := Run([]string{"--shell", "powershell"}, &stdout, &stderr, version.BuildInfo{})

	if code != ExitInvalidArgs {
		t.Fatalf("Run(invalid shell) exit code = %d, want %d", code, ExitInvalidArgs)
	}
	if !strings.Contains(stderr.String(), "--shell must be one of bash, zsh, fish, or auto") {
		t.Fatalf("stderr missing shell validation error: %q", stderr.String())
	}
}

func TestValidationRejectsInvalidFrequencies(t *testing.T) {
	tests := []struct {
		name string
		args []string
		want string
	}{
		{
			name: "zero min frequency",
			args: []string{"--min-frequency", "0"},
			want: "--min-frequency must be >= 1",
		},
		{
			name: "negative max aliases",
			args: []string{"--max-aliases", "-1"},
			want: "--max-aliases must be >= 0",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var stdout bytes.Buffer
			var stderr bytes.Buffer

			code := Run(tt.args, &stdout, &stderr, version.BuildInfo{})

			if code != ExitInvalidArgs {
				t.Fatalf("Run(%v) exit code = %d, want %d", tt.args, code, ExitInvalidArgs)
			}
			if stdout.Len() != 0 {
				t.Fatalf("stdout = %q, want empty", stdout.String())
			}
			if !strings.Contains(stderr.String(), tt.want) {
				t.Fatalf("stderr = %q, want substring %q", stderr.String(), tt.want)
			}
		})
	}
}

func TestValidationRejectsUnreadableHistoryFile(t *testing.T) {
	missing := filepath.Join(t.TempDir(), "missing.history")

	_, err := parseFlags([]string{"--history-file", missing}, &bytes.Buffer{})
	if err == nil {
		t.Fatal("parseFlags returned nil error for missing history file")
	}
	if !strings.Contains(err.Error(), "--history-file") {
		t.Fatalf("error = %q, want --history-file context", err.Error())
	}
}

func TestValidationRejectsOutputDirExistingRegularFile(t *testing.T) {
	dir := t.TempDir()
	outputFile := filepath.Join(dir, "report")
	if err := os.WriteFile(outputFile, []byte("not a directory"), 0o600); err != nil {
		t.Fatalf("WriteFile(%q) error = %v", outputFile, err)
	}

	_, err := parseFlags([]string{"--output-dir", outputFile}, &bytes.Buffer{})
	if err == nil {
		t.Fatal("parseFlags returned nil error for regular-file output dir")
	}
	if !strings.Contains(err.Error(), "must not be an existing regular file") {
		t.Fatalf("error = %q, want existing regular file validation", err.Error())
	}
}

func TestOutputDirExistingRegularFileExitsCode3(t *testing.T) {
	dir := t.TempDir()
	outputFile := filepath.Join(dir, "report")
	if err := os.WriteFile(outputFile, []byte("not a directory"), 0o600); err != nil {
		t.Fatalf("WriteFile(%q) error = %v", outputFile, err)
	}

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	code := Run([]string{"--output-dir", outputFile}, &stdout, &stderr, version.BuildInfo{})

	if code != ExitOutputError {
		t.Fatalf("Run(output regular file) exit code = %d, want %d", code, ExitOutputError)
	}
	if stdout.Len() != 0 {
		t.Fatalf("stdout = %q, want empty", stdout.String())
	}
	if !strings.Contains(stderr.String(), "output error") {
		t.Fatalf("stderr = %q, want output error", stderr.String())
	}
}

func TestHelpTextIncludesMVPFlagsAndExcludesDeferredFlags(t *testing.T) {
	text := helpText()

	for _, flag := range []string{
		"--history-file",
		"--shell",
		"--output-dir",
		"--max-aliases",
		"--min-frequency",
		"--no-alias-file",
		"--json",
		"--verbose",
		"--version",
		"--help",
	} {
		if !strings.Contains(text, flag) {
			t.Fatalf("help text missing %s:\n%s", flag, text)
		}
	}

	for _, flag := range []string{
		"--interactive",
		"--include-sensitive",
		"--redact-sensitive",
		"--config",
		"--install-aliases",
		"--follow-sourced-config",
		"--cloud",
	} {
		if strings.Contains(text, flag) {
			t.Fatalf("help text contains deferred flag %s:\n%s", flag, text)
		}
	}
}

func writeTempHistoryFile(t *testing.T, dir string, name string) string {
	t.Helper()

	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, []byte("git status\n"), 0o600); err != nil {
		t.Fatalf("WriteFile(%q) error = %v", path, err)
	}
	return path
}
