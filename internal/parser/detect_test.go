package parser

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"cmd-mint/internal/model"
)

func TestDetectShellChoosesZshExtended(t *testing.T) {
	shell, err := DetectShell(strings.NewReader(": 1717180000:0;git status\n"))
	if err != nil {
		t.Fatalf("DetectShell() error = %v", err)
	}
	if shell != model.ShellZsh {
		t.Fatalf("DetectShell() = %q, want zsh", shell)
	}
}

func TestDetectShellChoosesFish(t *testing.T) {
	shell, err := DetectShell(strings.NewReader("- cmd: git status\n  when: 1717180000\n"))
	if err != nil {
		t.Fatalf("DetectShell() error = %v", err)
	}
	if shell != model.ShellFish {
		t.Fatalf("DetectShell() = %q, want fish", shell)
	}
}

func TestDetectShellChoosesBashTimestampHistory(t *testing.T) {
	shell, err := DetectShell(strings.NewReader("#1717180000\ngit status\n"))
	if err != nil {
		t.Fatalf("DetectShell() error = %v", err)
	}
	if shell != model.ShellBash {
		t.Fatalf("DetectShell() = %q, want bash", shell)
	}
}

func TestDetectShellChoosesBashForPlainHistory(t *testing.T) {
	shell, err := DetectShell(strings.NewReader("git status\nnpm run build\n"))
	if err != nil {
		t.Fatalf("DetectShell() error = %v", err)
	}
	if shell != model.ShellBash {
		t.Fatalf("DetectShell() = %q, want bash", shell)
	}
}

func TestParseFileAutoDetectsFishAndPreservesMetadata(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "history")
	if err := os.WriteFile(path, []byte("- cmd: git status\n  when: 1717180000\n"), 0o600); err != nil {
		t.Fatalf("WriteFile(%q) error = %v", path, err)
	}

	result, err := ParseFile(model.HistorySource{
		SourceShell: model.ShellAuto,
		SourceFile:  path,
	})
	if err != nil {
		t.Fatalf("ParseFile() error = %v", err)
	}

	assertFishSummary(t, result.Summary, 1, 1, 0)
	if result.Summary.SourceFile != path {
		t.Fatalf("Summary.SourceFile = %q, want %q", result.Summary.SourceFile, path)
	}
	assertCommands(t, result.Commands, []string{"git status"})
	if result.Commands[0].SourceShell != model.ShellFish {
		t.Fatalf("command SourceShell = %q, want fish", result.Commands[0].SourceShell)
	}
}

func TestParseFileUsesExplicitShellWithoutAutoDetection(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "history")
	if err := os.WriteFile(path, []byte("- cmd: git status\n"), 0o600); err != nil {
		t.Fatalf("WriteFile(%q) error = %v", path, err)
	}

	result, err := ParseFile(model.HistorySource{
		SourceShell: model.ShellBash,
		SourceFile:  path,
	})
	if err != nil {
		t.Fatalf("ParseFile() error = %v", err)
	}

	if result.Summary.SourceShell != model.ShellBash {
		t.Fatalf("Summary.SourceShell = %q, want bash", result.Summary.SourceShell)
	}
	assertCommands(t, result.Commands, []string{"- cmd: git status"})
}
