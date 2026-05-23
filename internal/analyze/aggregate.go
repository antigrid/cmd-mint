package analyze

import (
	"sort"

	"cmd-mint/internal/model"
)

type AggregateOptions struct {
	MinFrequency int
}

type AggregateBuilder struct {
	minFrequency int
	commands     map[string]*CommandAggregate
	tools        map[string]*ToolAggregate
	sources      []model.SourceSummary
	summary      model.AggregateSummary
	exclusions   model.ExclusionSummary
	warnings     []model.Warning
}

type CommandAggregate struct {
	NormalizedCommand string
	DisplayCommand    string
	Tool              string
	Subcommand        string
	Count             int
	SourceShells      []model.Shell
	SourceFiles       []string
	RiskFlags         []model.RiskFlag
	ExclusionReasons  []model.ExclusionReason
	IsMultiline       bool
	AliasExcluded     bool
}

type ToolAggregate struct {
	Tool                 string
	Count                int
	AliasSuggestionCount int
	SubcommandCounts     map[string]int
}

type AggregateResult struct {
	CommandAggregates map[string]*CommandAggregate
	ToolAggregates    map[string]*ToolAggregate
	Commands          []CommandAggregate
	Tools             []ToolAggregate
	Patterns          []model.PatternSummary
	Sources           []model.SourceSummary
	Summary           model.AggregateSummary
	Exclusions        model.ExclusionSummary
	Warnings          []model.Warning
}

func NewAggregateBuilder(options AggregateOptions) *AggregateBuilder {
	minFrequency := options.MinFrequency
	if minFrequency < 1 {
		minFrequency = 1
	}

	return &AggregateBuilder{
		minFrequency: minFrequency,
		commands:     make(map[string]*CommandAggregate),
		tools:        make(map[string]*ToolAggregate),
		exclusions: model.ExclusionSummary{
			ByReason:            make(map[model.ExclusionReason]int),
			RiskyCategories:     make(map[model.RiskFlag]int),
			SensitiveCategories: make(map[model.SensitivityFlag]int),
		},
	}
}

func AggregateRecords(records []model.CommandRecord, sources []model.SourceSummary, options AggregateOptions) AggregateResult {
	builder := NewAggregateBuilder(options)
	for _, source := range sources {
		builder.AddSourceSummary(source)
	}
	for _, record := range records {
		builder.AddRecord(record)
	}
	return builder.Result()
}

func (builder *AggregateBuilder) AddSourceSummary(source model.SourceSummary) {
	source.Warnings = append([]string(nil), source.Warnings...)
	builder.sources = append(builder.sources, source)
	builder.summary.TotalSourcesScanned = len(builder.sources)
	builder.summary.TotalEntriesRead += source.EntriesRead
	builder.summary.TotalEntriesParsed += source.EntriesParsed
	builder.summary.TotalEntriesSkipped += source.EntriesSkipped

	for _, warning := range source.Warnings {
		builder.warnings = append(builder.warnings, model.Warning{
			Message:    warning,
			SourceFile: source.SourceFile,
		})
	}
}

func (builder *AggregateBuilder) AddRecord(record model.CommandRecord) {
	if record.ParseStatus == "" {
		record.ParseStatus = model.ParseStatusParsed
	}
	if record.ParseStatus != model.ParseStatusParsed {
		builder.countUnparsedRecord(record)
		return
	}
	if shouldAnalyzeRecord(record) {
		record = AnalyzeRecord(record)
	}

	if IsSensitiveRecord(record) {
		builder.summary.SensitiveCommandsSkipped++
		builder.countSensitivity(record)
		return
	}

	if HasSuspiciousControl(record.RawCommand) {
		builder.exclusions.MalformedCommandCount++
		builder.countReason(model.ExclusionMalformedEntry)
		return
	}

	if record.NormalizedCommand == "" || record.DisplayCommand == "" {
		builder.countReason(model.ExclusionParseFailed)
		return
	}

	builder.summary.SafeCommandsAnalyzed++
	builder.countAliasExclusions(record)
	builder.addSafeCommand(record)
}

func (builder *AggregateBuilder) Result() AggregateResult {
	exclusions := cloneExclusionSummary(builder.exclusions)
	lowFrequencyCount := 0
	for _, command := range builder.commands {
		if command.Count < builder.minFrequency {
			lowFrequencyCount++
		}
	}
	if lowFrequencyCount > 0 {
		exclusions.LowFrequencyCommandCount = lowFrequencyCount
		if exclusions.ByReason == nil {
			exclusions.ByReason = make(map[model.ExclusionReason]int)
		}
		exclusions.ByReason[model.ExclusionLowFrequency] = lowFrequencyCount
	}

	commands := SortedCommandAggregates(builder.commands)
	tools := SortedToolAggregates(builder.tools)
	toolSummaries := ToolSummariesFromAggregates(tools)
	patterns := GeneratePatternsFromCommands(commands)

	summary := builder.summary
	summary.TopTools = toolSummaries

	return AggregateResult{
		CommandAggregates: cloneCommandAggregateMap(builder.commands),
		ToolAggregates:    cloneToolAggregateMap(builder.tools),
		Commands:          commands,
		Tools:             tools,
		Patterns:          patterns,
		Sources:           cloneSourceSummaries(builder.sources),
		Summary:           summary,
		Exclusions:        exclusions,
		Warnings:          cloneWarnings(builder.warnings),
	}
}

func (builder *AggregateBuilder) countUnparsedRecord(record model.CommandRecord) {
	switch record.ParseStatus {
	case model.ParseStatusMalformedEntry:
		builder.exclusions.MalformedCommandCount++
		builder.countReason(model.ExclusionMalformedEntry)
	case model.ParseStatusFailed:
		builder.countReason(model.ExclusionParseFailed)
	case model.ParseStatusUnsupportedShell:
		builder.countReason(model.ExclusionUnsupportedShell)
	case model.ParseStatusUnreadableSource:
		builder.countReason(model.ExclusionUnreadableSource)
	default:
		builder.countReason(model.ExclusionParseFailed)
	}
}

func (builder *AggregateBuilder) countSensitivity(record model.CommandRecord) {
	for _, flag := range record.SensitivityFlags {
		builder.exclusions.SensitiveCategories[flag]++
	}

	countedReason := false
	for _, reason := range record.ExclusionReasons {
		if isSensitiveExclusion(reason) {
			builder.countReason(reason)
			countedReason = true
		}
	}
	if countedReason {
		return
	}

	for _, flag := range record.SensitivityFlags {
		if reason, ok := sensitivityReason(flag); ok {
			builder.countReason(reason)
		}
	}
}

func (builder *AggregateBuilder) countAliasExclusions(record model.CommandRecord) {
	if record.IsMultiline {
		builder.exclusions.MultilineCommandCount++
		builder.countReason(model.ExclusionMultiline)
	}

	if IsRiskyRecord(record) {
		builder.summary.RiskyCommandsExcluded++
		for _, flag := range record.RiskFlags {
			builder.exclusions.RiskyCategories[flag]++
		}
		for _, reason := range record.ExclusionReasons {
			if reason == model.ExclusionRiskyDestructive || reason == model.ExclusionRiskyProductionAction {
				builder.countReason(reason)
			}
		}
	}
}

func (builder *AggregateBuilder) addSafeCommand(record model.CommandRecord) {
	aggregate := builder.commands[record.NormalizedCommand]
	if aggregate == nil {
		aggregate = &CommandAggregate{
			NormalizedCommand: record.NormalizedCommand,
			DisplayCommand:    record.DisplayCommand,
			Tool:              record.Tool,
			Subcommand:        record.Subcommand,
		}
		builder.commands[record.NormalizedCommand] = aggregate
	}

	aggregate.Count++
	aggregate.SourceShells = appendUniqueShell(aggregate.SourceShells, record.SourceShell)
	aggregate.SourceFiles = appendUniqueString(aggregate.SourceFiles, record.SourceFile)
	aggregate.RiskFlags = appendUniqueRiskFlags(aggregate.RiskFlags, record.RiskFlags...)
	aggregate.ExclusionReasons = appendUniqueExclusionReasons(aggregate.ExclusionReasons, record.ExclusionReasons...)
	aggregate.IsMultiline = aggregate.IsMultiline || record.IsMultiline
	aggregate.AliasExcluded = aggregate.AliasExcluded || record.IsMultiline || IsRiskyRecord(record)

	toolName := record.Tool
	if toolName == "" {
		toolName = "Other tools"
	}
	tool := builder.tools[toolName]
	if tool == nil {
		tool = &ToolAggregate{
			Tool:             toolName,
			SubcommandCounts: make(map[string]int),
		}
		builder.tools[toolName] = tool
	}
	tool.Count++
	if record.Subcommand != "" {
		tool.SubcommandCounts[record.Subcommand]++
	}
}

func (builder *AggregateBuilder) countReason(reason model.ExclusionReason) {
	if builder.exclusions.ByReason == nil {
		builder.exclusions.ByReason = make(map[model.ExclusionReason]int)
	}
	builder.exclusions.ByReason[reason]++
}

func shouldAnalyzeRecord(record model.CommandRecord) bool {
	return record.RawCommand != "" &&
		record.NormalizedCommand == "" &&
		record.DisplayCommand == "" &&
		len(record.SensitivityFlags) == 0 &&
		len(record.ExclusionReasons) == 0
}

func sensitivityReason(flag model.SensitivityFlag) (model.ExclusionReason, bool) {
	switch flag {
	case model.SensitivitySecret:
		return model.ExclusionSensitiveSecret, true
	case model.SensitivityCredentialFile:
		return model.ExclusionSensitiveCredentialFile, true
	case model.SensitivityDatabaseURL:
		return model.ExclusionSensitiveDatabaseURL, true
	case model.SensitivityAuthHeader:
		return model.ExclusionSensitiveAuthHeader, true
	case model.SensitivityPrivateKey:
		return model.ExclusionSensitivePrivateKey, true
	case model.SensitivityClipboardOrKeychain:
		return model.ExclusionSensitiveClipboardOrKeychain, true
	default:
		return "", false
	}
}

func appendUniqueShell(values []model.Shell, additions ...model.Shell) []model.Shell {
	for _, addition := range additions {
		if addition == "" {
			continue
		}
		seen := false
		for _, value := range values {
			if value == addition {
				seen = true
				break
			}
		}
		if !seen {
			values = append(values, addition)
		}
	}
	return values
}

func appendUniqueString(values []string, additions ...string) []string {
	for _, addition := range additions {
		if addition == "" {
			continue
		}
		seen := false
		for _, value := range values {
			if value == addition {
				seen = true
				break
			}
		}
		if !seen {
			values = append(values, addition)
		}
	}
	return values
}

func appendUniqueRiskFlags(values []model.RiskFlag, additions ...model.RiskFlag) []model.RiskFlag {
	for _, addition := range additions {
		seen := false
		for _, value := range values {
			if value == addition {
				seen = true
				break
			}
		}
		if !seen {
			values = append(values, addition)
		}
	}
	return values
}

func appendUniqueExclusionReasons(values []model.ExclusionReason, additions ...model.ExclusionReason) []model.ExclusionReason {
	for _, addition := range additions {
		seen := false
		for _, value := range values {
			if value == addition {
				seen = true
				break
			}
		}
		if !seen {
			values = append(values, addition)
		}
	}
	return values
}

func cloneCommandAggregateMap(values map[string]*CommandAggregate) map[string]*CommandAggregate {
	cloned := make(map[string]*CommandAggregate, len(values))
	for key, value := range values {
		cloned[key] = cloneCommandAggregate(value)
	}
	return cloned
}

func cloneToolAggregateMap(values map[string]*ToolAggregate) map[string]*ToolAggregate {
	cloned := make(map[string]*ToolAggregate, len(values))
	for key, value := range values {
		cloned[key] = cloneToolAggregate(value)
	}
	return cloned
}

func cloneCommandAggregate(value *CommandAggregate) *CommandAggregate {
	if value == nil {
		return nil
	}
	cloned := *value
	cloned.SourceShells = append([]model.Shell(nil), value.SourceShells...)
	cloned.SourceFiles = append([]string(nil), value.SourceFiles...)
	cloned.RiskFlags = append([]model.RiskFlag(nil), value.RiskFlags...)
	cloned.ExclusionReasons = append([]model.ExclusionReason(nil), value.ExclusionReasons...)
	sortShells(cloned.SourceShells)
	sort.Strings(cloned.SourceFiles)
	sortRiskFlags(cloned.RiskFlags)
	sortExclusionReasons(cloned.ExclusionReasons)
	return &cloned
}

func cloneToolAggregate(value *ToolAggregate) *ToolAggregate {
	if value == nil {
		return nil
	}
	cloned := *value
	cloned.SubcommandCounts = make(map[string]int, len(value.SubcommandCounts))
	for key, count := range value.SubcommandCounts {
		cloned.SubcommandCounts[key] = count
	}
	return &cloned
}

func cloneSourceSummaries(values []model.SourceSummary) []model.SourceSummary {
	cloned := make([]model.SourceSummary, len(values))
	for i, value := range values {
		cloned[i] = value
		cloned[i].Warnings = append([]string(nil), value.Warnings...)
	}
	return cloned
}

func cloneWarnings(values []model.Warning) []model.Warning {
	return append([]model.Warning(nil), values...)
}

func cloneExclusionSummary(value model.ExclusionSummary) model.ExclusionSummary {
	cloned := value
	cloned.ByReason = cloneReasonCounts(value.ByReason)
	cloned.RiskyCategories = cloneRiskCounts(value.RiskyCategories)
	cloned.SensitiveCategories = cloneSensitivityCounts(value.SensitiveCategories)
	cloned.ExistingAliasConflicts = append([]string(nil), value.ExistingAliasConflicts...)
	return cloned
}

func cloneReasonCounts(values map[model.ExclusionReason]int) map[model.ExclusionReason]int {
	if len(values) == 0 {
		return nil
	}
	cloned := make(map[model.ExclusionReason]int, len(values))
	for key, count := range values {
		cloned[key] = count
	}
	return cloned
}

func cloneRiskCounts(values map[model.RiskFlag]int) map[model.RiskFlag]int {
	if len(values) == 0 {
		return nil
	}
	cloned := make(map[model.RiskFlag]int, len(values))
	for key, count := range values {
		cloned[key] = count
	}
	return cloned
}

func cloneSensitivityCounts(values map[model.SensitivityFlag]int) map[model.SensitivityFlag]int {
	if len(values) == 0 {
		return nil
	}
	cloned := make(map[model.SensitivityFlag]int, len(values))
	for key, count := range values {
		cloned[key] = count
	}
	return cloned
}
