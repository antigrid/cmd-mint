package parser

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"cmd-mint/internal/model"
)

func TestParseZshPlainHistoryFixture(t *testing.T) {
	path := filepath.Join("..", "testdata", "histories", "zsh_plain_history")

	result, err := ParseZshFile(path)
	if err != nil {
		t.Fatalf("ParseZshFile() error = %v", err)
	}

	assertZshSummary(t, result.Summary, 3, 3, 0)
	assertCommands(t, result.Commands, []string{
		"git status",
		"npm run build",
		"# keep this comment-shaped command",
	})
	assertZshCommandMetadata(t, result.Commands, path)
	for i, command := range result.Commands {
		if command.Timestamp != nil {
			t.Fatalf("command %d Timestamp = %v, want nil", i, command.Timestamp)
		}
	}
}

func TestParseZshExtendedHistoryFixtureExtractsTimestampsAndCommands(t *testing.T) {
	path := filepath.Join("..", "testdata", "histories", "zsh_extended_history")

	result, err := ParseZshFile(path)
	if err != nil {
		t.Fatalf("ParseZshFile() error = %v", err)
	}

	assertZshSummary(t, result.Summary, 3, 3, 0)
	assertCommands(t, result.Commands, []string{
		"git status",
		"npm run build",
		"printf 'one;two'",
	})
	assertTimestamp(t, result.Commands[0].Timestamp, 1717180000)
	assertTimestamp(t, result.Commands[1].Timestamp, 1717180050)
	assertTimestamp(t, result.Commands[2].Timestamp, 1717180060)
	assertZshCommandMetadata(t, result.Commands, path)
}

func TestParseZshMixedPlainAndExtendedHistory(t *testing.T) {
	const history = "git status\n: 1717180000:0;npm run build\n  docker ps  \n: plain zsh no-op\n"

	result, err := ParseZsh(strings.NewReader(history), "/tmp/.zsh_history")
	if err != nil {
		t.Fatalf("ParseZsh() error = %v", err)
	}

	assertZshSummary(t, result.Summary, 4, 4, 0)
	assertCommands(t, result.Commands, []string{
		"git status",
		"npm run build",
		"docker ps",
		": plain zsh no-op",
	})
	if result.Commands[0].Timestamp != nil {
		t.Fatalf("plain command Timestamp = %v, want nil", result.Commands[0].Timestamp)
	}
	assertTimestamp(t, result.Commands[1].Timestamp, 1717180000)
	if result.Commands[2].EntryIndex != 3 {
		t.Fatalf("third EntryIndex = %d, want 3", result.Commands[2].EntryIndex)
	}
}

func TestParseZshMalformedExtendedEntriesAreSkippedWithoutRawWarningLeakage(t *testing.T) {
	const secret = "TOKEN=super-secret"
	history := strings.Join([]string{
		": 1717180000:0;git status",
		": not-a-timestamp:0;" + secret,
		": 1717180001:nope;" + secret,
		": 1717180002:0",
		": 1717180003:0;",
		"docker ps",
	}, "\n")

	result, err := ParseZsh(strings.NewReader(history), "/tmp/.zsh_history")
	if err != nil {
		t.Fatalf("ParseZsh() error = %v", err)
	}

	assertZshSummary(t, result.Summary, 6, 2, 4)
	assertCommands(t, result.Commands, []string{
		"git status",
		"docker ps",
	})
	if len(result.Summary.Warnings) != 1 {
		t.Fatalf("Warnings = %#v, want one malformed zsh warning", result.Summary.Warnings)
	}
	if result.Summary.Warnings[0] != malformedZshExtendedWarning {
		t.Fatalf("Warnings[0] = %q, want %q", result.Summary.Warnings[0], malformedZshExtendedWarning)
	}
	for _, warning := range result.Summary.Warnings {
		if strings.Contains(warning, secret) || strings.Contains(warning, "not-a-timestamp") {
			t.Fatalf("warning leaks raw malformed entry data: %q", warning)
		}
	}
	if result.Commands[1].EntryIndex != 6 {
		t.Fatalf("second parsed EntryIndex = %d, want 6", result.Commands[1].EntryIndex)
	}
}

func TestParseZshSkipsBlankLines(t *testing.T) {
	result, err := ParseZsh(strings.NewReader("\n  \r\n git status \n"), "/tmp/.zsh_history")
	if err != nil {
		t.Fatalf("ParseZsh() error = %v", err)
	}

	assertZshSummary(t, result.Summary, 3, 1, 2)
	assertCommands(t, result.Commands, []string{"git status"})
}

func TestParseZshInvalidUTF8UsesSafeReplacementAndWarning(t *testing.T) {
	result, err := ParseZsh(strings.NewReader(": 1717180000:0;kubectl get \xff pods\n"), "/tmp/.zsh_history")
	if err != nil {
		t.Fatalf("ParseZsh() error = %v", err)
	}

	assertZshSummary(t, result.Summary, 1, 1, 0)
	assertCommands(t, result.Commands, []string{"kubectl get \uFFFD pods"})
	if len(result.Summary.Warnings) != 1 {
		t.Fatalf("Warnings = %#v, want one invalid UTF-8 warning", result.Summary.Warnings)
	}
	if result.Summary.Warnings[0] != invalidUTF8Warning {
		t.Fatalf("Warnings[0] = %q, want %q", result.Summary.Warnings[0], invalidUTF8Warning)
	}
}

func TestParseZshFileDoesNotModifyHistory(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, ".zsh_history")
	const history = ": 1717180000:0;git status\n"
	if err := os.WriteFile(path, []byte(history), 0o600); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	if _, err := ParseZshFile(path); err != nil {
		t.Fatalf("ParseZshFile() error = %v", err)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}
	if string(data) != history {
		t.Fatalf("history file changed: %q", string(data))
	}
}

func TestParseZshReturnsReadErrorAfterPartialLine(t *testing.T) {
	reader := &errorAfterReader{
		data: []byte(": 1717180000:0;git status"),
		err:  errors.New("read failed"),
	}

	result, err := ParseZsh(reader, "/tmp/.zsh_history")
	if err == nil {
		t.Fatal("ParseZsh() error = nil, want read error")
	}
	if len(result.Commands) != 1 {
		t.Fatalf("len(Commands) = %d, want parsed partial command before error", len(result.Commands))
	}
}

func assertZshSummary(t *testing.T, summary model.SourceSummary, read int, parsed int, skipped int) {
	t.Helper()

	if summary.SourceShell != model.ShellZsh {
		t.Fatalf("SourceShell = %q, want zsh", summary.SourceShell)
	}
	if summary.EntriesRead != read {
		t.Fatalf("EntriesRead = %d, want %d", summary.EntriesRead, read)
	}
	if summary.EntriesParsed != parsed {
		t.Fatalf("EntriesParsed = %d, want %d", summary.EntriesParsed, parsed)
	}
	if summary.EntriesSkipped != skipped {
		t.Fatalf("EntriesSkipped = %d, want %d", summary.EntriesSkipped, skipped)
	}
}

func assertZshCommandMetadata(t *testing.T, commands []model.CommandRecord, sourceFile string) {
	t.Helper()

	for i, command := range commands {
		if command.SourceShell != model.ShellZsh {
			t.Fatalf("command %d SourceShell = %q, want zsh", i, command.SourceShell)
		}
		if command.SourceFile != sourceFile {
			t.Fatalf("command %d SourceFile = %q, want %q", i, command.SourceFile, sourceFile)
		}
		if command.EntryIndex != i+1 {
			t.Fatalf("command %d EntryIndex = %d, want %d", i, command.EntryIndex, i+1)
		}
		if command.ParseStatus != model.ParseStatusParsed {
			t.Fatalf("command %d ParseStatus = %q, want parsed", i, command.ParseStatus)
		}
	}
}
