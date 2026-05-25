package output

import (
	"strings"

	"cmd-mint/internal/analyze"
	"cmd-mint/internal/model"
)

// safeReport returns a copy of report with every rendered command string passed
// through containsSensitiveOutputText a second time. Sensitive records are
// already dropped during aggregation, so this is defense in depth: it is the
// single gate shared by every artifact (cheatsheet.md, report.json, and the
// terminal summary) so that a classification miss upstream cannot leak a raw
// sensitive command into any output. Risky-but-non-sensitive commands are
// intentionally preserved here; they are filtered/labeled elsewhere.
func safeReport(report model.Report) model.Report {
	safe := report
	safe.Sources = safeSourceSummaries(report.Sources)
	safe.TopTools = append([]model.ToolSummary(nil), report.TopTools...)
	if len(safe.TopTools) == 0 {
		safe.TopTools = append([]model.ToolSummary(nil), report.Summary.TopTools...)
	}
	safe.AliasSuggestions = safeAliasSuggestions(report.AliasSuggestions)
	safe.ToolSections = safeToolSections(report.ToolSections)
	safe.Patterns = safePatterns(report.Patterns)
	safe.Warnings = safeWarnings(report.Warnings)
	return safe
}

func safeSourceSummaries(sources []model.SourceSummary) []model.SourceSummary {
	safe := make([]model.SourceSummary, 0, len(sources))
	for _, source := range sources {
		source.Warnings = safeWarningMessages(source.Warnings)
		safe = append(safe, source)
	}
	return safe
}

func safeAliasSuggestions(suggestions []model.AliasSuggestion) []model.AliasSuggestion {
	safe := make([]model.AliasSuggestion, 0, len(suggestions))
	for _, suggestion := range suggestions {
		if containsSensitiveOutputText(suggestion.Command) || strings.ContainsAny(suggestion.Command, "\r\n") {
			continue
		}
		suggestion.SourceShells = append([]model.Shell(nil), suggestion.SourceShells...)
		suggestion.ExclusionReasons = append([]model.ExclusionReason(nil), suggestion.ExclusionReasons...)
		safe = append(safe, suggestion)
	}
	return safe
}

func safeToolSections(sections []model.ToolSection) []model.ToolSection {
	safe := make([]model.ToolSection, 0, len(sections))
	for _, section := range sections {
		section.Commands = safeCommands(section.Commands)
		section.Subcommands = append([]model.SubcommandSummary(nil), section.Subcommands...)
		safe = append(safe, section)
	}
	return safe
}

func safeCommands(commands []model.SafeCommandSummary) []model.SafeCommandSummary {
	safe := make([]model.SafeCommandSummary, 0, len(commands))
	for _, command := range commands {
		if containsSensitiveOutputText(command.Command) || containsSensitiveOutputText(command.NormalizedCommand) {
			continue
		}
		command.SourceShells = append([]model.Shell(nil), command.SourceShells...)
		command.RiskFlags = append([]model.RiskFlag(nil), command.RiskFlags...)
		command.ExclusionReasons = append([]model.ExclusionReason(nil), command.ExclusionReasons...)
		safe = append(safe, command)
	}
	return safe
}

func safePatterns(patterns []model.PatternSummary) []model.PatternSummary {
	safe := make([]model.PatternSummary, 0, len(patterns))
	for _, pattern := range patterns {
		if containsSensitiveOutputText(pattern.Pattern) {
			continue
		}
		pattern.Examples = safePatternExamples(pattern.Examples)
		safe = append(safe, pattern)
	}
	return safe
}

func safePatternExamples(examples []string) []string {
	safe := make([]string, 0, len(examples))
	for _, example := range examples {
		if containsSensitiveOutputText(example) {
			continue
		}
		safe = append(safe, example)
	}
	return safe
}

func safeWarnings(warnings []model.Warning) []model.Warning {
	safe := make([]model.Warning, 0, len(warnings))
	for _, warning := range warnings {
		if containsSensitiveOutputText(warning.Message) {
			continue
		}
		safe = append(safe, warning)
	}
	return safe
}

func safeWarningMessages(warnings []string) []string {
	safe := make([]string, 0, len(warnings))
	for _, warning := range warnings {
		if containsSensitiveOutputText(warning) {
			continue
		}
		safe = append(safe, warning)
	}
	return safe
}

func containsSensitiveOutputText(value string) bool {
	lower := strings.ToLower(value)
	if lower == "" {
		return false
	}

	if analyze.ClassifyRawSensitivity(value).Sensitive() {
		return true
	}
	analysis := analyze.AnalyzeCommand(value)
	if analyze.ClassifyNormalizedSensitivity(analysis.NormalizedCommand, analysis.Tokens).Sensitive() {
		return true
	}

	sensitiveMarkers := []string{
		"authorization:",
		"bearer ",
		"api_key",
		"apikey",
		"aws_secret_access_key",
		"database_url",
		"private key",
		"postgres://",
		"postgresql://",
		"mysql://",
		"mongodb://",
		"redis://",
		"password=",
		"passwd=",
		"secret=",
		"token=",
	}
	for _, marker := range sensitiveMarkers {
		if strings.Contains(lower, marker) {
			return true
		}
	}
	return false
}
