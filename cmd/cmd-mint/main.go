package main

import (
	"os"

	"cmd-mint/internal/cli"
	"cmd-mint/internal/version"
)

func main() {
	os.Exit(cli.Run(os.Args[1:], os.Stdout, os.Stderr, version.Info()))
}
