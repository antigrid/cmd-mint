package output

import (
	"bytes"
	"encoding/json"
	"strings"

	"cmd-mint/internal/model"
)

func RenderReportJSON(report model.Report) ([]byte, error) {
	safe := safeJSONReport(report)
	if safe.SchemaVersion == "" {
		safe.SchemaVersion = model.ReportSchemaVersion
	}

	var buffer bytes.Buffer
	encoder := json.NewEncoder(&buffer)
	encoder.SetEscapeHTML(false)
	encoder.SetIndent("", "  ")
	if err := encoder.Encode(safe); err != nil {
		return nil, err
	}
	return buffer.Bytes(), nil
}

func safeJSONReport(report model.Report) model.Report {
	safe := report
	safe.Sources = safeSourceSummaries(report.Sources)
	safe.TopTools = append([]model.ToolSummary(nil), report.TopTools...)
	if len(safe.TopTools) == 0 {
		safe.TopTools = append([]model.ToolSummary(nil), report.Summary.TopTools...)
	}
	safe.AliasSuggestions = safeAliasSuggestionsForJSON(report.AliasSuggestions)
	safe.ToolSections = safeToolSectionsForJSON(report.ToolSections)
	safe.Patterns = safePatternsForJSON(report.Patterns)
	safe.Warnings = safeWarningsForJSON(report.Warnings)
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

func safeAliasSuggestionsForJSON(suggestions []model.AliasSuggestion) []model.AliasSuggestion {
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

func safeToolSectionsForJSON(sections []model.ToolSection) []model.ToolSection {
	safe := make([]model.ToolSection, 0, len(sections))
	for _, section := range sections {
		section.Commands = safeCommandsForJSON(section.Commands)
		section.Subcommands = append([]model.SubcommandSummary(nil), section.Subcommands...)
		safe = append(safe, section)
	}
	return safe
}

func safeCommandsForJSON(commands []model.SafeCommandSummary) []model.SafeCommandSummary {
	safe := make([]model.SafeCommandSummary, 0, len(commands))
	for _, command := range commands {
		if containsSensitiveOutputText(command.Command) || containsSensitiveOutputText(command.NormalizedCommand) {
			continue
		}
		command.SourceShells = append([]model.Shell(nil), command.SourceShells...)
		safe = append(safe, command)
	}
	return safe
}

func safePatternsForJSON(patterns []model.PatternSummary) []model.PatternSummary {
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

func safeWarningsForJSON(warnings []model.Warning) []model.Warning {
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

	sensitiveMarkers := []string{
		"authorization:",
		"bearer ",
		"raw-secret-token",
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
