package model

import (
	"encoding/json"
	"reflect"
	"strings"
	"testing"
	"time"
)

const fakeRawCommand = `curl -H "Authorization: Bearer raw-secret-token" https://example.invalid`

func TestCommandRecordJSONDoesNotSerializeRawCommand(t *testing.T) {
	record := CommandRecord{
		SourceShell:       ShellZsh,
		SourceFile:        "/home/alex/.zsh_history",
		EntryIndex:        42,
		RawCommand:        fakeRawCommand,
		NormalizedCommand: "git status",
		DisplayCommand:    "git status",
		Tokens:            []string{"git", "status"},
		Tool:              "git",
		Subcommand:        "status",
		ParseStatus:       ParseStatusParsed,
	}

	data, err := json.Marshal(record)
	if err != nil {
		t.Fatalf("Marshal(CommandRecord) error = %v", err)
	}

	assertJSONOmitsRawCommand(t, string(data))
}

func TestSafeReportJSONDoesNotSerializeRawCommand(t *testing.T) {
	record := CommandRecord{
		RawCommand:        fakeRawCommand,
		NormalizedCommand: "git status",
		DisplayCommand:    "git status",
		Tool:              "git",
		Subcommand:        "status",
	}
	report := NewReport(time.Date(2026, 6, 1, 14, 30, 22, 0, time.UTC))
	report.Summary = AggregateSummary{
		TotalSourcesScanned:  1,
		SafeCommandsAnalyzed: 1,
		TopTools:             []ToolSummary{{Tool: "git", Count: 1}},
	}
	report.Sources = []SourceSummary{{
		SourceShell:   ShellZsh,
		SourceFile:    "/home/alex/.zsh_history",
		EntriesRead:   1,
		EntriesParsed: 1,
	}}
	report.TopTools = []ToolSummary{{Tool: "git", Count: 1}}
	report.AliasSuggestions = []AliasSuggestion{{
		Name:                 "gs",
		Command:              "git status",
		Tool:                 "git",
		Frequency:            3,
		EstimatedSavedPerUse: 8,
		EstimatedSavedTotal:  24,
		Confidence:           ConfidenceHigh,
		Reason:               "frequent exact command",
		SourceShells:         []Shell{ShellZsh},
	}}
	safeCommand, ok := SafeCommandFromRecord(record, 1, []Shell{ShellZsh})
	if !ok {
		t.Fatal("SafeCommandFromRecord returned false for record with display command")
	}
	report.ToolSections = []ToolSection{{
		Tool:     "git",
		Count:    1,
		Commands: []SafeCommandSummary{safeCommand},
	}}
	report.Patterns = []PatternSummary{{
		Pattern: "git <verb>",
		Count:   1,
		Examples: []string{
			"git status",
		},
	}}
	report.Exclusions = ExclusionSummary{
		ByReason: map[ExclusionReason]int{
			ExclusionSensitiveAuthHeader: 1,
		},
	}

	data, err := json.Marshal(report)
	if err != nil {
		t.Fatalf("Marshal(Report) error = %v", err)
	}

	assertJSONOmitsRawCommand(t, string(data))
}

func TestSafeCommandFromRecordRejectsSensitiveRecord(t *testing.T) {
	record := CommandRecord{
		RawCommand:        fakeRawCommand,
		NormalizedCommand: `curl -H "Authorization: Bearer raw-secret-token" https://example.invalid`,
		DisplayCommand:    `curl -H "Authorization: Bearer raw-secret-token" https://example.invalid`,
		SensitivityFlags:  []SensitivityFlag{SensitivityAuthHeader},
		ExclusionReasons:  []ExclusionReason{ExclusionSensitiveAuthHeader},
	}

	if safeCommand, ok := SafeCommandFromRecord(record, 1, []Shell{ShellZsh}); ok {
		t.Fatalf("SafeCommandFromRecord() = %#v, true; want false for sensitive record", safeCommand)
	}
}

func TestSafeReportStructsDoNotContainRawCommandField(t *testing.T) {
	types := []reflect.Type{
		reflect.TypeOf(Report{}),
		reflect.TypeOf(AggregateSummary{}),
		reflect.TypeOf(SourceSummary{}),
		reflect.TypeOf(AliasSuggestion{}),
		reflect.TypeOf(ToolSummary{}),
		reflect.TypeOf(ToolSection{}),
		reflect.TypeOf(SafeCommandSummary{}),
		reflect.TypeOf(SubcommandSummary{}),
		reflect.TypeOf(PatternSummary{}),
		reflect.TypeOf(ExclusionSummary{}),
		reflect.TypeOf(Warning{}),
	}

	for _, typ := range types {
		if _, ok := typ.FieldByName("RawCommand"); ok {
			t.Fatalf("%s has RawCommand field", typ.Name())
		}
	}
}

func TestMVPExclusionReasonsIncludeSpecReasons(t *testing.T) {
	want := []ExclusionReason{
		"sensitive_secret",
		"sensitive_credential_file",
		"sensitive_database_url",
		"sensitive_auth_header",
		"sensitive_private_key",
		"sensitive_clipboard_or_keychain",
		"risky_destructive",
		"risky_production_action",
		"multiline",
		"parse_failed",
		"too_short",
		"low_frequency",
		"low_savings",
		"alias_conflict",
		"unsupported_shell",
		"unreadable_source",
		"malformed_entry",
	}

	got := MVPExclusionReasons()
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("MVPExclusionReasons() = %#v, want %#v", got, want)
	}
}

func assertJSONOmitsRawCommand(t *testing.T, encoded string) {
	t.Helper()

	for _, forbidden := range []string{
		fakeRawCommand,
		"raw-secret-token",
		"RawCommand",
		"raw_command",
	} {
		if strings.Contains(encoded, forbidden) {
			t.Fatalf("marshaled JSON contains %q: %s", forbidden, encoded)
		}
	}
}
