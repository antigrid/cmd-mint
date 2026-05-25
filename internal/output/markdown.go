package output

import (
	"bytes"
	"fmt"
	"sort"
	"strings"

	"cmd-mint/internal/model"
)

const generatedAtFormat = "2006-01-02 15:04:05"

// RenderCheatsheetMarkdown renders the human-readable report from safe report
// data only. Sensitive skipped commands are represented only by aggregate counts.
func RenderCheatsheetMarkdown(report model.Report) []byte {
	report = safeReport(report)
	var buffer bytes.Buffer

	fmt.Fprintln(&buffer, "# Shell History Cheat Sheet")
	fmt.Fprintln(&buffer)
	fmt.Fprintf(&buffer, "Generated locally by cmd-mint on %s.\n", report.GeneratedAt.Format(generatedAtFormat))
	fmt.Fprintln(&buffer)
	fmt.Fprintln(&buffer, "> Review before sharing or committing. This report was generated from local shell history and may reveal personal workflows.")
	fmt.Fprintln(&buffer)

	renderSummary(&buffer, report)
	renderSources(&buffer, report.Sources)
	renderAliases(&buffer, report.AliasSuggestions)
	renderTools(&buffer, report)
	renderPatterns(&buffer, report.Patterns)
	renderExclusions(&buffer, report)
	renderPrivacy(&buffer, report)

	return buffer.Bytes()
}

func renderSummary(buffer *bytes.Buffer, report model.Report) {
	fmt.Fprintln(buffer, "## Summary")
	fmt.Fprintln(buffer)
	fmt.Fprintf(buffer, "- Total sources scanned: %d\n", report.Summary.TotalSourcesScanned)
	fmt.Fprintf(buffer, "- Non-sensitive commands analyzed: %d\n", report.Summary.SafeCommandsAnalyzed)
	fmt.Fprintf(buffer, "- Sensitive-looking commands skipped: %d\n", report.Summary.SensitiveCommandsSkipped)
	fmt.Fprintf(buffer, "- Risky commands excluded from alias suggestions: %d\n", report.Summary.RiskyCommandsExcluded)
	fmt.Fprintf(buffer, "- Alias conflicts skipped: %d\n", report.Summary.AliasConflictsSkipped)

	topTools := sortedToolSummaries(report.Summary.TopTools)
	if len(topTools) == 0 {
		topTools = sortedToolSummaries(report.TopTools)
	}
	if len(topTools) == 0 {
		fmt.Fprintln(buffer, "- Top tools: none")
		fmt.Fprintln(buffer)
		return
	}

	fmt.Fprintln(buffer, "- Top tools:")
	for _, tool := range topTools {
		fmt.Fprintf(buffer, "  - %s: %d commands\n", codeSpan(tool.Tool), tool.Count)
	}
	fmt.Fprintln(buffer)
}

func renderSources(buffer *bytes.Buffer, sources []model.SourceSummary) {
	fmt.Fprintln(buffer, "## Sources scanned")
	fmt.Fprintln(buffer)

	sources = sortedSources(sources)
	if len(sources) == 0 {
		fmt.Fprintln(buffer, "No sources were recorded in the safe report data.")
		fmt.Fprintln(buffer)
		return
	}

	for _, source := range sources {
		fmt.Fprintf(buffer, "- %s (%s)\n", codeSpan(source.SourceFile), source.SourceShell)
		fmt.Fprintf(buffer, "  - Entries read: %d\n", source.EntriesRead)
		fmt.Fprintf(buffer, "  - Entries parsed: %d\n", source.EntriesParsed)
		fmt.Fprintf(buffer, "  - Entries skipped: %d\n", source.EntriesSkipped)
		if len(source.Warnings) == 0 {
			fmt.Fprintln(buffer, "  - Warnings: none")
			continue
		}
		fmt.Fprintln(buffer, "  - Warnings:")
		for _, warning := range source.Warnings {
			fmt.Fprintf(buffer, "    - %s\n", warning)
		}
	}
	fmt.Fprintln(buffer)
}

func renderAliases(buffer *bytes.Buffer, suggestions []model.AliasSuggestion) {
	fmt.Fprintln(buffer, "## Alias suggestions")
	fmt.Fprintln(buffer)

	suggestions = sortedAliasSuggestions(suggestions)
	if len(suggestions) == 0 {
		fmt.Fprintln(buffer, "No safe alias suggestions were found.")
		fmt.Fprintln(buffer)
		return
	}

	for _, suggestion := range suggestions {
		fmt.Fprintf(buffer, "### %s\n", codeSpan(suggestion.Name))
		fmt.Fprintln(buffer)
		fmt.Fprintf(buffer, "- Expands to: %s\n", codeSpan(suggestion.Command))
		fmt.Fprintf(buffer, "- Frequency: %d\n", suggestion.Frequency)
		fmt.Fprintf(buffer, "- Estimated characters saved per use: %d\n", suggestion.EstimatedSavedPerUse)
		fmt.Fprintf(buffer, "- Estimated total characters saved: %d\n", suggestion.EstimatedSavedTotal)
		fmt.Fprintf(buffer, "- Confidence: %s\n", suggestion.Confidence)
		if suggestion.Reason == "" {
			fmt.Fprintln(buffer, "- Reason: not recorded")
		} else {
			fmt.Fprintf(buffer, "- Reason: %s\n", suggestion.Reason)
		}
		fmt.Fprintf(buffer, "- Source shells: %s\n", shellList(suggestion.SourceShells))
		fmt.Fprintln(buffer)
	}
}

func renderTools(buffer *bytes.Buffer, report model.Report) {
	fmt.Fprintln(buffer, "## Commands by tool")
	fmt.Fprintln(buffer)

	sections := sortedToolSections(report.ToolSections, report.AliasSuggestions)
	if len(sections) == 0 {
		fmt.Fprintln(buffer, "No frequent observed non-sensitive commands were recorded by tool.")
		fmt.Fprintln(buffer)
		return
	}

	for _, section := range sections {
		tool := section.Tool
		if tool == "" {
			tool = "Other tools"
		}
		fmt.Fprintf(buffer, "### %s\n", tool)
		fmt.Fprintln(buffer)
		fmt.Fprintf(buffer, "- Observed non-sensitive commands for tool: %d\n", section.Count)

		commands := sortedCommands(section.Commands)
		if len(commands) == 0 {
			fmt.Fprintln(buffer, "- Frequent observed exact commands: none recorded")
		} else {
			fmt.Fprintln(buffer, "- Frequent observed exact commands:")
			for _, command := range commands {
				normalized := command.NormalizedCommand
				if normalized == "" {
					normalized = command.Command
				}
				fmt.Fprintf(buffer, "  - %s: seen %dx; representative normalized command: %s", codeSpan(command.Command), command.Count, codeSpan(normalized))
				if len(command.SourceShells) > 0 {
					fmt.Fprintf(buffer, "; source shells: %s", shellList(command.SourceShells))
				}
				if commandSummaryIsRisky(command) {
					fmt.Fprint(buffer, "; label: risky observed command, excluded from alias suggestions")
				}
				fmt.Fprintln(buffer)
			}
		}

		subcommands := sortedSubcommands(section.Subcommands)
		if len(subcommands) == 0 {
			fmt.Fprintln(buffer, "- Subcommand groupings: none recorded")
		} else {
			fmt.Fprintln(buffer, "- Subcommand groupings:")
			for _, subcommand := range subcommands {
				fmt.Fprintf(buffer, "  - %s: %d commands\n", codeSpan(subcommand.Subcommand), subcommand.Count)
			}
		}
		fmt.Fprintln(buffer)
	}
}

func renderPatterns(buffer *bytes.Buffer, patterns []model.PatternSummary) {
	fmt.Fprintln(buffer, "## Frequent command patterns")
	fmt.Fprintln(buffer)
	fmt.Fprintln(buffer, "Patterns are shown for review only and are not emitted as aliases in the MVP.")
	fmt.Fprintln(buffer)

	patterns = sortedPatterns(patterns)
	if len(patterns) == 0 {
		fmt.Fprintln(buffer, "No frequent safe command patterns were recorded.")
		fmt.Fprintln(buffer)
		return
	}

	for _, pattern := range patterns {
		fmt.Fprintf(buffer, "- %s: seen %dx\n", codeSpan(pattern.Pattern), pattern.Count)
		examples := sortedStrings(pattern.Examples)
		if len(examples) == 0 {
			continue
		}
		fmt.Fprintln(buffer, "  - Safe examples:")
		for _, example := range examples {
			fmt.Fprintf(buffer, "    - %s\n", codeSpan(example))
		}
	}
	fmt.Fprintln(buffer)
}

func renderExclusions(buffer *bytes.Buffer, report model.Report) {
	exclusions := report.Exclusions

	fmt.Fprintln(buffer, "## Excluded from alias suggestions")
	fmt.Fprintln(buffer)
	fmt.Fprintf(buffer, "- Existing alias conflicts skipped: %d\n", report.Summary.AliasConflictsSkipped)
	if len(exclusions.ExistingAliasConflicts) > 0 {
		fmt.Fprintf(buffer, "- Existing alias conflict names: %s\n", codeSpanList(sortedStrings(exclusions.ExistingAliasConflicts)))
	}
	fmt.Fprintf(buffer, "- Multiline command count: %d\n", exclusions.MultilineCommandCount)
	fmt.Fprintf(buffer, "- Low-frequency candidate count: %d\n", exclusions.LowFrequencyCommandCount)
	fmt.Fprintf(buffer, "- Too-short candidate count: %d\n", exclusions.TooShortCommandCount)
	fmt.Fprintf(buffer, "- Malformed command count: %d\n", exclusions.MalformedCommandCount)

	risky := sortedRiskCounts(exclusions.RiskyCategories)
	if len(risky) == 0 {
		fmt.Fprintln(buffer, "- Risky command categories: none")
	} else {
		fmt.Fprintln(buffer, "- Risky command categories:")
		for _, count := range risky {
			fmt.Fprintf(buffer, "  - %s: %d\n", codeSpan(string(count.key)), count.count)
		}
	}

	reasons := sortedReasonCounts(exclusions.ByReason)
	if len(reasons) == 0 {
		fmt.Fprintln(buffer, "- Exclusion reasons: none")
	} else {
		fmt.Fprintln(buffer, "- Exclusion reasons:")
		for _, count := range reasons {
			fmt.Fprintf(buffer, "  - %s: %d\n", codeSpan(string(count.key)), count.count)
		}
	}
	fmt.Fprintln(buffer)
}

func renderPrivacy(buffer *bytes.Buffer, report model.Report) {
	exclusions := report.Exclusions

	fmt.Fprintln(buffer, "## Privacy and safety summary")
	fmt.Fprintln(buffer)
	fmt.Fprintln(buffer, "- Generated locally from safe report data.")
	fmt.Fprintln(buffer, "- Raw sensitive skipped commands are not included in this Markdown report.")
	fmt.Fprintf(buffer, "- Sensitive-looking commands skipped: %d\n", report.Summary.SensitiveCommandsSkipped)
	fmt.Fprintf(buffer, "- Risky commands excluded from alias suggestions: %d\n", report.Summary.RiskyCommandsExcluded)
	fmt.Fprintf(buffer, "- Multiline commands excluded from alias suggestions: %d\n", exclusions.MultilineCommandCount)

	sensitive := sortedSensitivityCounts(exclusions.SensitiveCategories)
	if len(sensitive) == 0 {
		fmt.Fprintln(buffer, "- Sensitive categories: none")
	} else {
		fmt.Fprintln(buffer, "- Sensitive categories:")
		for _, count := range sensitive {
			fmt.Fprintf(buffer, "  - %s: %d\n", codeSpan(string(count.key)), count.count)
		}
	}
}

func commandSummaryIsRisky(command model.SafeCommandSummary) bool {
	if len(command.RiskFlags) > 0 {
		return true
	}
	for _, reason := range command.ExclusionReasons {
		if reason == model.ExclusionRiskyDestructive || reason == model.ExclusionRiskyProductionAction {
			return true
		}
	}
	return false
}

func sortedSources(sources []model.SourceSummary) []model.SourceSummary {
	sorted := append([]model.SourceSummary(nil), sources...)
	sort.SliceStable(sorted, func(i, j int) bool {
		if sorted[i].SourceFile != sorted[j].SourceFile {
			return sorted[i].SourceFile < sorted[j].SourceFile
		}
		return sorted[i].SourceShell < sorted[j].SourceShell
	})
	return sorted
}

func sortedToolSummaries(tools []model.ToolSummary) []model.ToolSummary {
	sorted := append([]model.ToolSummary(nil), tools...)
	sort.SliceStable(sorted, func(i, j int) bool {
		if sorted[i].Count != sorted[j].Count {
			return sorted[i].Count > sorted[j].Count
		}
		return sorted[i].Tool < sorted[j].Tool
	})
	return sorted
}

func sortedAliasSuggestions(suggestions []model.AliasSuggestion) []model.AliasSuggestion {
	sorted := append([]model.AliasSuggestion(nil), suggestions...)
	sort.SliceStable(sorted, func(i, j int) bool {
		left := sorted[i]
		right := sorted[j]
		switch {
		case left.Frequency != right.Frequency:
			return left.Frequency > right.Frequency
		case left.EstimatedSavedTotal != right.EstimatedSavedTotal:
			return left.EstimatedSavedTotal > right.EstimatedSavedTotal
		case left.EstimatedSavedPerUse != right.EstimatedSavedPerUse:
			return left.EstimatedSavedPerUse > right.EstimatedSavedPerUse
		case left.Tool != right.Tool:
			return left.Tool < right.Tool
		case left.Command != right.Command:
			return left.Command < right.Command
		default:
			return left.Name < right.Name
		}
	})
	return sorted
}

func sortedToolSections(sections []model.ToolSection, suggestions []model.AliasSuggestion) []model.ToolSection {
	aliasCounts := make(map[string]int)
	for _, suggestion := range suggestions {
		tool := suggestion.Tool
		if tool == "" {
			tool = "Other tools"
		}
		aliasCounts[tool]++
	}

	sorted := append([]model.ToolSection(nil), sections...)
	sort.SliceStable(sorted, func(i, j int) bool {
		leftTool := sorted[i].Tool
		rightTool := sorted[j].Tool
		if leftTool == "" {
			leftTool = "Other tools"
		}
		if rightTool == "" {
			rightTool = "Other tools"
		}
		if sorted[i].Count != sorted[j].Count {
			return sorted[i].Count > sorted[j].Count
		}
		if aliasCounts[leftTool] != aliasCounts[rightTool] {
			return aliasCounts[leftTool] > aliasCounts[rightTool]
		}
		return leftTool < rightTool
	})
	return sorted
}

func sortedCommands(commands []model.SafeCommandSummary) []model.SafeCommandSummary {
	sorted := append([]model.SafeCommandSummary(nil), commands...)
	sort.SliceStable(sorted, func(i, j int) bool {
		if sorted[i].Count != sorted[j].Count {
			return sorted[i].Count > sorted[j].Count
		}
		left := sorted[i].NormalizedCommand
		right := sorted[j].NormalizedCommand
		if left == "" {
			left = sorted[i].Command
		}
		if right == "" {
			right = sorted[j].Command
		}
		return left < right
	})
	return sorted
}

func sortedSubcommands(subcommands []model.SubcommandSummary) []model.SubcommandSummary {
	sorted := append([]model.SubcommandSummary(nil), subcommands...)
	sort.SliceStable(sorted, func(i, j int) bool {
		if sorted[i].Count != sorted[j].Count {
			return sorted[i].Count > sorted[j].Count
		}
		return sorted[i].Subcommand < sorted[j].Subcommand
	})
	return sorted
}

func sortedPatterns(patterns []model.PatternSummary) []model.PatternSummary {
	sorted := append([]model.PatternSummary(nil), patterns...)
	sort.SliceStable(sorted, func(i, j int) bool {
		if sorted[i].Count != sorted[j].Count {
			return sorted[i].Count > sorted[j].Count
		}
		return sorted[i].Pattern < sorted[j].Pattern
	})
	return sorted
}

type reasonCount struct {
	key   model.ExclusionReason
	count int
}

func sortedReasonCounts(values map[model.ExclusionReason]int) []reasonCount {
	counts := make([]reasonCount, 0, len(values))
	for key, count := range values {
		if count == 0 {
			continue
		}
		counts = append(counts, reasonCount{key: key, count: count})
	}
	sort.SliceStable(counts, func(i, j int) bool {
		return counts[i].key < counts[j].key
	})
	return counts
}

type riskCount struct {
	key   model.RiskFlag
	count int
}

func sortedRiskCounts(values map[model.RiskFlag]int) []riskCount {
	counts := make([]riskCount, 0, len(values))
	for key, count := range values {
		if count == 0 {
			continue
		}
		counts = append(counts, riskCount{key: key, count: count})
	}
	sort.SliceStable(counts, func(i, j int) bool {
		return counts[i].key < counts[j].key
	})
	return counts
}

type sensitivityCount struct {
	key   model.SensitivityFlag
	count int
}

func sortedSensitivityCounts(values map[model.SensitivityFlag]int) []sensitivityCount {
	counts := make([]sensitivityCount, 0, len(values))
	for key, count := range values {
		if count == 0 {
			continue
		}
		counts = append(counts, sensitivityCount{key: key, count: count})
	}
	sort.SliceStable(counts, func(i, j int) bool {
		return counts[i].key < counts[j].key
	})
	return counts
}

func sortedStrings(values []string) []string {
	sorted := append([]string(nil), values...)
	sort.Strings(sorted)
	return sorted
}

func shellList(shells []model.Shell) string {
	if len(shells) == 0 {
		return "not recorded"
	}
	sorted := append([]model.Shell(nil), shells...)
	sort.SliceStable(sorted, func(i, j int) bool {
		return sorted[i] < sorted[j]
	})
	parts := make([]string, 0, len(sorted))
	for _, shell := range sorted {
		if shell == "" {
			continue
		}
		parts = append(parts, string(shell))
	}
	if len(parts) == 0 {
		return "not recorded"
	}
	return strings.Join(parts, ", ")
}

func codeSpanList(values []string) string {
	if len(values) == 0 {
		return "none"
	}
	parts := make([]string, 0, len(values))
	for _, value := range values {
		parts = append(parts, codeSpan(value))
	}
	return strings.Join(parts, ", ")
}

func codeSpan(value string) string {
	if value == "" {
		return "`<empty>`"
	}
	longestRun := 0
	currentRun := 0
	for _, r := range value {
		if r == '`' {
			currentRun++
			if currentRun > longestRun {
				longestRun = currentRun
			}
			continue
		}
		currentRun = 0
	}
	delimiter := strings.Repeat("`", longestRun+1)
	if strings.HasPrefix(value, "`") || strings.HasSuffix(value, "`") || strings.HasPrefix(value, " ") || strings.HasSuffix(value, " ") {
		return delimiter + " " + value + " " + delimiter
	}
	return delimiter + value + delimiter
}
