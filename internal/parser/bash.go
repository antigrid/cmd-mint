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

const invalidUTF8Warning = "invalid UTF-8 replaced while parsing history"

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

		if timestamp, ok := parseBashTimestampMarker(line); ok {
			pendingTimestamp = &timestamp
		} else {
			command := strings.TrimSpace(line)
			result.Summary.EntriesRead++
			if command == "" {
				result.Summary.EntriesSkipped++
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
