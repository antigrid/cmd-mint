package cli

import (
	"flag"
	"fmt"
	"io"

	"cmd-mint/internal/version"
)

const (
	ExitOK            = 0
	ExitInvalidArgs   = 2
	ExitInternalError = 4
)

func Run(args []string, stdout io.Writer, stderr io.Writer, build version.BuildInfo) int {
	fs := flag.NewFlagSet("cmd-mint", flag.ContinueOnError)
	fs.SetOutput(stderr)

	showVersion := fs.Bool("version", false, "print version information and exit")
	fs.Bool("help", false, "print help and exit")

	fs.Usage = func() {
		fmt.Fprint(fs.Output(), helpText())
	}

	if err := fs.Parse(args); err != nil {
		if err == flag.ErrHelp {
			return ExitOK
		}
		return ExitInvalidArgs
	}

	if helpRequested(args) {
		fmt.Fprint(stdout, helpText())
		return ExitOK
	}

	if *showVersion {
		fmt.Fprintln(stdout, build.String())
		return ExitOK
	}

	fmt.Fprintln(stderr, "cmd-mint scaffold: history analysis is not implemented yet")
	return ExitInternalError
}

func helpRequested(args []string) bool {
	for _, arg := range args {
		if arg == "--help" || arg == "-h" {
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
  --help       print help and exit
  --version    print version information and exit

This scaffold does not read shell history, execute commands, modify shell
configuration, modify history files, create reports, call networks, or collect
telemetry.
`
}
