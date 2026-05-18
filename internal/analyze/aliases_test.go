package analyze

import (
	"reflect"
	"testing"

	"cmd-mint/internal/model"
)

func TestGenerateAliasSuggestionsSelectsConventionalAliases(t *testing.T) {
	result := aggregateRawCommands("git status", "git status", "git status")

	aliases := GenerateAliasSuggestions(result, AliasOptions{MinFrequency: 3, MaxAliases: 25})

	suggestion := requireAlias(t, aliases.Suggestions, "gs")
	if suggestion.Command != "git status" {
		t.Fatalf("gs command = %q, want git status", suggestion.Command)
	}
	if suggestion.Confidence != model.ConfidenceHigh {
		t.Fatalf("gs confidence = %q, want high", suggestion.Confidence)
	}
	if !suggestion.AliasFileEligible {
		t.Fatal("gs AliasFileEligible = false, want true")
	}
	if suggestion.EstimatedSavedPerUse != 8 || suggestion.EstimatedSavedTotal != 24 {
		t.Fatalf("gs savings = %d/%d, want 8/24", suggestion.EstimatedSavedPerUse, suggestion.EstimatedSavedTotal)
	}
}

func TestGenerateAliasSuggestionsBuildsFallbackAliasesDeterministically(t *testing.T) {
	result := aggregateRawCommands(
		"npm run typecheck",
		"npm run typecheck",
		"npm run typecheck",
		"docker compose logs -f",
		"docker compose logs -f",
		"docker compose logs -f",
		"kubectl get pods -A",
		"kubectl get pods -A",
		"kubectl get pods -A",
	)

	aliases := GenerateAliasSuggestions(result, AliasOptions{MinFrequency: 3, MaxAliases: 25})

	requireAlias(t, aliases.Suggestions, "nrtypecheck")
	requireAlias(t, aliases.Suggestions, "dclogsf")
	requireAlias(t, aliases.Suggestions, "kgpa")
}

func TestGenerateAliasSuggestionsSkipsExistingAliasConflict(t *testing.T) {
	result := aggregateRawCommands("git status", "git status", "git status")
	existing := map[string]model.ExistingAliasDefinition{
		"gs": {Name: "gs", SourceShell: model.ShellZsh, SourceFile: "/home/alex/.zshrc", Kind: model.ExistingAliasKindShellAlias},
	}

	aliases := GenerateAliasSuggestions(result, AliasOptions{MinFrequency: 3, MaxAliases: 25, ExistingAliases: existing})

	if hasAlias(aliases.Suggestions, "gs") {
		t.Fatal("conflicting gs alias was suggested")
	}
	if got := aliases.Exclusions.ByReason[model.ExclusionAliasConflict]; got != 1 {
		t.Fatalf("alias conflict count = %d, want 1", got)
	}
	if !reflect.DeepEqual(aliases.Exclusions.ExistingAliasConflicts, []string{"gs"}) {
		t.Fatalf("ExistingAliasConflicts = %#v, want [gs]", aliases.Exclusions.ExistingAliasConflicts)
	}
}

func TestGenerateAliasSuggestionsSkipsCommonSystemCommandConflict(t *testing.T) {
	command := directAggregate("make", 5)

	aliases := GenerateAliasSuggestionsFromCommands([]CommandAggregate{command}, AliasOptions{MinFrequency: 3, MaxAliases: 25})

	if len(aliases.Suggestions) != 0 {
		t.Fatalf("suggestions = %#v, want none", aliases.Suggestions)
	}
	if got := aliases.Exclusions.ByReason[model.ExclusionAliasConflict]; got != 1 {
		t.Fatalf("system command conflict count = %d, want 1", got)
	}
}

func TestGenerateAliasSuggestionsSkipsInvalidAliasName(t *testing.T) {
	command := directAggregate("123tool command", 5)

	aliases := GenerateAliasSuggestionsFromCommands([]CommandAggregate{command}, AliasOptions{MinFrequency: 3, MaxAliases: 25})

	if len(aliases.Suggestions) != 0 {
		t.Fatalf("suggestions = %#v, want none for invalid alias name", aliases.Suggestions)
	}
	if got := aliases.Exclusions.ByReason[model.ExclusionTooShort]; got != 1 {
		t.Fatalf("invalid alias exclusion count = %d, want 1", got)
	}
}

func TestGenerateAliasSuggestionsAppliesThresholdsAndConventionExceptions(t *testing.T) {
	commands := []CommandAggregate{
		directAggregate("git add", 3),
		directAggregate("abcd efg hij", 3),
		directAggregate("npm run once", 2),
	}

	aliases := GenerateAliasSuggestionsFromCommands(commands, AliasOptions{MinFrequency: 3, MaxAliases: 25})

	requireAlias(t, aliases.Suggestions, "ga")
	if hasAlias(aliases.Suggestions, "abcdefhij") {
		t.Fatal("low-savings fallback alias was suggested")
	}
	if hasAlias(aliases.Suggestions, "nronce") {
		t.Fatal("low-frequency fallback alias was suggested")
	}
	if got := aliases.Exclusions.ByReason[model.ExclusionLowSavings]; got != 1 {
		t.Fatalf("low savings count = %d, want 1", got)
	}
	if got := aliases.Exclusions.ByReason[model.ExclusionLowFrequency]; got != 1 {
		t.Fatalf("low frequency count = %d, want 1", got)
	}
}

func TestGenerateAliasSuggestionsEnforcesMaxAliasesAndMinFrequency(t *testing.T) {
	commands := []CommandAggregate{
		directAggregate("git status", 4),
		directAggregate("npm run build", 4),
		directAggregate("pnpm run build", 4),
		directAggregate("yarn test", 3),
	}

	aliases := GenerateAliasSuggestionsFromCommands(commands, AliasOptions{MinFrequency: 4, MaxAliases: 2})

	if hasAlias(aliases.Suggestions, "yt") {
		t.Fatal("alias below min frequency was suggested")
	}
	if len(aliases.AliasFileSuggestions) != 2 {
		t.Fatalf("AliasFileSuggestions len = %d, want 2", len(aliases.AliasFileSuggestions))
	}
	for _, suggestion := range aliases.AliasFileSuggestions {
		if !suggestion.AliasFileEligible {
			t.Fatalf("%q AliasFileEligible = false, want true", suggestion.Name)
		}
	}
}

func TestGenerateAliasSuggestionsExcludesUnsafeAggregates(t *testing.T) {
	result := AggregateRecords([]model.CommandRecord{
		rawRecord(model.ShellZsh, "zsh", "kubectl delete pod old-worker"),
		rawRecord(model.ShellZsh, "zsh", "echo token=fake-secret-value"),
		rawRecord(model.ShellZsh, "zsh", "npm run build\nnpm test"),
		{SourceShell: model.ShellZsh, SourceFile: "zsh", ParseStatus: model.ParseStatusFailed},
	}, nil, AggregateOptions{})

	aliases := GenerateAliasSuggestions(result, AliasOptions{MinFrequency: 1, MaxAliases: 25})

	if len(aliases.Suggestions) != 0 {
		t.Fatalf("unsafe suggestions = %#v, want none", aliases.Suggestions)
	}
}

func TestGenerateAliasSuggestionsKeepsLowConfidenceOutOfAliasFiles(t *testing.T) {
	command := directAggregate("npm run ./scripts/project-build", 5)

	aliases := GenerateAliasSuggestionsFromCommands([]CommandAggregate{command}, AliasOptions{MinFrequency: 3, MaxAliases: 25})

	if len(aliases.Suggestions) != 1 {
		t.Fatalf("suggestions len = %d, want 1", len(aliases.Suggestions))
	}
	if aliases.Suggestions[0].Confidence != model.ConfidenceLow {
		t.Fatalf("confidence = %q, want low", aliases.Suggestions[0].Confidence)
	}
	if len(aliases.AliasFileSuggestions) != 0 {
		t.Fatalf("AliasFileSuggestions = %#v, want none for low confidence", aliases.AliasFileSuggestions)
	}
}

func TestGenerateAliasSuggestionsStableEqualScoreOrdering(t *testing.T) {
	commands := []CommandAggregate{
		directAggregateWithTool("btool subcommand extra", "btool", 3),
		directAggregateWithTool("atool subcommand extra", "atool", 3),
	}

	var want []string
	for i := 0; i < 20; i++ {
		aliases := GenerateAliasSuggestionsFromCommands(commands, AliasOptions{MinFrequency: 3, MaxAliases: 25})
		got := aliasNames(aliases.Suggestions)
		if i == 0 {
			want = got
			continue
		}
		if !reflect.DeepEqual(got, want) {
			t.Fatalf("alias order run %d = %#v, want %#v", i, got, want)
		}
	}

	if !reflect.DeepEqual(want, []string{"atoolsubext", "btoolsubext"}) {
		t.Fatalf("equal score alias order = %#v", want)
	}
}

func aggregateRawCommands(raws ...string) AggregateResult {
	records := make([]model.CommandRecord, 0, len(raws))
	for _, raw := range raws {
		records = append(records, rawRecord(model.ShellZsh, "zsh", raw))
	}
	return AggregateRecords(records, nil, AggregateOptions{})
}

func directAggregate(normalized string, count int) CommandAggregate {
	analysis := AnalyzeCommand(normalized)
	return CommandAggregate{
		NormalizedCommand: analysis.NormalizedCommand,
		DisplayCommand:    analysis.NormalizedCommand,
		Tool:              analysis.Tool,
		Subcommand:        analysis.Subcommand,
		Count:             count,
		SourceShells:      []model.Shell{model.ShellZsh},
	}
}

func directAggregateWithTool(normalized string, tool string, count int) CommandAggregate {
	command := directAggregate(normalized, count)
	command.Tool = tool
	return command
}

func requireAlias(t *testing.T, suggestions []model.AliasSuggestion, name string) model.AliasSuggestion {
	t.Helper()
	for _, suggestion := range suggestions {
		if suggestion.Name == name {
			return suggestion
		}
	}
	t.Fatalf("missing alias %q in %#v", name, suggestions)
	return model.AliasSuggestion{}
}

func hasAlias(suggestions []model.AliasSuggestion, name string) bool {
	for _, suggestion := range suggestions {
		if suggestion.Name == name {
			return true
		}
	}
	return false
}

func aliasNames(suggestions []model.AliasSuggestion) []string {
	names := make([]string, len(suggestions))
	for i, suggestion := range suggestions {
		names[i] = suggestion.Name
	}
	return names
}
