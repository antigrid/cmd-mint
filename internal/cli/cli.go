package cli

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"

	"cmd-mint/internal/analyze"
	"cmd-mint/internal/discovery"
	"cmd-mint/internal/model"
	"cmd-mint/internal/output"
	"cmd-mint/internal/parser"
	"cmd-mint/internal/version"
)

func Run(args []string, stdout io.Writer, stderr io.Writer, build version.BuildInfo) int {
	return Runner{
		Stdout: stdout,
		Stderr: stderr,
		Build:  build,
	}.Run(args)
}

type Runner struct {
	Stdout io.Writer
	Stderr io.Writer
	Now    func() time.Time
	Build  version.BuildInfo
}

func (r Runner) Run(args []string) int {
	stdout := r.Stdout
	if stdout == nil {
		stdout = io.Discard
	}
	stderr := r.Stderr
	if stderr == nil {
		stderr = io.Discard
	}
	now := r.Now
	if now == nil {
		now = time.Now
	}
	return run(args, stdout, stderr, r.Build, now)
}

func run(args []string, stdout io.Writer, stderr io.Writer, build version.BuildInfo, nowFunc func() time.Time) int {
	result, err := parseFlags(args, stderr)
	if err != nil {
		var outputErr *output.Error
		if errors.As(err, &outputErr) {
			fmt.Fprintf(stderr, "cmd-mint: output error: %v\n", err)
			return ExitOutputError
		}
		fmt.Fprintf(stderr, "cmd-mint: invalid arguments: %v\n", err)
		return ExitInvalidArgs
	}

	if result.Help {
		fmt.Fprint(stdout, helpText())
		return ExitOK
	}

	if result.Options.ShowVersion {
		fmt.Fprintln(stdout, build.String())
		return ExitOK
	}

	discovered, err := discovery.DiscoverHistorySources(discovery.Options{
		HistoryFiles: result.Options.HistoryFiles,
		Shell:        result.Options.Shell,
	})
	if err != nil {
		if errors.Is(err, discovery.ErrNoUsableSources) {
			fmt.Fprint(stderr, discovery.NoSourcesMessage())
			return ExitNoInput
		}
		fmt.Fprintf(stderr, "cmd-mint: internal error: %v\n", err)
		return ExitInternalError
	}

	shellConfig := discovery.DiscoverShellConfigAliases()
	records, sources, parseWarnings := parseHistorySources(discovered.Sources)
	if !hasParsedSource(sources) {
		fmt.Fprint(stderr, discovery.NoSourcesMessage())
		return ExitNoInput
	}

	now := nowFunc()
	report := analyze.BuildReport(records, sources, analyze.ReportOptions{
		GeneratedAt:     now,
		MinFrequency:    result.Options.MinFrequency,
		MaxAliases:      result.Options.MaxAliases,
		ExistingAliases: shellConfig.Aliases,
		Warnings:        append(shellConfig.Warnings, parseWarnings...),
	})

	cwd, err := os.Getwd()
	if err != nil {
		fmt.Fprintf(stderr, "cmd-mint: internal error: %v\n", err)
		return ExitInternalError
	}

	outputDir, err := output.CreateReportDirectory(cwd, result.Options.OutputDir, now)
	if err != nil {
		var outputErr *output.Error
		if errors.As(err, &outputErr) {
			fmt.Fprintf(stderr, "cmd-mint: output error: %v\n", err)
			return ExitOutputError
		}
		fmt.Fprintf(stderr, "cmd-mint: internal error: %v\n", err)
		return ExitInternalError
	}

	generatedPaths := []string{filepath.Join(outputDir, output.ArtifactCheatsheet)}
	if err := output.WriteArtifact(outputDir, output.ArtifactCheatsheet, output.RenderCheatsheetMarkdown(report)); err != nil {
		fmt.Fprintf(stderr, "cmd-mint: output error: %v\n", err)
		return ExitOutputError
	}

	artifactPaths, err := output.WriteAliasAndJSONArtifacts(outputDir, report, output.ArtifactOptions{
		NoAliasFile: result.Options.NoAliasFile,
		MaxAliases:  result.Options.MaxAliases,
		JSON:        result.Options.JSON,
		Shell:       result.Options.Shell,
	})
	if err != nil {
		fmt.Fprintf(stderr, "cmd-mint: output error: %v\n", err)
		return ExitOutputError
	}
	generatedPaths = append(generatedPaths, artifactPaths...)

	if _, err := stdout.Write(output.RenderTerminalSummary(report, output.TerminalSummaryOptions{
		GeneratedPaths: generatedPaths,
		CWD:            cwd,
		Verbose:        result.Options.Verbose,
	})); err != nil {
		fmt.Fprintf(stderr, "cmd-mint: internal error: %v\n", err)
		return ExitInternalError
	}
	return ExitOK
}

func parseHistorySources(sources []model.HistorySource) ([]model.CommandRecord, []model.SourceSummary, []model.Warning) {
	var records []model.CommandRecord
	var summaries []model.SourceSummary
	var warnings []model.Warning

	for _, source := range sources {
		result, err := parser.ParseFile(source)
		if err != nil {
			warnings = append(warnings, model.Warning{
				Code:       "parse_source_failed",
				Message:    "history source could not be parsed",
				SourceFile: source.SourceFile,
			})
			summaries = append(summaries, model.SourceSummary{
				SourceShell:    source.SourceShell,
				SourceFile:     source.SourceFile,
				EntriesSkipped: 1,
				Warnings:       []string{"history source could not be parsed"},
			})
			continue
		}
		records = append(records, result.Commands...)
		summaries = append(summaries, result.Summary)
	}

	return records, summaries, warnings
}

func hasParsedSource(sources []model.SourceSummary) bool {
	for _, source := range sources {
		if source.EntriesParsed > 0 {
			return true
		}
	}
	return false
}

func helpText() string {
	return `cmd-mint analyzes local shell history and suggests safe command cheatsheets.

Usage:
  cmd-mint [flags]

Flags:
  --history-file PATH    history file to scan; repeat for multiple files
  --shell SHELL          shell format for explicit history files: bash, zsh, fish, or auto (default auto)
  --output-dir PATH      directory for generated artifacts
  --max-aliases N        maximum alias suggestions to write (default 25)
  --min-frequency N      minimum command frequency for alias eligibility (default 3)
  --no-alias-file        do not generate alias snippet files
  --json                 also write safe report.json
  --verbose              print extra safe parsing and exclusion summary information
  --version              print version information and exit
  --help                 print help and exit

cmd-mint reads history files locally and writes a report directory. It does not
execute history commands, modify shell configuration, modify history files, call
networks, or collect telemetry.
`
}
