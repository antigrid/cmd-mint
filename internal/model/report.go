package model

import "time"

const ReportSchemaVersion = "1.0"

type Report struct {
	SchemaVersion    string            `json:"schema_version"`
	GeneratedAt      time.Time         `json:"generated_at"`
	Summary          AggregateSummary  `json:"summary"`
	Sources          []SourceSummary   `json:"sources,omitempty"`
	TopTools         []ToolSummary     `json:"top_tools,omitempty"`
	AliasSuggestions []AliasSuggestion `json:"alias_suggestions,omitempty"`
	ToolSections     []ToolSection     `json:"tool_sections,omitempty"`
	Patterns         []PatternSummary  `json:"patterns,omitempty"`
	Exclusions       ExclusionSummary  `json:"exclusions"`
	Warnings         []Warning         `json:"warnings,omitempty"`
}

func NewReport(generatedAt time.Time) Report {
	return Report{
		SchemaVersion: ReportSchemaVersion,
		GeneratedAt:   generatedAt,
	}
}

type AggregateSummary struct {
	TotalSourcesScanned      int           `json:"total_sources_scanned"`
	SafeCommandsAnalyzed     int           `json:"safe_commands_analyzed"`
	SensitiveCommandsSkipped int           `json:"sensitive_commands_skipped"`
	RiskyCommandsExcluded    int           `json:"risky_commands_excluded"`
	AliasConflictsSkipped    int           `json:"alias_conflicts_skipped"`
	TotalEntriesRead         int           `json:"total_entries_read"`
	TotalEntriesParsed       int           `json:"total_entries_parsed"`
	TotalEntriesSkipped      int           `json:"total_entries_skipped"`
	TopTools                 []ToolSummary `json:"top_tools,omitempty"`
}

type ToolSummary struct {
	Tool  string `json:"tool"`
	Count int    `json:"count"`
}

type ToolSection struct {
	Tool        string               `json:"tool"`
	Count       int                  `json:"count"`
	Commands    []SafeCommandSummary `json:"commands,omitempty"`
	Subcommands []SubcommandSummary  `json:"subcommands,omitempty"`
}

type SafeCommandSummary struct {
	Command           string            `json:"command"`
	NormalizedCommand string            `json:"normalized_command,omitempty"`
	Count             int               `json:"count"`
	Tool              string            `json:"tool,omitempty"`
	Subcommand        string            `json:"subcommand,omitempty"`
	SourceShells      []Shell           `json:"source_shells,omitempty"`
	RiskFlags         []RiskFlag        `json:"risk_flags,omitempty"`
	ExclusionReasons  []ExclusionReason `json:"exclusion_reasons,omitempty"`
}

func SafeCommandFromRecord(record CommandRecord, count int, sourceShells []Shell) (SafeCommandSummary, bool) {
	if recordHasSensitiveExclusion(record) {
		return SafeCommandSummary{}, false
	}
	if record.DisplayCommand == "" {
		return SafeCommandSummary{}, false
	}

	return SafeCommandSummary{
		Command:           record.DisplayCommand,
		NormalizedCommand: record.NormalizedCommand,
		Count:             count,
		Tool:              record.Tool,
		Subcommand:        record.Subcommand,
		SourceShells:      append([]Shell(nil), sourceShells...),
		RiskFlags:         append([]RiskFlag(nil), record.RiskFlags...),
		ExclusionReasons:  append([]ExclusionReason(nil), record.ExclusionReasons...),
	}, true
}

func recordHasSensitiveExclusion(record CommandRecord) bool {
	if len(record.SensitivityFlags) > 0 {
		return true
	}

	for _, reason := range record.ExclusionReasons {
		switch reason {
		case ExclusionSensitiveSecret,
			ExclusionSensitiveCredentialFile,
			ExclusionSensitiveDatabaseURL,
			ExclusionSensitiveAuthHeader,
			ExclusionSensitivePrivateKey,
			ExclusionSensitiveClipboardOrKeychain:
			return true
		}
	}
	return false
}

type SubcommandSummary struct {
	Subcommand string `json:"subcommand"`
	Count      int    `json:"count"`
}

type PatternSummary struct {
	Pattern  string   `json:"pattern"`
	Count    int      `json:"count"`
	Examples []string `json:"examples,omitempty"`
}

type ExclusionSummary struct {
	ByReason                 map[ExclusionReason]int `json:"by_reason,omitempty"`
	ExistingAliasConflicts   []string                `json:"existing_alias_conflicts,omitempty"`
	RiskyCategories          map[RiskFlag]int        `json:"risky_categories,omitempty"`
	SensitiveCategories      map[SensitivityFlag]int `json:"sensitive_categories,omitempty"`
	MultilineCommandCount    int                     `json:"multiline_command_count,omitempty"`
	LowFrequencyCommandCount int                     `json:"low_frequency_command_count,omitempty"`
	TooShortCommandCount     int                     `json:"too_short_command_count,omitempty"`
	MalformedCommandCount    int                     `json:"malformed_command_count,omitempty"`
}

type Warning struct {
	Code       string `json:"code,omitempty"`
	Message    string `json:"message"`
	SourceFile string `json:"source_file,omitempty"`
}
