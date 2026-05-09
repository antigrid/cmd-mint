package output

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestRenderTerminalSummaryIncludesRequiredFieldsAndPrivacyNote(t *testing.T) {
	report := sampleReport()
	report.AliasSuggestions[0].AliasFileEligible = true
	cwd := "/home/alex/project"

	summary := string(RenderTerminalSummary(report, TerminalSummaryOptions{
		CWD: cwd,
		GeneratedPaths: []string{
			filepath.Join(cwd, "shell-history-report-2026-06-01-143022", ArtifactCheatsheet),
			filepath.Join(cwd, "shell-history-report-2026-06-01-143022", ArtifactAliasesSH),
		},
	}))

	for _, want := range []string{
		"cmd-mint: analyzed shell history locally",
		"Sources scanned:",
		"/home/alex/.zsh_history",
		"parsed 10 / skipped 2",
		"Safe commands analyzed: 14",
		"Sensitive-looking commands skipped: 2",
		"Risky commands excluded from aliases: 1",
		"Alias conflicts skipped: 1",
		"Top tools:",
		"git",
		"Top alias suggestions:",
		"alias gs='git status'  seen 4x",
		"Generated:",
		"./shell-history-report-2026-06-01-143022/cheatsheet.md",
		"./shell-history-report-2026-06-01-143022/aliases.suggested.sh",
		"Generated locally from shell history. Review before sharing or committing.",
	} {
		if !strings.Contains(summary, want) {
			t.Fatalf("terminal summary missing %q:\n%s", want, summary)
		}
	}
}

func TestRenderTerminalSummaryOmitsRawSensitiveCommands(t *testing.T) {
	report := sampleReport()
	summary := string(RenderTerminalSummary(report, TerminalSummaryOptions{}))

	for _, forbidden := range []string{
		fakeSensitiveCommand,
		"raw-secret-token",
		"Authorization: Bearer",
	} {
		if strings.Contains(summary, forbidden) {
			t.Fatalf("terminal summary contains forbidden %q:\n%s", forbidden, summary)
		}
	}
}
