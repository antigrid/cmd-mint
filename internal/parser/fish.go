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

const malformedFishRecordWarning = "malformed fish history records skipped"

type fishRecord struct {
	command   string
	timestamp *time.Time
	index     int
	malformed bool
}

func ParseFishFile(path string) (Result, error) {
	file, err := os.Open(path)
	if err != nil {
		return Result{}, err
	}
	defer file.Close()

	return ParseFish(file, path)
}

func ParseFish(r io.Reader, sourceFile string) (Result, error) {
	result := Result{
		Summary: model.SourceSummary{
			SourceShell: model.ShellFish,
			SourceFile:  sourceFile,
		},
	}

	reader := bufio.NewReaderSize(r, 64*1024)
	var current *fishRecord
	var warnedInvalidUTF8 bool
	var warnedMalformedRecord bool

	for {
		line, err := reader.ReadString('\n')
		if err != nil && len(line) == 0 {
			if errors.Is(err, io.EOF) {
				break
			}
			finalizeFishRecord(&result, current, &warnedMalformedRecord)
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

		if command, ok := parseFishCommandLine(line); ok {
			finalizeFishRecord(&result, current, &warnedMalformedRecord)
			result.Summary.EntriesRead++
			current = &fishRecord{
				command: command,
				index:   result.Summary.EntriesRead,
			}
		} else if current != nil {
			applyFishRecordLine(current, line)
		} else if strings.TrimSpace(line) != "" {
			result.Summary.EntriesSkipped++
			if !warnedMalformedRecord {
				result.Summary.Warnings = append(result.Summary.Warnings, malformedFishRecordWarning)
				warnedMalformedRecord = true
			}
		}

		if err != nil {
			if errors.Is(err, io.EOF) {
				break
			}
			finalizeFishRecord(&result, current, &warnedMalformedRecord)
			return result, err
		}
	}

	finalizeFishRecord(&result, current, &warnedMalformedRecord)
	return result, nil
}

func parseFishCommandLine(line string) (string, bool) {
	trimmed := strings.TrimLeft(line, " \t")
	const prefix = "- cmd:"
	if !strings.HasPrefix(trimmed, prefix) {
		return "", false
	}
	return strings.TrimSpace(trimmed[len(prefix):]), true
}

func applyFishRecordLine(record *fishRecord, line string) {
	trimmed := strings.TrimLeft(line, " \t")
	switch {
	case strings.HasPrefix(trimmed, "when:"):
		timestamp, ok := parseFishTimestamp(strings.TrimSpace(trimmed[len("when:"):]))
		if !ok {
			record.malformed = true
			return
		}
		record.timestamp = timestamp
	case strings.TrimSpace(line) == "":
		return
	case isFishCommandContinuation(line):
		record.command += "\n" + strings.TrimSpace(line)
	}
}

func isFishCommandContinuation(line string) bool {
	if len(line) == 0 || (line[0] != ' ' && line[0] != '\t') {
		return false
	}
	trimmed := strings.TrimSpace(line)
	if trimmed == "" {
		return false
	}
	if strings.Contains(trimmed, ":") {
		return false
	}
	return !strings.HasPrefix(trimmed, "- ")
}

func parseFishTimestamp(value string) (*time.Time, bool) {
	if value == "" || !isASCIIUnsignedInteger(value) {
		return nil, false
	}
	seconds, err := strconv.ParseInt(value, 10, 64)
	if err != nil {
		return nil, false
	}
	timestamp := time.Unix(seconds, 0).UTC()
	return &timestamp, true
}

func finalizeFishRecord(result *Result, record *fishRecord, warnedMalformedRecord *bool) {
	if record == nil {
		return
	}

	command := decodeFishEscapes(strings.TrimSpace(record.command))
	if record.malformed || command == "" {
		result.Summary.EntriesSkipped++
		if !*warnedMalformedRecord {
			result.Summary.Warnings = append(result.Summary.Warnings, malformedFishRecordWarning)
			*warnedMalformedRecord = true
		}
		return
	}

	result.Commands = append(result.Commands, model.CommandRecord{
		SourceShell: model.ShellFish,
		SourceFile:  result.Summary.SourceFile,
		EntryIndex:  record.index,
		Timestamp:   record.timestamp,
		RawCommand:  command,
		ParseStatus: model.ParseStatusParsed,
	})
	result.Summary.EntriesParsed++
}

func decodeFishEscapes(value string) string {
	var builder strings.Builder
	builder.Grow(len(value))
	for i := 0; i < len(value); i++ {
		if value[i] != '\\' || i+1 >= len(value) {
			builder.WriteByte(value[i])
			continue
		}

		i++
		switch value[i] {
		case 'n':
			builder.WriteByte('\n')
		case 'r':
			builder.WriteByte('\r')
		case 't':
			builder.WriteByte('\t')
		case '\\':
			builder.WriteByte('\\')
		default:
			builder.WriteByte('\\')
			builder.WriteByte(value[i])
		}
	}
	return builder.String()
}
