package parser

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"cmd-mint/internal/model"
)

func TestParseFishHistoryFixtureExtractsCommandsAndTimestamps(t *testing.T) {
	path := filepath.Join("..", "testdata", "histories", "fish_history")

	result, err := ParseFishFile(path)
	if err != nil {
		t.Fatalf("ParseFishFile() error = %v", err)
	}

	assertFishSummary(t, result.Summary, 3, 3, 0)
	assertCommands(t, result.Commands, []string{
		"git status",
		"npm run build",
		"printf 'one\ntwo'",
	})
	assertTimestamp(t, result.Commands[0].Timestamp, 1717180000)
	assertTimestamp(t, result.Commands[1].Timestamp, 1717180050)
	if result.Commands[2].Timestamp != nil {
		t.Fatalf("third command Timestamp = %v, want nil", result.Commands[2].Timestamp)
	}
	assertFishCommandMetadata(t, result.Commands, path)
}

func TestParseFishSkipsMalformedRecordAndContinues(t *testing.T) {
	const secret = "TOKEN=super-secret"
	history := strings.Join([]string{
		"- cmd: git status",
		"  when: 1717180000",
		"- cmd: " + secret,
		"  when: not-a-timestamp",
		"- cmd:",
		"  when: 1717180001",
		"- cmd: docker ps",
	}, "\n")

	result, err := ParseFish(strings.NewReader(history), "/tmp/fish_history")
	if err != nil {
		t.Fatalf("ParseFish() error = %v", err)
	}

	assertFishSummary(t, result.Summary, 4, 2, 2)
	assertCommands(t, result.Commands, []string{
		"git status",
		"docker ps",
	})
	if len(result.Summary.Warnings) != 1 {
		t.Fatalf("Warnings = %#v, want one malformed fish warning", result.Summary.Warnings)
	}
	if result.Summary.Warnings[0] != malformedFishRecordWarning {
		t.Fatalf("Warnings[0] = %q, want %q", result.Summary.Warnings[0], malformedFishRecordWarning)
	}
	for _, warning := range result.Summary.Warnings {
		if strings.Contains(warning, secret) || strings.Contains(warning, "not-a-timestamp") {
			t.Fatalf("warning leaks raw malformed entry data: %q", warning)
		}
	}
	if result.Commands[1].EntryIndex != 4 {
		t.Fatalf("second parsed EntryIndex = %d, want 4", result.Commands[1].EntryIndex)
	}
}

func TestParseFishEscapedNewline(t *testing.T) {
	const history = "- cmd: printf 'one\\ntwo'\n  when: 1717180000\n"

	result, err := ParseFish(strings.NewReader(history), "/tmp/fish_history")
	if err != nil {
		t.Fatalf("ParseFish() error = %v", err)
	}

	assertFishSummary(t, result.Summary, 1, 1, 0)
	assertCommands(t, result.Commands, []string{"printf 'one\ntwo'"})
}

func TestParseFishPhysicalContinuation(t *testing.T) {
	const history = "- cmd: printf 'one\n  two'\n  when: 1717180000\n"

	result, err := ParseFish(strings.NewReader(history), "/tmp/fish_history")
	if err != nil {
		t.Fatalf("ParseFish() error = %v", err)
	}

	assertFishSummary(t, result.Summary, 1, 1, 0)
	assertCommands(t, result.Commands, []string{"printf 'one\ntwo'"})
}

func TestParseFishBlockScalarLiteral(t *testing.T) {
	history := strings.Join([]string{
		"- cmd: |",
		"    printf 'one: two'",
		"    echo done",
		"  when: 1717180000",
		"- cmd: docker ps",
		"  when: 1717180001",
	}, "\n")

	result, err := ParseFish(strings.NewReader(history), "/tmp/fish_history")
	if err != nil {
		t.Fatalf("ParseFish() error = %v", err)
	}

	assertFishSummary(t, result.Summary, 2, 2, 0)
	assertCommands(t, result.Commands, []string{
		"printf 'one: two'\necho done",
		"docker ps",
	})
	assertTimestamp(t, result.Commands[0].Timestamp, 1717180000)
	if result.Commands[0].EntryIndex != 1 {
		t.Fatalf("first command EntryIndex = %d, want 1", result.Commands[0].EntryIndex)
	}
}

func TestParseFishBlockScalarFoldedKeptMultilineForSafety(t *testing.T) {
	history := strings.Join([]string{
		"- cmd: >",
		"    git status",
		"    git branch --show-current",
		"  when: 1717180000",
	}, "\n")

	result, err := ParseFish(strings.NewReader(history), "/tmp/fish_history")
	if err != nil {
		t.Fatalf("ParseFish() error = %v", err)
	}

	assertFishSummary(t, result.Summary, 1, 1, 0)
	assertCommands(t, result.Commands, []string{"git status\ngit branch --show-current"})
}

func TestParseFishEmptyBlockScalarIsMalformedRecord(t *testing.T) {
	const secret = "TOKEN=super-secret"
	history := strings.Join([]string{
		"- cmd: |",
		"  when: 1717180000",
		"- cmd: git status",
		"  when: 1717180001",
		"- cmd: " + secret,
		"  when: not-a-timestamp",
	}, "\n")

	result, err := ParseFish(strings.NewReader(history), "/tmp/fish_history")
	if err != nil {
		t.Fatalf("ParseFish() error = %v", err)
	}

	assertFishSummary(t, result.Summary, 3, 1, 2)
	assertCommands(t, result.Commands, []string{"git status"})
	if len(result.Summary.Warnings) != 1 {
		t.Fatalf("Warnings = %#v, want one malformed fish warning", result.Summary.Warnings)
	}
	if result.Summary.Warnings[0] != malformedFishRecordWarning {
		t.Fatalf("Warnings[0] = %q, want %q", result.Summary.Warnings[0], malformedFishRecordWarning)
	}
	for _, warning := range result.Summary.Warnings {
		if strings.Contains(warning, secret) || strings.Contains(warning, "not-a-timestamp") {
			t.Fatalf("warning leaks raw malformed entry data: %q", warning)
		}
	}
}

func TestParseFishBlockScalarInvalidUTF8UsesSafeReplacementAndWarning(t *testing.T) {
	result, err := ParseFish(strings.NewReader("- cmd: |\n    kubectl get \xff pods\n"), "/tmp/fish_history")
	if err != nil {
		t.Fatalf("ParseFish() error = %v", err)
	}

	assertFishSummary(t, result.Summary, 1, 1, 0)
	assertCommands(t, result.Commands, []string{"kubectl get \uFFFD pods"})
	if len(result.Summary.Warnings) != 1 {
		t.Fatalf("Warnings = %#v, want one invalid UTF-8 warning", result.Summary.Warnings)
	}
	if result.Summary.Warnings[0] != invalidUTF8Warning {
		t.Fatalf("Warnings[0] = %q, want %q", result.Summary.Warnings[0], invalidUTF8Warning)
	}
}

func TestParseFishInvalidUTF8UsesSafeReplacementAndWarning(t *testing.T) {
	result, err := ParseFish(strings.NewReader("- cmd: kubectl get \xff pods\n"), "/tmp/fish_history")
	if err != nil {
		t.Fatalf("ParseFish() error = %v", err)
	}

	assertFishSummary(t, result.Summary, 1, 1, 0)
	assertCommands(t, result.Commands, []string{"kubectl get \uFFFD pods"})
	if len(result.Summary.Warnings) != 1 {
		t.Fatalf("Warnings = %#v, want one invalid UTF-8 warning", result.Summary.Warnings)
	}
	if result.Summary.Warnings[0] != invalidUTF8Warning {
		t.Fatalf("Warnings[0] = %q, want %q", result.Summary.Warnings[0], invalidUTF8Warning)
	}
}

func TestParseFishFileDoesNotModifyHistory(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "fish_history")
	const history = "- cmd: git status\n  when: 1717180000\n"
	if err := os.WriteFile(path, []byte(history), 0o600); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	if _, err := ParseFishFile(path); err != nil {
		t.Fatalf("ParseFishFile() error = %v", err)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}
	if string(data) != history {
		t.Fatalf("history file changed: %q", string(data))
	}
}

func TestParseFishReturnsReadErrorAfterPartialLine(t *testing.T) {
	reader := &errorAfterReader{
		data: []byte("- cmd: git status"),
		err:  errors.New("read failed"),
	}

	result, err := ParseFish(reader, "/tmp/fish_history")
	if err == nil {
		t.Fatal("ParseFish() error = nil, want read error")
	}
	if len(result.Commands) != 1 {
		t.Fatalf("len(Commands) = %d, want parsed partial command before error", len(result.Commands))
	}
}

func assertFishSummary(t *testing.T, summary model.SourceSummary, read int, parsed int, skipped int) {
	t.Helper()

	if summary.SourceShell != model.ShellFish {
		t.Fatalf("SourceShell = %q, want fish", summary.SourceShell)
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

func assertFishCommandMetadata(t *testing.T, commands []model.CommandRecord, sourceFile string) {
	t.Helper()

	for i, command := range commands {
		if command.SourceShell != model.ShellFish {
			t.Fatalf("command %d SourceShell = %q, want fish", i, command.SourceShell)
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
