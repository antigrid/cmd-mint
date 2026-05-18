package analyze

import (
	"reflect"
	"strings"
	"testing"

	"cmd-mint/internal/model"
)

func TestAggregateRecordsMergesDuplicateNormalizedCommandsAcrossShells(t *testing.T) {
	result := AggregateRecords([]model.CommandRecord{
		rawRecord(model.ShellZsh, "/home/alex/.zsh_history", " git   status "),
		rawRecord(model.ShellBash, "/home/alex/.bash_history", "git status"),
		rawRecord(model.ShellFish, "/home/alex/.local/share/fish/fish_history", "docker ps"),
	}, nil, AggregateOptions{MinFrequency: 2})

	command := result.CommandAggregates["git status"]
	if command == nil {
		t.Fatal("missing aggregate for git status")
	}
	if command.Count != 2 {
		t.Fatalf("git status count = %d, want 2", command.Count)
	}

	gotShells := result.Commands[indexCommand(t, result.Commands, "git status")].SourceShells
	wantShells := []model.Shell{model.ShellBash, model.ShellZsh}
	if !reflect.DeepEqual(gotShells, wantShells) {
		t.Fatalf("source shells = %#v, want %#v", gotShells, wantShells)
	}
	if result.Summary.SafeCommandsAnalyzed != 3 {
		t.Fatalf("SafeCommandsAnalyzed = %d, want 3", result.Summary.SafeCommandsAnalyzed)
	}
	if result.ToolAggregates["git"].Count != 2 {
		t.Fatalf("git tool count = %d, want 2", result.ToolAggregates["git"].Count)
	}
}

func TestAggregateRecordsExcludesSensitiveCommandsFromSafeAggregates(t *testing.T) {
	result := AggregateRecords([]model.CommandRecord{
		rawRecord(model.ShellZsh, "/home/alex/.zsh_history", "git status"),
		rawRecord(model.ShellZsh, "/home/alex/.zsh_history", "curl -H 'Authorization: Bearer fake-token' https://example.invalid"),
	}, nil, AggregateOptions{})

	if _, ok := result.CommandAggregates["git status"]; !ok {
		t.Fatal("safe command missing from aggregate")
	}
	for command := range result.CommandAggregates {
		if strings.Contains(command, "fake-token") || strings.Contains(command, "Authorization") {
			t.Fatalf("sensitive command entered safe aggregates: %q", command)
		}
	}
	if result.Summary.SafeCommandsAnalyzed != 1 {
		t.Fatalf("SafeCommandsAnalyzed = %d, want 1", result.Summary.SafeCommandsAnalyzed)
	}
	if result.Summary.SensitiveCommandsSkipped != 1 {
		t.Fatalf("SensitiveCommandsSkipped = %d, want 1", result.Summary.SensitiveCommandsSkipped)
	}
	if got := result.Exclusions.ByReason[model.ExclusionSensitiveAuthHeader]; got != 1 {
		t.Fatalf("sensitive auth header count = %d, want 1", got)
	}
}

func TestAggregateRecordsKeepsRiskyNonSensitiveCommandsAndCountsAliasExclusions(t *testing.T) {
	const command = "kubectl delete pod old-worker -n staging"

	result := AggregateRecords([]model.CommandRecord{
		rawRecord(model.ShellBash, "/home/alex/.bash_history", command),
	}, nil, AggregateOptions{})

	aggregate := result.CommandAggregates[command]
	if aggregate == nil {
		t.Fatal("risky non-sensitive command missing from safe aggregate")
	}
	if !aggregate.AliasExcluded {
		t.Fatal("risky aggregate AliasExcluded = false, want true")
	}
	if result.Summary.SafeCommandsAnalyzed != 1 {
		t.Fatalf("SafeCommandsAnalyzed = %d, want 1", result.Summary.SafeCommandsAnalyzed)
	}
	if result.Summary.RiskyCommandsExcluded != 1 {
		t.Fatalf("RiskyCommandsExcluded = %d, want 1", result.Summary.RiskyCommandsExcluded)
	}
	if got := result.Exclusions.ByReason[model.ExclusionRiskyDestructive]; got != 1 {
		t.Fatalf("risky destructive count = %d, want 1", got)
	}
	if got := result.Exclusions.RiskyCategories[model.RiskDestructive]; got != 1 {
		t.Fatalf("risky category count = %d, want 1", got)
	}
}

func TestAggregateRecordsTreatsSuspiciousControlCharactersAsMalformed(t *testing.T) {
	result := AggregateRecords([]model.CommandRecord{
		rawRecord(model.ShellBash, "/home/alex/.bash_history", "git status\x00 --short"),
		rawRecord(model.ShellBash, "/home/alex/.bash_history", "git status"),
	}, nil, AggregateOptions{})

	if _, ok := result.CommandAggregates["git status\x00 --short"]; ok {
		t.Fatal("control-character command entered safe aggregates")
	}
	if result.Summary.SafeCommandsAnalyzed != 1 {
		t.Fatalf("SafeCommandsAnalyzed = %d, want 1", result.Summary.SafeCommandsAnalyzed)
	}
	if result.Exclusions.MalformedCommandCount != 1 {
		t.Fatalf("MalformedCommandCount = %d, want 1", result.Exclusions.MalformedCommandCount)
	}
	if got := result.Exclusions.ByReason[model.ExclusionMalformedEntry]; got != 1 {
		t.Fatalf("malformed count = %d, want 1", got)
	}
}

func TestAggregateRecordsTracksSourceWarningsMalformedAndLowFrequencyCounts(t *testing.T) {
	source := model.SourceSummary{
		SourceShell:    model.ShellZsh,
		SourceFile:     "/home/alex/.zsh_history",
		EntriesRead:    3,
		EntriesParsed:  2,
		EntriesSkipped: 1,
		Warnings:       []string{"malformed zsh extended history entries skipped"},
	}

	result := AggregateRecords([]model.CommandRecord{
		rawRecord(model.ShellZsh, source.SourceFile, "git status"),
		{SourceShell: model.ShellZsh, SourceFile: source.SourceFile, ParseStatus: model.ParseStatusMalformedEntry},
	}, []model.SourceSummary{source}, AggregateOptions{MinFrequency: 2})

	if result.Summary.TotalSourcesScanned != 1 {
		t.Fatalf("TotalSourcesScanned = %d, want 1", result.Summary.TotalSourcesScanned)
	}
	if result.Summary.TotalEntriesSkipped != 1 {
		t.Fatalf("TotalEntriesSkipped = %d, want 1", result.Summary.TotalEntriesSkipped)
	}
	if result.Exclusions.MalformedCommandCount != 1 {
		t.Fatalf("MalformedCommandCount = %d, want 1", result.Exclusions.MalformedCommandCount)
	}
	if got := result.Exclusions.ByReason[model.ExclusionMalformedEntry]; got != 1 {
		t.Fatalf("malformed count = %d, want 1", got)
	}
	if result.Exclusions.LowFrequencyCommandCount != 1 {
		t.Fatalf("LowFrequencyCommandCount = %d, want 1", result.Exclusions.LowFrequencyCommandCount)
	}
	if len(result.Warnings) != 1 || result.Warnings[0].SourceFile != source.SourceFile {
		t.Fatalf("warnings = %#v, want source warning", result.Warnings)
	}
}

func TestAggregateSortingIsDeterministicForTies(t *testing.T) {
	records := []model.CommandRecord{
		rawRecord(model.ShellZsh, "zsh", "npm test"),
		rawRecord(model.ShellZsh, "zsh", "git status"),
		rawRecord(model.ShellZsh, "zsh", "docker ps"),
	}

	var wantCommands []string
	var wantTools []string
	for i := 0; i < 20; i++ {
		result := AggregateRecords(records, nil, AggregateOptions{})
		gotCommands := commandNames(result.Commands)
		gotTools := toolNames(result.Tools)
		if i == 0 {
			wantCommands = gotCommands
			wantTools = gotTools
			continue
		}
		if !reflect.DeepEqual(gotCommands, wantCommands) {
			t.Fatalf("command order run %d = %#v, want %#v", i, gotCommands, wantCommands)
		}
		if !reflect.DeepEqual(gotTools, wantTools) {
			t.Fatalf("tool order run %d = %#v, want %#v", i, gotTools, wantTools)
		}
	}

	if !reflect.DeepEqual(wantCommands, []string{"docker ps", "git status", "npm test"}) {
		t.Fatalf("command tie order = %#v", wantCommands)
	}
	if !reflect.DeepEqual(wantTools, []string{"docker", "git", "npm"}) {
		t.Fatalf("tool tie order = %#v", wantTools)
	}
}

func TestSortAliasRankInputsUsesSpecTieBreakers(t *testing.T) {
	inputs := []AliasRankInput{
		{Score: 10, Frequency: 2, EstimatedSavedTotal: 20, EstimatedSavedPerUse: 10, Tool: "git", NormalizedCommand: "git status", AliasName: "gs"},
		{Score: 20, Frequency: 1, EstimatedSavedTotal: 10, EstimatedSavedPerUse: 10, Tool: "npm", NormalizedCommand: "npm test", AliasName: "nt"},
		{Score: 10, Frequency: 3, EstimatedSavedTotal: 10, EstimatedSavedPerUse: 10, Tool: "docker", NormalizedCommand: "docker ps", AliasName: "dps"},
		{Score: 10, Frequency: 2, EstimatedSavedTotal: 30, EstimatedSavedPerUse: 5, Tool: "go", NormalizedCommand: "go test ./...", AliasName: "gt"},
		{Score: 10, Frequency: 2, EstimatedSavedTotal: 20, EstimatedSavedPerUse: 10, Tool: "git", NormalizedCommand: "git branch", AliasName: "gb"},
	}

	SortAliasRankInputs(inputs)

	got := make([]string, len(inputs))
	for i, input := range inputs {
		got[i] = input.AliasName
	}
	want := []string{"nt", "dps", "gt", "gb", "gs"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("alias rank order = %#v, want %#v", got, want)
	}
}

func TestAggregateTypesDoNotRetainRawCommandFields(t *testing.T) {
	for _, typ := range []reflect.Type{
		reflect.TypeOf(CommandAggregate{}),
		reflect.TypeOf(ToolAggregate{}),
		reflect.TypeOf(AggregateResult{}),
	} {
		if _, ok := typ.FieldByName("RawCommand"); ok {
			t.Fatalf("%s has RawCommand field", typ.Name())
		}
	}
}

func rawRecord(shell model.Shell, sourceFile string, raw string) model.CommandRecord {
	return model.CommandRecord{
		SourceShell: shell,
		SourceFile:  sourceFile,
		RawCommand:  raw,
		ParseStatus: model.ParseStatusParsed,
	}
}

func indexCommand(t *testing.T, commands []CommandAggregate, normalized string) int {
	t.Helper()
	for i, command := range commands {
		if command.NormalizedCommand == normalized {
			return i
		}
	}
	t.Fatalf("missing command %q in %#v", normalized, commands)
	return -1
}

func commandNames(commands []CommandAggregate) []string {
	names := make([]string, len(commands))
	for i, command := range commands {
		names[i] = command.NormalizedCommand
	}
	return names
}

func toolNames(tools []ToolAggregate) []string {
	names := make([]string, len(tools))
	for i, tool := range tools {
		names[i] = tool.Tool
	}
	return names
}
