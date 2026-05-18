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
	rawSensitivity := ClassifyRawSensitivity(record.RawCommand)
	analysis := AnalyzeCommand(record.RawCommand)
	record.IsMultiline = analysis.IsMultiline
	record.SensitivityFlags = appendSensitivityFlags(record.SensitivityFlags, rawSensitivity.Flags...)
	record.ExclusionReasons = appendExclusionReasons(record.ExclusionReasons, rawSensitivity.Reasons...)

	normalizedSensitivity := ClassifyNormalizedSensitivity(analysis.NormalizedCommand, analysis.Tokens)
	record.SensitivityFlags = appendSensitivityFlags(record.SensitivityFlags, normalizedSensitivity.Flags...)
	record.ExclusionReasons = appendExclusionReasons(record.ExclusionReasons, normalizedSensitivity.Reasons...)

	if IsSensitiveRecord(record) {
		record.NormalizedCommand = ""
		record.DisplayCommand = ""
		record.Tokens = nil
		record.Tool = ""
		record.Subcommand = ""
		return record
	}

	risk := ClassifyRisk(analysis.NormalizedCommand, analysis.Tokens)
	record.RiskFlags = appendRiskFlags(record.RiskFlags, risk.Flags...)
	record.ExclusionReasons = appendExclusionReasons(record.ExclusionReasons, risk.Reasons...)

	record.NormalizedCommand = analysis.NormalizedCommand
	record.DisplayCommand = analysis.NormalizedCommand
	record.Tokens = analysis.Tokens
	record.Tool = analysis.Tool
	record.Subcommand = analysis.Subcommand
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

func HasSuspiciousControl(command string) bool {
	for _, r := range command {
		if r == '\n' || r == '\r' || r == '\t' {
			continue
		}
		if r < 0x20 || r == 0x7f {
			return true
		}
	}
	return false
}

func appendSensitivityFlags(existing []model.SensitivityFlag, additions ...model.SensitivityFlag) []model.SensitivityFlag {
	for _, addition := range additions {
		seen := false
		for _, value := range existing {
			if value == addition {
				seen = true
				break
			}
		}
		if !seen {
			existing = append(existing, addition)
		}
	}
	return existing
}

func appendRiskFlags(existing []model.RiskFlag, additions ...model.RiskFlag) []model.RiskFlag {
	for _, addition := range additions {
		seen := false
		for _, value := range existing {
			if value == addition {
				seen = true
				break
			}
		}
		if !seen {
			existing = append(existing, addition)
		}
	}
	return existing
}

func appendExclusionReasons(existing []model.ExclusionReason, additions ...model.ExclusionReason) []model.ExclusionReason {
	for _, addition := range additions {
		seen := false
		for _, value := range existing {
			if value == addition {
				seen = true
				break
			}
		}
		if !seen {
			existing = append(existing, addition)
		}
	}
	return existing
}
