package analyze

import (
	"sort"
	"time"

	"cmd-mint/internal/model"
)

const maxCommandsPerToolSection = 10

const (
	RiskToleranceBalanced     = "balanced"
	RiskToleranceConservative = "conservative"
)

type ReportOptions struct {
	GeneratedAt     time.Time
	MinFrequency    int
	MaxAliases      int
	IgnoredTools    []string
	RiskTolerance   string
	ExistingAliases map[string]model.ExistingAliasDefinition
	Warnings        []model.Warning
}

func BuildReport(records []model.CommandRecord, sources []model.SourceSummary, options ReportOptions) model.Report {
	aggregate := AggregateRecords(records, sources, AggregateOptions{
		MinFrequency:  options.MinFrequency,
		IgnoredTools:  options.IgnoredTools,
		RiskTolerance: options.RiskTolerance,
	})
	aliases := GenerateAliasSuggestions(aggregate, AliasOptions{
		MinFrequency:    options.MinFrequency,
		MaxAliases:      options.MaxAliases,
		ExistingAliases: options.ExistingAliases,
	})

	report := model.NewReport(options.GeneratedAt)
	report.Sources = aggregate.Sources
	report.TopTools = aggregate.Summary.TopTools
	report.AliasSuggestions = aliases.Suggestions
	report.ToolSections = buildToolSections(aggregate, options.MinFrequency)
	report.Patterns = aggregate.Patterns
	report.Exclusions = mergeExclusions(aggregate.Exclusions, aliases.Exclusions)
	report.Warnings = append(append([]model.Warning(nil), aggregate.Warnings...), options.Warnings...)
	report.Summary = aggregate.Summary
	report.Summary.AliasConflictsSkipped = aliases.Exclusions.ByReason[model.ExclusionAliasConflict]

	return report
}

func buildToolSections(result AggregateResult, minFrequency int) []model.ToolSection {
	if minFrequency < 1 {
		minFrequency = 1
	}

	commandsByTool := make(map[string][]model.SafeCommandSummary)
	for _, command := range result.Commands {
		if command.Count < minFrequency {
			continue
		}
		tool := command.Tool
		if tool == "" {
			tool = "Other tools"
		}
		commandsByTool[tool] = append(commandsByTool[tool], model.SafeCommandSummary{
			Command:           command.DisplayCommand,
			NormalizedCommand: command.NormalizedCommand,
			Count:             command.Count,
			Tool:              command.Tool,
			Subcommand:        command.Subcommand,
			SourceShells:      append([]model.Shell(nil), command.SourceShells...),
			RiskFlags:         append([]model.RiskFlag(nil), command.RiskFlags...),
			ExclusionReasons:  append([]model.ExclusionReason(nil), command.ExclusionReasons...),
		})
	}

	sections := make([]model.ToolSection, 0, len(result.Tools))
	for _, tool := range result.Tools {
		name := tool.Tool
		if name == "" {
			name = "Other tools"
		}
		commands := commandsByTool[name]
		sortSafeCommands(commands)
		if len(commands) > maxCommandsPerToolSection {
			commands = commands[:maxCommandsPerToolSection]
		}

		sections = append(sections, model.ToolSection{
			Tool:        name,
			Count:       tool.Count,
			Commands:    commands,
			Subcommands: sortedSubcommandSummaries(tool.SubcommandCounts),
		})
	}

	return sections
}

func mergeExclusions(aggregate model.ExclusionSummary, aliases model.ExclusionSummary) model.ExclusionSummary {
	merged := aggregate
	merged.ByReason = cloneReasonCounts(aggregate.ByReason)
	merged.RiskyCategories = cloneRiskCounts(aggregate.RiskyCategories)
	merged.SensitiveCategories = cloneSensitivityCounts(aggregate.SensitiveCategories)
	merged.ExistingAliasConflicts = append([]string(nil), aliases.ExistingAliasConflicts...)
	sort.Strings(merged.ExistingAliasConflicts)

	addReasonCount(&merged, model.ExclusionAliasConflict, aliases.ByReason[model.ExclusionAliasConflict])
	addReasonCount(&merged, model.ExclusionLowSavings, aliases.ByReason[model.ExclusionLowSavings])
	addReasonCount(&merged, model.ExclusionTooShort, aliases.ByReason[model.ExclusionTooShort])
	merged.TooShortCommandCount += aliases.TooShortCommandCount

	return merged
}

func addReasonCount(exclusions *model.ExclusionSummary, reason model.ExclusionReason, count int) {
	if count == 0 {
		return
	}
	if exclusions.ByReason == nil {
		exclusions.ByReason = make(map[model.ExclusionReason]int)
	}
	exclusions.ByReason[reason] += count
}

func sortSafeCommands(commands []model.SafeCommandSummary) {
	sort.SliceStable(commands, func(i, j int) bool {
		if commands[i].Count != commands[j].Count {
			return commands[i].Count > commands[j].Count
		}
		left := commands[i].NormalizedCommand
		right := commands[j].NormalizedCommand
		if left == "" {
			left = commands[i].Command
		}
		if right == "" {
			right = commands[j].Command
		}
		return left < right
	})
}

func sortedSubcommandSummaries(counts map[string]int) []model.SubcommandSummary {
	summaries := make([]model.SubcommandSummary, 0, len(counts))
	for subcommand, count := range counts {
		if subcommand == "" || count == 0 {
			continue
		}
		summaries = append(summaries, model.SubcommandSummary{
			Subcommand: subcommand,
			Count:      count,
		})
	}
	sort.SliceStable(summaries, func(i, j int) bool {
		if summaries[i].Count != summaries[j].Count {
			return summaries[i].Count > summaries[j].Count
		}
		return summaries[i].Subcommand < summaries[j].Subcommand
	})
	return summaries
}
