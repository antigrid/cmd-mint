package analyze

import (
	"sort"

	"cmd-mint/internal/model"
)

type AliasRankInput struct {
	Score                int
	Frequency            int
	EstimatedSavedTotal  int
	EstimatedSavedPerUse int
	Tool                 string
	NormalizedCommand    string
	AliasName            string
}

func SortedCommandAggregates(values map[string]*CommandAggregate) []CommandAggregate {
	commands := make([]CommandAggregate, 0, len(values))
	for _, value := range values {
		if value == nil {
			continue
		}
		commands = append(commands, *cloneCommandAggregate(value))
	}
	SortCommandAggregates(commands)
	return commands
}

func SortCommandAggregates(commands []CommandAggregate) {
	sort.SliceStable(commands, func(i, j int) bool {
		if commands[i].Count != commands[j].Count {
			return commands[i].Count > commands[j].Count
		}
		return commands[i].NormalizedCommand < commands[j].NormalizedCommand
	})
	for i := range commands {
		sortShells(commands[i].SourceShells)
		sort.Strings(commands[i].SourceFiles)
		sortRiskFlags(commands[i].RiskFlags)
		sortExclusionReasons(commands[i].ExclusionReasons)
	}
}

func SortedToolAggregates(values map[string]*ToolAggregate) []ToolAggregate {
	tools := make([]ToolAggregate, 0, len(values))
	for _, value := range values {
		if value == nil {
			continue
		}
		tools = append(tools, *cloneToolAggregate(value))
	}
	SortToolAggregates(tools)
	return tools
}

func SortToolAggregates(tools []ToolAggregate) {
	sort.SliceStable(tools, func(i, j int) bool {
		if tools[i].Count != tools[j].Count {
			return tools[i].Count > tools[j].Count
		}
		if tools[i].AliasSuggestionCount != tools[j].AliasSuggestionCount {
			return tools[i].AliasSuggestionCount > tools[j].AliasSuggestionCount
		}
		return tools[i].Tool < tools[j].Tool
	})
}

func SortAliasRankInputs(inputs []AliasRankInput) {
	sort.SliceStable(inputs, func(i, j int) bool {
		left := inputs[i]
		right := inputs[j]
		switch {
		case left.Score != right.Score:
			return left.Score > right.Score
		case left.Frequency != right.Frequency:
			return left.Frequency > right.Frequency
		case left.EstimatedSavedTotal != right.EstimatedSavedTotal:
			return left.EstimatedSavedTotal > right.EstimatedSavedTotal
		case left.EstimatedSavedPerUse != right.EstimatedSavedPerUse:
			return left.EstimatedSavedPerUse > right.EstimatedSavedPerUse
		case left.Tool != right.Tool:
			return left.Tool < right.Tool
		case left.NormalizedCommand != right.NormalizedCommand:
			return left.NormalizedCommand < right.NormalizedCommand
		default:
			return left.AliasName < right.AliasName
		}
	})
}

func ToolSummariesFromAggregates(tools []ToolAggregate) []model.ToolSummary {
	summaries := make([]model.ToolSummary, 0, len(tools))
	for _, tool := range tools {
		summaries = append(summaries, model.ToolSummary{
			Tool:  tool.Tool,
			Count: tool.Count,
		})
	}
	return summaries
}

func SafeCommandSummariesFromAggregates(commands []CommandAggregate) []model.SafeCommandSummary {
	summaries := make([]model.SafeCommandSummary, 0, len(commands))
	for _, command := range commands {
		summaries = append(summaries, model.SafeCommandSummary{
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
	return summaries
}

func sortShells(values []model.Shell) {
	sort.Slice(values, func(i, j int) bool {
		return values[i] < values[j]
	})
}

func sortRiskFlags(values []model.RiskFlag) {
	sort.Slice(values, func(i, j int) bool {
		return values[i] < values[j]
	})
}

func sortExclusionReasons(values []model.ExclusionReason) {
	sort.Slice(values, func(i, j int) bool {
		return values[i] < values[j]
	})
}
