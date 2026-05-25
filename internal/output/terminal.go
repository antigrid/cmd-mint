package output

import (
	"bytes"
	"fmt"
	"path/filepath"
	"strings"

	"cmd-mint/internal/model"
)

const topTerminalItems = 4

type TerminalSummaryOptions struct {
	GeneratedPaths []string
	CWD            string
	Verbose        bool
}

func RenderTerminalSummary(report model.Report, options TerminalSummaryOptions) []byte {
	report = safeReport(report)
	var buffer bytes.Buffer

	fmt.Fprintln(&buffer, "cmd-mint: analyzed shell history locally")
	fmt.Fprintln(&buffer)

	fmt.Fprintln(&buffer, "Sources scanned:")
	for _, source := range sortedSources(report.Sources) {
		fmt.Fprintf(&buffer, "  - %-36s %-5s parsed %d / skipped %d\n",
			displayPath(source.SourceFile, options.CWD),
			source.SourceShell,
			source.EntriesParsed,
			source.EntriesSkipped,
		)
	}
	if len(report.Sources) == 0 {
		fmt.Fprintln(&buffer, "  - none")
	}
	fmt.Fprintln(&buffer)

	fmt.Fprintf(&buffer, "Non-sensitive commands analyzed: %d\n", report.Summary.SafeCommandsAnalyzed)
	fmt.Fprintf(&buffer, "Sensitive-looking commands skipped: %d\n", report.Summary.SensitiveCommandsSkipped)
	fmt.Fprintf(&buffer, "Risky commands excluded from aliases: %d\n", report.Summary.RiskyCommandsExcluded)
	fmt.Fprintf(&buffer, "Alias conflicts skipped: %d\n", report.Summary.AliasConflictsSkipped)
	if options.Verbose {
		fmt.Fprintf(&buffer, "History entries read: %d\n", report.Summary.TotalEntriesRead)
		fmt.Fprintf(&buffer, "History entries skipped: %d\n", report.Summary.TotalEntriesSkipped)
		fmt.Fprintf(&buffer, "Warnings: %d\n", len(report.Warnings))
	}
	fmt.Fprintln(&buffer)

	fmt.Fprintln(&buffer, "Top tools:")
	topTools := sortedToolSummaries(report.TopTools)
	if len(topTools) == 0 {
		topTools = sortedToolSummaries(report.Summary.TopTools)
	}
	if len(topTools) == 0 {
		fmt.Fprintln(&buffer, "  none")
	} else {
		for _, tool := range limitTools(topTools, topTerminalItems) {
			fmt.Fprintf(&buffer, "  %-10s %d commands\n", tool.Tool, tool.Count)
		}
	}
	fmt.Fprintln(&buffer)

	fmt.Fprintln(&buffer, "Top alias suggestions:")
	suggestions := sortedAliasSuggestions(report.AliasSuggestions)
	if len(suggestions) == 0 {
		fmt.Fprintln(&buffer, "  none")
	} else {
		for _, suggestion := range limitAliases(suggestions, topTerminalItems) {
			fmt.Fprintf(&buffer, "  alias %s=%s  seen %dx\n",
				suggestion.Name,
				quoteShellSingle(suggestion.Command),
				suggestion.Frequency,
			)
		}
	}
	fmt.Fprintln(&buffer)

	fmt.Fprintln(&buffer, "Generated:")
	paths := append([]string(nil), options.GeneratedPaths...)
	for _, path := range paths {
		fmt.Fprintf(&buffer, "  %s\n", displayPath(path, options.CWD))
	}
	if len(paths) == 0 {
		fmt.Fprintln(&buffer, "  none")
	}
	fmt.Fprintln(&buffer)
	fmt.Fprintln(&buffer, "Generated locally from shell history. Review before sharing or committing.")

	return buffer.Bytes()
}

func displayPath(path string, cwd string) string {
	if path == "" {
		return ""
	}
	if cwd == "" {
		return path
	}
	rel, err := filepath.Rel(cwd, path)
	if err != nil || rel == "." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) || rel == ".." {
		return path
	}
	if filepath.IsAbs(path) {
		return "." + string(filepath.Separator) + rel
	}
	return rel
}

func limitTools(values []model.ToolSummary, limit int) []model.ToolSummary {
	if len(values) <= limit {
		return values
	}
	return values[:limit]
}

func limitAliases(values []model.AliasSuggestion, limit int) []model.AliasSuggestion {
	if len(values) <= limit {
		return values
	}
	return values[:limit]
}
