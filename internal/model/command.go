package model

import "time"

type Shell string

const (
	ShellAuto Shell = "auto"
	ShellBash Shell = "bash"
	ShellZsh  Shell = "zsh"
	ShellFish Shell = "fish"
)

type ParseStatus string

const (
	ParseStatusParsed           ParseStatus = "parsed"
	ParseStatusSkipped          ParseStatus = "skipped"
	ParseStatusFailed           ParseStatus = "parse_failed"
	ParseStatusUnsupportedShell ParseStatus = "unsupported_shell"
	ParseStatusUnreadableSource ParseStatus = "unreadable_source"
	ParseStatusMalformedEntry   ParseStatus = "malformed_entry"
)

type SensitivityFlag string

const (
	SensitivitySecret              SensitivityFlag = "secret"
	SensitivityCredentialFile      SensitivityFlag = "credential_file"
	SensitivityDatabaseURL         SensitivityFlag = "database_url"
	SensitivityAuthHeader          SensitivityFlag = "auth_header"
	SensitivityPrivateKey          SensitivityFlag = "private_key"
	SensitivityClipboardOrKeychain SensitivityFlag = "clipboard_or_keychain"
)

type RiskFlag string

const (
	RiskDestructive      RiskFlag = "destructive"
	RiskProductionAction RiskFlag = "production_action"
)

type Confidence string

const (
	ConfidenceHigh   Confidence = "high"
	ConfidenceMedium Confidence = "medium"
	ConfidenceLow    Confidence = "low"
)

type CommandRecord struct {
	SourceShell       Shell             `json:"source_shell,omitempty"`
	SourceFile        string            `json:"source_file,omitempty"`
	EntryIndex        int               `json:"entry_index,omitempty"`
	Timestamp         *time.Time        `json:"timestamp,omitempty"`
	RawCommand        string            `json:"-"`
	NormalizedCommand string            `json:"normalized_command,omitempty"`
	DisplayCommand    string            `json:"display_command,omitempty"`
	Tokens            []string          `json:"tokens,omitempty"`
	Tool              string            `json:"tool,omitempty"`
	Subcommand        string            `json:"subcommand,omitempty"`
	IsMultiline       bool              `json:"is_multiline,omitempty"`
	ParseStatus       ParseStatus       `json:"parse_status,omitempty"`
	SensitivityFlags  []SensitivityFlag `json:"sensitivity_flags,omitempty"`
	RiskFlags         []RiskFlag        `json:"risk_flags,omitempty"`
	ExclusionReasons  []ExclusionReason `json:"exclusion_reasons,omitempty"`
}

type SourceSummary struct {
	SourceShell    Shell    `json:"source_shell"`
	SourceFile     string   `json:"source_file"`
	EntriesRead    int      `json:"entries_read"`
	EntriesParsed  int      `json:"entries_parsed"`
	EntriesSkipped int      `json:"entries_skipped"`
	Warnings       []string `json:"warnings,omitempty"`
}

type AliasSuggestion struct {
	Name                 string            `json:"name"`
	Command              string            `json:"command"`
	Tool                 string            `json:"tool,omitempty"`
	Frequency            int               `json:"frequency"`
	EstimatedSavedPerUse int               `json:"estimated_saved_per_use"`
	EstimatedSavedTotal  int               `json:"estimated_saved_total"`
	Confidence           Confidence        `json:"confidence"`
	Reason               string            `json:"reason,omitempty"`
	SourceShells         []Shell           `json:"source_shells,omitempty"`
	ExclusionReasons     []ExclusionReason `json:"exclusion_reasons,omitempty"`
}
