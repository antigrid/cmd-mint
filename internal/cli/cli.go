package cli

import (
	"errors"
	"fmt"
	"io"

	"cmd-mint/internal/discovery"
	"cmd-mint/internal/output"
	"cmd-mint/internal/version"
)

func Run(args []string, stdout io.Writer, stderr io.Writer, build version.BuildInfo) int {
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

	_, err = discovery.DiscoverHistorySources(discovery.Options{
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

	fmt.Fprintln(stderr, "cmd-mint scaffold: history analysis is not implemented yet")
	return ExitInternalError
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

This scaffold does not read shell history, execute commands, modify shell
configuration, modify history files, create reports, call networks, or collect
telemetry.
`
}
