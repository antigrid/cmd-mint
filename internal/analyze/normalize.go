package analyze

import (
	"strings"
	"unicode"

	"cmd-mint/internal/model"
)

type NormalizedCommand struct {
	Command     string
	IsMultiline bool
}

type CommandAnalysis struct {
	NormalizedCommand string
	Tokens            []string
	Tool              string
	Subcommand        string
	IsMultiline       bool
}

func AnalyzeCommand(raw string) CommandAnalysis {
	normalized := NormalizeCommand(raw)
	tokens := Tokenize(normalized.Command)
	group := DetectTool(tokens)

	return CommandAnalysis{
		NormalizedCommand: normalized.Command,
		Tokens:            tokens,
		Tool:              group.Tool,
		Subcommand:        group.Subcommand,
		IsMultiline:       normalized.IsMultiline || ContainsHeredoc(tokens),
	}
}

func AnalyzeRecord(record model.CommandRecord) model.CommandRecord {
	analysis := AnalyzeCommand(record.RawCommand)
	record.NormalizedCommand = analysis.NormalizedCommand
	record.Tokens = analysis.Tokens
	record.Tool = analysis.Tool
	record.Subcommand = analysis.Subcommand
	record.IsMultiline = analysis.IsMultiline
	return record
}

func NormalizeCommand(raw string) NormalizedCommand {
	trimmed := strings.TrimSpace(raw)
	isMultiline := strings.ContainsAny(trimmed, "\n\r")
	if trimmed == "" {
		return NormalizedCommand{IsMultiline: isMultiline}
	}

	var builder strings.Builder
	builder.Grow(len(trimmed))

	var quote rune
	var escaped bool
	var wroteSpace bool

	for _, r := range trimmed {
		switch {
		case escaped:
			builder.WriteRune(r)
			escaped = false
			wroteSpace = false
		case r == '\\' && quote != '\'':
			builder.WriteRune(r)
			escaped = true
			wroteSpace = false
		case quote == 0 && (r == '\'' || r == '"'):
			quote = r
			builder.WriteRune(r)
			wroteSpace = false
		case quote == r:
			quote = 0
			builder.WriteRune(r)
			wroteSpace = false
		case quote == 0 && unicode.IsSpace(r):
			if builder.Len() > 0 && !wroteSpace {
				builder.WriteByte(' ')
				wroteSpace = true
			}
		default:
			builder.WriteRune(r)
			wroteSpace = false
		}
	}

	return NormalizedCommand{
		Command:     strings.TrimSpace(builder.String()),
		IsMultiline: isMultiline,
	}
}
