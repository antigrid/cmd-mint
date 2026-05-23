package parser

import (
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"cmd-mint/internal/model"
)

func TestParseBashPlainHistoryFixture(t *testing.T) {
	path := filepath.Join("..", "testdata", "histories", "bash_plain_history")

	result, err := ParseBashFile(path)
	if err != nil {
		t.Fatalf("ParseBashFile() error = %v", err)
	}

	assertSummary(t, result.Summary, 3, 3, 0)
	assertCommands(t, result.Commands, []string{
		"git status",
		"npm run build",
		"# keep this comment-shaped command",
	})
	for i, command := range result.Commands {
		if command.SourceShell != model.ShellBash {
			t.Fatalf("command %d SourceShell = %q, want bash", i, command.SourceShell)
		}
		if command.SourceFile != path {
			t.Fatalf("command %d SourceFile = %q, want %q", i, command.SourceFile, path)
		}
		if command.EntryIndex != i+1 {
			t.Fatalf("command %d EntryIndex = %d, want %d", i, command.EntryIndex, i+1)
		}
		if command.ParseStatus != model.ParseStatusParsed {
			t.Fatalf("command %d ParseStatus = %q, want parsed", i, command.ParseStatus)
		}
		if command.Timestamp != nil {
			t.Fatalf("command %d Timestamp = %v, want nil", i, command.Timestamp)
		}
	}
}

func TestParseBashTimestampMarkersAttachToFollowingCommand(t *testing.T) {
	const history = "#1717180000\ngit status\n#1717180050\nnpm run build\n# not a timestamp\n"

	result, err := ParseBash(strings.NewReader(history), "/tmp/.bash_history")
	if err != nil {
		t.Fatalf("ParseBash() error = %v", err)
	}

	assertSummary(t, result.Summary, 3, 3, 0)
	assertCommands(t, result.Commands, []string{
		"git status",
		"npm run build",
		"# not a timestamp",
	})

	assertTimestamp(t, result.Commands[0].Timestamp, 1717180000)
	assertTimestamp(t, result.Commands[1].Timestamp, 1717180050)
	if result.Commands[2].Timestamp != nil {
		t.Fatalf("non-timestamp comment command Timestamp = %v, want nil", result.Commands[2].Timestamp)
	}
}

func TestParseBashEmptyFile(t *testing.T) {
	result, err := ParseBash(strings.NewReader(""), "/tmp/empty")
	if err != nil {
		t.Fatalf("ParseBash() error = %v", err)
	}

	assertSummary(t, result.Summary, 0, 0, 0)
	if len(result.Commands) != 0 {
		t.Fatalf("len(Commands) = %d, want 0", len(result.Commands))
	}
}

func TestParseBashSkipsBlankLines(t *testing.T) {
	result, err := ParseBash(strings.NewReader("\n  \r\n git status \n"), "/tmp/.bash_history")
	if err != nil {
		t.Fatalf("ParseBash() error = %v", err)
	}

	assertSummary(t, result.Summary, 3, 1, 2)
	assertCommands(t, result.Commands, []string{"git status"})
}

func TestParseBashInvalidUTF8UsesSafeReplacementAndWarning(t *testing.T) {
	result, err := ParseBash(strings.NewReader("git status\nkubectl get \xff pods\n"), "/tmp/.bash_history")
	if err != nil {
		t.Fatalf("ParseBash() error = %v", err)
	}

	assertSummary(t, result.Summary, 2, 2, 0)
	assertCommands(t, result.Commands, []string{
		"git status",
		"kubectl get \uFFFD pods",
	})
	if len(result.Summary.Warnings) != 1 {
		t.Fatalf("Warnings = %#v, want one invalid UTF-8 warning", result.Summary.Warnings)
	}
	if result.Summary.Warnings[0] != invalidUTF8Warning {
		t.Fatalf("Warnings[0] = %q, want %q", result.Summary.Warnings[0], invalidUTF8Warning)
	}
}

func TestParseBashSkipsHeredocBlockWithoutEmittingBodyFragments(t *testing.T) {
	history := strings.Join([]string{
		"git status",
		"cat <<EOF",
		"opaque-body-fragment",
		"Authorization: Bearer fake-token",
		"EOF",
		"docker ps",
	}, "\n")

	result, err := ParseBash(strings.NewReader(history), "/tmp/.bash_history")
	if err != nil {
		t.Fatalf("ParseBash() error = %v", err)
	}

	assertSummary(t, result.Summary, 6, 2, 4)
	assertCommands(t, result.Commands, []string{
		"git status",
		"docker ps",
	})
	if len(result.Summary.Warnings) != 1 || result.Summary.Warnings[0] != bashMultilineSkipWarning {
		t.Fatalf("Warnings = %#v, want one bash multiline warning", result.Summary.Warnings)
	}
	for _, command := range result.Commands {
		if strings.Contains(command.RawCommand, "opaque-body-fragment") || strings.Contains(command.RawCommand, "fake-token") {
			t.Fatalf("heredoc body fragment emitted as command: %#v", command)
		}
	}
}

func TestParseBashSkipsIncompleteQuotedMultilineEntry(t *testing.T) {
	history := strings.Join([]string{
		`printf "first line`,
		"opaque-body-fragment",
		`last line"`,
		"git status",
	}, "\n")

	result, err := ParseBash(strings.NewReader(history), "/tmp/.bash_history")
	if err != nil {
		t.Fatalf("ParseBash() error = %v", err)
	}

	assertSummary(t, result.Summary, 4, 1, 3)
	assertCommands(t, result.Commands, []string{"git status"})
	if len(result.Summary.Warnings) != 1 || result.Summary.Warnings[0] != bashMultilineSkipWarning {
		t.Fatalf("Warnings = %#v, want one bash multiline warning", result.Summary.Warnings)
	}
}

func TestParseBashStreamsReader(t *testing.T) {
	reader := &generatedLineReader{
		line:      "git status\n",
		remaining: 100_000,
		chunkSize: 17,
	}

	result, err := ParseBash(reader, "/tmp/large.bash_history")
	if err != nil {
		t.Fatalf("ParseBash() error = %v", err)
	}

	assertSummary(t, result.Summary, 100_000, 100_000, 0)
	if len(result.Commands) != 100_000 {
		t.Fatalf("len(Commands) = %d, want 100000", len(result.Commands))
	}
	if reader.readCalls <= 1 {
		t.Fatalf("reader readCalls = %d, want streaming multiple reads", reader.readCalls)
	}
}

func TestParseBashFileDoesNotModifyHistory(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, ".bash_history")
	const history = "git status\n"
	if err := os.WriteFile(path, []byte(history), 0o600); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	if _, err := ParseBashFile(path); err != nil {
		t.Fatalf("ParseBashFile() error = %v", err)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}
	if string(data) != history {
		t.Fatalf("history file changed: %q", string(data))
	}
}

func TestParseBashReturnsReadErrorAfterPartialLine(t *testing.T) {
	reader := &errorAfterReader{
		data: []byte("git status"),
		err:  errors.New("read failed"),
	}

	result, err := ParseBash(reader, "/tmp/.bash_history")
	if err == nil {
		t.Fatal("ParseBash() error = nil, want read error")
	}
	if len(result.Commands) != 1 {
		t.Fatalf("len(Commands) = %d, want parsed partial command before error", len(result.Commands))
	}
}

func assertSummary(t *testing.T, summary model.SourceSummary, read int, parsed int, skipped int) {
	t.Helper()

	if summary.SourceShell != model.ShellBash {
		t.Fatalf("SourceShell = %q, want bash", summary.SourceShell)
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

func assertCommands(t *testing.T, commands []model.CommandRecord, want []string) {
	t.Helper()

	if len(commands) != len(want) {
		t.Fatalf("len(Commands) = %d, want %d", len(commands), len(want))
	}
	for i, command := range commands {
		if command.RawCommand != want[i] {
			t.Fatalf("command %d RawCommand = %q, want %q", i, command.RawCommand, want[i])
		}
	}
}

func assertTimestamp(t *testing.T, got *time.Time, unix int64) {
	t.Helper()

	if got == nil {
		t.Fatalf("Timestamp = nil, want %d", unix)
	}
	want := time.Unix(unix, 0).UTC()
	if !got.Equal(want) {
		t.Fatalf("Timestamp = %v, want %v", got, want)
	}
}

type generatedLineReader struct {
	line      string
	remaining int
	chunkSize int
	offset    int
	readCalls int
}

func (r *generatedLineReader) Read(p []byte) (int, error) {
	if r.remaining == 0 {
		return 0, io.EOF
	}
	r.readCalls++
	limit := r.chunkSize
	if len(p) < limit {
		limit = len(p)
	}

	n := 0
	for n < limit && r.remaining > 0 {
		p[n] = r.line[r.offset]
		n++
		r.offset++
		if r.offset == len(r.line) {
			r.offset = 0
			r.remaining--
		}
	}
	return n, nil
}

type errorAfterReader struct {
	data []byte
	err  error
	done bool
}

func (r *errorAfterReader) Read(p []byte) (int, error) {
	if r.done {
		return 0, r.err
	}
	r.done = true
	return copy(p, r.data), nil
}
