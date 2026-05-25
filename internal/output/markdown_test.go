package output

import (
	"strings"
	"testing"
	"time"

	"cmd-mint/internal/model"
)

const fakeSensitiveCommand = `curl -H "Authorization: Bearer raw-secret-token" https://example.invalid`

func TestRenderCheatsheetMarkdownIncludesRequiredSectionsInOrder(t *testing.T) {
	markdown := string(RenderCheatsheetMarkdown(sampleReport()))

	required := []string{
		"# Shell History Cheat Sheet",
		"Generated locally by cmd-mint on 2026-06-01 14:30:22.",
		"## Summary",
		"## Sources scanned",
		"## Alias suggestions",
		"## Commands by tool",
		"## Frequent command patterns",
		"## Excluded from alias suggestions",
		"## Privacy and safety summary",
	}

	last := -1
	for _, section := range required {
		index := strings.Index(markdown, section)
		if index == -1 {
			t.Fatalf("missing required section %q in:\n%s", section, markdown)
		}
		if index <= last {
			t.Fatalf("section %q appeared out of order in:\n%s", section, markdown)
		}
		last = index
	}
}

func TestRenderCheatsheetMarkdownOmitsSensitiveCommandsAndShowsAggregates(t *testing.T) {
	report := sampleReport()
	report.Exclusions.ByReason[model.ExclusionSensitiveAuthHeader] = 2
	report.Exclusions.SensitiveCategories[model.SensitivityAuthHeader] = 2

	markdown := string(RenderCheatsheetMarkdown(report))

	for _, forbidden := range []string{
		fakeSensitiveCommand,
		"raw-secret-token",
		"Authorization: Bearer",
	} {
		if strings.Contains(markdown, forbidden) {
			t.Fatalf("markdown contains raw sensitive value %q:\n%s", forbidden, markdown)
		}
	}
	for _, want := range []string{
		"Sensitive-looking commands skipped: 2",
		"`sensitive_auth_header`: 2",
		"`auth_header`: 2",
		"Raw sensitive skipped commands are not included",
	} {
		if !strings.Contains(markdown, want) {
			t.Fatalf("markdown missing aggregate %q:\n%s", want, markdown)
		}
	}
}

func TestRenderCheatsheetMarkdownFiltersSensitiveCommandsThatSlipIntoRenderData(t *testing.T) {
	// Defense in depth: even if a sensitive command evades upstream
	// classification and reaches the rendered report structures, the shared
	// safeReport gate must strip it from cheatsheet.md.
	report := sampleReport()
	report.AliasSuggestions = append(report.AliasSuggestions, model.AliasSuggestion{
		Name:       "leak",
		Command:    fakeSensitiveCommand,
		Tool:       "curl",
		Frequency:  5,
		Confidence: model.ConfidenceHigh,
	})
	report.ToolSections = append(report.ToolSections, model.ToolSection{
		Tool:  "curl",
		Count: 5,
		Commands: []model.SafeCommandSummary{
			{Command: fakeSensitiveCommand, NormalizedCommand: fakeSensitiveCommand, Count: 5, Tool: "curl"},
		},
	})
	report.Patterns = append(report.Patterns, model.PatternSummary{
		Pattern:  fakeSensitiveCommand,
		Count:    5,
		Examples: []string{fakeSensitiveCommand},
	})

	markdown := string(RenderCheatsheetMarkdown(report))

	for _, forbidden := range []string{fakeSensitiveCommand, "raw-secret-token", "Authorization: Bearer"} {
		if strings.Contains(markdown, forbidden) {
			t.Fatalf("markdown leaked sensitive value %q:\n%s", forbidden, markdown)
		}
	}
	if !strings.Contains(markdown, "git status") {
		t.Fatalf("markdown dropped safe content alongside the sensitive command:\n%s", markdown)
	}
}

func TestRenderTerminalSummaryFiltersSensitiveAliasThatSlipsIntoRenderData(t *testing.T) {
	report := sampleReport()
	report.AliasSuggestions = append(report.AliasSuggestions, model.AliasSuggestion{
		Name:       "leak",
		Command:    fakeSensitiveCommand,
		Tool:       "curl",
		Frequency:  99,
		Confidence: model.ConfidenceHigh,
	})

	summary := string(RenderTerminalSummary(report, TerminalSummaryOptions{}))

	for _, forbidden := range []string{fakeSensitiveCommand, "raw-secret-token", "Authorization: Bearer"} {
		if strings.Contains(summary, forbidden) {
			t.Fatalf("terminal summary leaked sensitive value %q:\n%s", forbidden, summary)
		}
	}
}

func TestRenderCheatsheetMarkdownIncludesAliasDetails(t *testing.T) {
	markdown := string(RenderCheatsheetMarkdown(sampleReport()))

	for _, want := range []string{
		"### `gs`",
		"- Expands to: `git status`",
		"- Frequency: 4",
		"- Estimated characters saved per use: 8",
		"- Estimated total characters saved: 32",
		"- Confidence: high",
		"- Reason: known conservative convention for a frequent exact command",
		"- Source shells: bash, zsh",
	} {
		if !strings.Contains(markdown, want) {
			t.Fatalf("markdown missing alias detail %q:\n%s", want, markdown)
		}
	}
}

func TestRenderCheatsheetMarkdownSortsToolSectionsDeterministically(t *testing.T) {
	report := sampleReport()
	report.ToolSections = []model.ToolSection{
		{
			Tool:  "npm",
			Count: 5,
			Commands: []model.SafeCommandSummary{
				{Command: "npm test", NormalizedCommand: "npm test", Count: 2},
				{Command: "npm run build", NormalizedCommand: "npm run build", Count: 4},
			},
			Subcommands: []model.SubcommandSummary{
				{Subcommand: "test", Count: 2},
				{Subcommand: "run", Count: 4},
			},
		},
		{
			Tool:  "docker",
			Count: 7,
			Commands: []model.SafeCommandSummary{
				{Command: "docker ps", NormalizedCommand: "docker ps", Count: 7},
			},
		},
		{
			Tool:  "git",
			Count: 5,
			Commands: []model.SafeCommandSummary{
				{Command: "git status", NormalizedCommand: "git status", Count: 4},
			},
		},
	}
	report.AliasSuggestions = []model.AliasSuggestion{
		{Name: "gs", Command: "git status", Tool: "git", Frequency: 4},
	}

	markdown := string(RenderCheatsheetMarkdown(report))

	assertAppearsInOrder(t, markdown, []string{
		"### docker",
		"### git",
		"### npm",
	})
	assertAppearsInOrder(t, markdown, []string{
		"`npm run build`: seen 4x",
		"`npm test`: seen 2x",
	})
	assertAppearsInOrder(t, markdown, []string{
		"`run`: 4 commands",
		"`test`: 2 commands",
	})
}

func TestRenderCheatsheetMarkdownIncludesPatternsAndExclusions(t *testing.T) {
	markdown := string(RenderCheatsheetMarkdown(sampleReport()))

	for _, want := range []string{
		"Patterns are shown for review only and are not emitted as aliases in the MVP.",
		"`git checkout <branch>`: seen 3x",
		"`git checkout feature/ticket-16`",
		"`kubectl delete pod old-worker -n staging`: seen 1x; representative normalized command: `kubectl delete pod old-worker -n staging`; source shells: zsh; label: risky observed command, excluded from alias suggestions",
		"- Existing alias conflicts skipped: 1",
		"- Existing alias conflict names: `ll`",
		"- Multiline command count: 1",
		"- Low-frequency candidate count: 3",
		"- Too-short candidate count: 2",
		"`destructive`: 1",
	} {
		if !strings.Contains(markdown, want) {
			t.Fatalf("markdown missing %q:\n%s", want, markdown)
		}
	}
}

func sampleReport() model.Report {
	generatedAt := time.Date(2026, 6, 1, 14, 30, 22, 0, time.UTC)
	report := model.NewReport(generatedAt)
	report.Summary = model.AggregateSummary{
		TotalSourcesScanned:      2,
		SafeCommandsAnalyzed:     14,
		SensitiveCommandsSkipped: 2,
		RiskyCommandsExcluded:    1,
		AliasConflictsSkipped:    1,
		TopTools: []model.ToolSummary{
			{Tool: "git", Count: 8},
			{Tool: "docker", Count: 6},
		},
	}
	report.Sources = []model.SourceSummary{
		{
			SourceShell:    model.ShellZsh,
			SourceFile:     "/home/alex/.zsh_history",
			EntriesRead:    12,
			EntriesParsed:  10,
			EntriesSkipped: 2,
			Warnings:       []string{"malformed zsh extended history entries skipped"},
		},
		{
			SourceShell:    model.ShellBash,
			SourceFile:     "/home/alex/.bash_history",
			EntriesRead:    4,
			EntriesParsed:  4,
			EntriesSkipped: 0,
		},
	}
	report.AliasSuggestions = []model.AliasSuggestion{
		{
			Name:                 "gs",
			Command:              "git status",
			Tool:                 "git",
			Frequency:            4,
			EstimatedSavedPerUse: 8,
			EstimatedSavedTotal:  32,
			Confidence:           model.ConfidenceHigh,
			Reason:               "known conservative convention for a frequent exact command",
			SourceShells:         []model.Shell{model.ShellZsh, model.ShellBash},
		},
	}
	report.ToolSections = []model.ToolSection{
		{
			Tool:  "git",
			Count: 8,
			Commands: []model.SafeCommandSummary{
				{
					Command:           "git status",
					NormalizedCommand: "git status",
					Count:             4,
					Tool:              "git",
					Subcommand:        "status",
					SourceShells:      []model.Shell{model.ShellZsh, model.ShellBash},
				},
				{
					Command:           "git checkout feature/ticket-16",
					NormalizedCommand: "git checkout feature/ticket-16",
					Count:             3,
					Tool:              "git",
					Subcommand:        "checkout",
					SourceShells:      []model.Shell{model.ShellZsh},
				},
			},
			Subcommands: []model.SubcommandSummary{
				{Subcommand: "status", Count: 4},
				{Subcommand: "checkout", Count: 3},
			},
		},
		{
			Tool:  "docker",
			Count: 6,
			Commands: []model.SafeCommandSummary{
				{Command: "docker ps", NormalizedCommand: "docker ps", Count: 6, Tool: "docker", Subcommand: "ps"},
			},
			Subcommands: []model.SubcommandSummary{{Subcommand: "ps", Count: 6}},
		},
		{
			Tool:  "kubectl",
			Count: 1,
			Commands: []model.SafeCommandSummary{
				{
					Command:           "kubectl delete pod old-worker -n staging",
					NormalizedCommand: "kubectl delete pod old-worker -n staging",
					Count:             1,
					Tool:              "kubectl",
					Subcommand:        "delete",
					SourceShells:      []model.Shell{model.ShellZsh},
					RiskFlags:         []model.RiskFlag{model.RiskDestructive},
					ExclusionReasons:  []model.ExclusionReason{model.ExclusionRiskyDestructive},
				},
			},
			Subcommands: []model.SubcommandSummary{{Subcommand: "delete", Count: 1}},
		},
	}
	report.Patterns = []model.PatternSummary{
		{
			Pattern:  "git checkout <branch>",
			Count:    3,
			Examples: []string{"git checkout feature/ticket-16"},
		},
	}
	report.Exclusions = model.ExclusionSummary{
		ByReason: map[model.ExclusionReason]int{
			model.ExclusionSensitiveAuthHeader: 2,
			model.ExclusionRiskyDestructive:    1,
			model.ExclusionAliasConflict:       1,
			model.ExclusionLowFrequency:        3,
			model.ExclusionTooShort:            2,
		},
		ExistingAliasConflicts:   []string{"ll"},
		RiskyCategories:          map[model.RiskFlag]int{model.RiskDestructive: 1},
		SensitiveCategories:      map[model.SensitivityFlag]int{model.SensitivityAuthHeader: 2},
		MultilineCommandCount:    1,
		LowFrequencyCommandCount: 3,
		TooShortCommandCount:     2,
	}
	return report
}

func assertAppearsInOrder(t *testing.T, text string, values []string) {
	t.Helper()

	last := -1
	for _, value := range values {
		index := strings.Index(text, value)
		if index == -1 {
			t.Fatalf("missing %q in:\n%s", value, text)
		}
		if index <= last {
			t.Fatalf("%q appeared out of order in:\n%s", value, text)
		}
		last = index
	}
}
