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
	malformedZshExtendedWarning = "malformed zsh extended history entries skipped"
	zshMultilineSkipWarning     = "zsh multiline history entries skipped"
)

func ParseZshFile(path string) (Result, error) {
	file, err := os.Open(path)
	if err != nil {
		return Result{}, err
	}
	defer file.Close()

	return ParseZsh(file, path)
}

func ParseZsh(r io.Reader, sourceFile string) (Result, error) {
	result := Result{
		Summary: model.SourceSummary{
			SourceShell: model.ShellZsh,
			SourceFile:  sourceFile,
		},
	}

	reader := bufio.NewReaderSize(r, 64*1024)
	var warnedInvalidUTF8 bool
	var warnedMalformedExtended bool
	var warnedMultiline bool
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

		result.Summary.EntriesRead++

		if continuationSkip != nil {
			// Subsequent physical lines of a multiline command carry no entry
			// metadata. Skip them so command fragments (which may include secret
			// values stripped of the naming context that flags them) never become
			// their own records.
			result.Summary.EntriesSkipped++
			if quote, needsMore := shellContinuationState(line, continuationSkip.quote); needsMore {
				continuationSkip.quote = quote
			} else {
				continuationSkip = nil
			}
		} else {
			command, timestamp, status := parseZshEntry(line)
			switch status {
			case model.ParseStatusParsed:
				if quote, needsMore := shellContinuationState(command, 0); needsMore {
					// zsh persists multiline commands by escaping each embedded
					// newline with a trailing backslash, so the continuation state
					// also covers heredocs and quoted blocks.
					result.Summary.EntriesSkipped++
					continuationSkip = &shellContinuationSkip{quote: quote}
					warnedMultiline = appendZshMultilineWarning(&result, warnedMultiline)
					break
				}
				result.Commands = append(result.Commands, model.CommandRecord{
					SourceShell: model.ShellZsh,
					SourceFile:  sourceFile,
					EntryIndex:  result.Summary.EntriesRead,
					Timestamp:   timestamp,
					RawCommand:  command,
					ParseStatus: model.ParseStatusParsed,
				})
				result.Summary.EntriesParsed++
			case model.ParseStatusMalformedEntry:
				result.Summary.EntriesSkipped++
				if !warnedMalformedExtended {
					result.Summary.Warnings = append(result.Summary.Warnings, malformedZshExtendedWarning)
					warnedMalformedExtended = true
				}
			default:
				result.Summary.EntriesSkipped++
			}
		}

		if err != nil {
			if errors.Is(err, io.EOF) {
				break
			}
			return result, err
		}
	}

	return result, nil
}

func appendZshMultilineWarning(result *Result, warned bool) bool {
	if warned {
		return true
	}
	result.Summary.Warnings = append(result.Summary.Warnings, zshMultilineSkipWarning)
	return true
}

func parseZshEntry(line string) (string, *time.Time, model.ParseStatus) {
	if looksLikeZshExtendedHistory(line) {
		return parseZshExtendedEntry(line)
	}

	command := strings.TrimSpace(line)
	if command == "" {
		return "", nil, model.ParseStatusSkipped
	}
	return command, nil, model.ParseStatusParsed
}

func looksLikeZshExtendedHistory(line string) bool {
	if !strings.HasPrefix(line, ": ") {
		return false
	}

	metadataStart := len(": ")
	if len(line) > metadataStart && line[metadataStart] >= '0' && line[metadataStart] <= '9' {
		return true
	}

	if semicolon := strings.IndexByte(line, ';'); semicolon != -1 {
		return strings.Contains(line[metadataStart:semicolon], ":")
	}
	return false
}

func parseZshExtendedEntry(line string) (string, *time.Time, model.ParseStatus) {
	semicolon := strings.IndexByte(line, ';')
	if semicolon == -1 {
		return "", nil, model.ParseStatusMalformedEntry
	}

	metadata := strings.TrimPrefix(line[:semicolon], ": ")
	parts := strings.SplitN(metadata, ":", 2)
	if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
		return "", nil, model.ParseStatusMalformedEntry
	}
	if !isASCIIUnsignedInteger(parts[0]) || !isASCIIUnsignedInteger(parts[1]) {
		return "", nil, model.ParseStatusMalformedEntry
	}

	seconds, err := strconv.ParseInt(parts[0], 10, 64)
	if err != nil {
		return "", nil, model.ParseStatusMalformedEntry
	}

	command := strings.TrimSpace(line[semicolon+1:])
	if command == "" {
		return "", nil, model.ParseStatusSkipped
	}

	timestamp := time.Unix(seconds, 0).UTC()
	return command, &timestamp, model.ParseStatusParsed
}

func isASCIIUnsignedInteger(value string) bool {
	for _, r := range value {
		if r < '0' || r > '9' {
			return false
		}
	}
	return true
}
