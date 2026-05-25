package parser

import (
	"bufio"
	"errors"
	"io"
	"os"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"cmd-mint/internal/model"
)

const (
	invalidUTF8Warning       = "invalid UTF-8 replaced while parsing history"
	bashMultilineSkipWarning = "bash multiline history entries skipped"
)

type bashHeredocSkip struct {
	delimiter string
	stripTabs bool
}

type shellContinuationSkip struct {
	quote rune
}

func ParseBashFile(path string) (Result, error) {
	file, err := os.Open(path)
	if err != nil {
		return Result{}, err
	}
	defer file.Close()

	return ParseBash(file, path)
}

func ParseBash(r io.Reader, sourceFile string) (Result, error) {
	result := Result{
		Summary: model.SourceSummary{
			SourceShell: model.ShellBash,
			SourceFile:  sourceFile,
		},
	}

	reader := bufio.NewReaderSize(r, 64*1024)
	var pendingTimestamp *time.Time
	var warnedInvalidUTF8 bool
	var warnedMultiline bool
	var heredocSkip *bashHeredocSkip
	var continuationSkip *shellContinuationSkip

	for {
		line, err := reader.ReadString('\n')
		if err != nil && len(line) == 0 {
			if errors.Is(err, io.EOF) {
				break
			}
			return result, err
		}

		line = trimLineEnding(line)
		if !utf8.ValidString(line) {
			line = strings.ToValidUTF8(line, "\uFFFD")
			if !warnedInvalidUTF8 {
				result.Summary.Warnings = append(result.Summary.Warnings, invalidUTF8Warning)
				warnedInvalidUTF8 = true
			}
		}

		if heredocSkip != nil {
			result.Summary.EntriesRead++
			result.Summary.EntriesSkipped++
			if bashHeredocDelimiterMatches(line, *heredocSkip) {
				heredocSkip = nil
			}
			pendingTimestamp = nil
		} else if continuationSkip != nil {
			result.Summary.EntriesRead++
			result.Summary.EntriesSkipped++
			quote, needsMore := shellContinuationState(line, continuationSkip.quote)
			if needsMore {
				continuationSkip.quote = quote
			} else {
				continuationSkip = nil
			}
			pendingTimestamp = nil
		} else if timestamp, ok := parseBashTimestampMarker(line); ok {
			pendingTimestamp = &timestamp
		} else {
			command := strings.TrimSpace(line)
			result.Summary.EntriesRead++
			if command == "" {
				result.Summary.EntriesSkipped++
			} else if heredoc, ok := bashHeredocStart(command); ok {
				result.Summary.EntriesSkipped++
				pendingTimestamp = nil
				heredocSkip = &heredoc
				warnedMultiline = appendBashMultilineWarning(&result, warnedMultiline)
			} else if quote, needsMore := shellContinuationState(command, 0); needsMore {
				result.Summary.EntriesSkipped++
				pendingTimestamp = nil
				continuationSkip = &shellContinuationSkip{quote: quote}
				warnedMultiline = appendBashMultilineWarning(&result, warnedMultiline)
			} else {
				timestamp := cloneTimePointer(pendingTimestamp)
				result.Commands = append(result.Commands, model.CommandRecord{
					SourceShell: model.ShellBash,
					SourceFile:  sourceFile,
					EntryIndex:  result.Summary.EntriesRead,
					Timestamp:   timestamp,
					RawCommand:  command,
					ParseStatus: model.ParseStatusParsed,
				})
				result.Summary.EntriesParsed++
				pendingTimestamp = nil
			}
		}

		if err != nil {
			if errors.Is(err, io.EOF) {
				break
			}
			return result, err
		}
	}

	if pendingTimestamp != nil {
		result.Summary.Warnings = append(result.Summary.Warnings, "timestamp marker without following command")
	}

	return result, nil
}

func appendBashMultilineWarning(result *Result, warned bool) bool {
	if warned {
		return true
	}
	result.Summary.Warnings = append(result.Summary.Warnings, bashMultilineSkipWarning)
	return true
}

func bashHeredocStart(command string) (bashHeredocSkip, bool) {
	tokens := bashShellFields(command)
	for i, token := range tokens {
		if token == "<<<" || strings.HasPrefix(token, "<<<") {
			continue
		}
		switch token {
		case "<<", "<<-":
			if i+1 >= len(tokens) {
				return bashHeredocSkip{}, false
			}
			return bashHeredocSkip{
				delimiter: tokens[i+1],
				stripTabs: token == "<<-",
			}, true
		default:
			operator := strings.Index(token, "<<")
			if operator == -1 {
				continue
			}
			rest := token[operator+2:]
			if strings.HasPrefix(rest, "<") {
				continue
			}
			stripTabs := false
			if strings.HasPrefix(rest, "-") {
				stripTabs = true
				rest = strings.TrimPrefix(rest, "-")
			}
			if rest == "" {
				continue
			}
			return bashHeredocSkip{
				delimiter: rest,
				stripTabs: stripTabs,
			}, true
		}
	}
	return bashHeredocSkip{}, false
}

func bashHeredocDelimiterMatches(line string, heredoc bashHeredocSkip) bool {
	if heredoc.delimiter == "" {
		return true
	}
	candidate := line
	if heredoc.stripTabs {
		candidate = strings.TrimLeft(candidate, "\t")
	}
	return strings.TrimSpace(candidate) == heredoc.delimiter
}

// shellContinuationState reports whether a physical history line leaves a
// command open across a newline, either via a trailing unescaped backslash or
// an unterminated quote. It is shared by the bash and zsh parsers because both
// shells persist multiline commands using the same continuation conventions.
func shellContinuationState(line string, quote rune) (rune, bool) {
	escaped := false
	for _, r := range line {
		if escaped {
			escaped = false
			continue
		}
		if r == '\\' && quote != '\'' {
			escaped = true
			continue
		}
		if quote == 0 && (r == '\'' || r == '"') {
			quote = r
			continue
		}
		if quote == r {
			quote = 0
		}
	}
	return quote, quote != 0 || escaped
}

func bashShellFields(line string) []string {
	var fields []string
	var builder strings.Builder
	var quote rune
	escaped := false
	inToken := false

	flush := func() {
		if !inToken {
			return
		}
		fields = append(fields, builder.String())
		builder.Reset()
		inToken = false
	}

	for _, r := range line {
		if escaped {
			builder.WriteRune(r)
			inToken = true
			escaped = false
			continue
		}
		if r == '\\' && quote != '\'' {
			escaped = true
			inToken = true
			continue
		}
		if quote == 0 && (r == ' ' || r == '\t') {
			flush()
			continue
		}
		if quote == 0 && (r == '\'' || r == '"') {
			quote = r
			inToken = true
			continue
		}
		if quote == r {
			quote = 0
			inToken = true
			continue
		}
		builder.WriteRune(r)
		inToken = true
	}
	if escaped {
		builder.WriteRune('\\')
	}
	flush()

	return fields
}

func trimLineEnding(line string) string {
	line = strings.TrimSuffix(line, "\n")
	return strings.TrimSuffix(line, "\r")
}

func parseBashTimestampMarker(line string) (time.Time, bool) {
	if len(line) < 2 || line[0] != '#' {
		return time.Time{}, false
	}

	value := line[1:]
	for _, r := range value {
		if r < '0' || r > '9' {
			return time.Time{}, false
		}
	}

	seconds, err := strconv.ParseInt(value, 10, 64)
	if err != nil {
		return time.Time{}, false
	}

	return time.Unix(seconds, 0).UTC(), true
}

func cloneTimePointer(value *time.Time) *time.Time {
	if value == nil {
		return nil
	}
	cloned := *value
	return &cloned
}
