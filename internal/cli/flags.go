package cli

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"strings"

	"cmd-mint/internal/discovery"
	"cmd-mint/internal/model"
	"cmd-mint/internal/output"
)

const (
	defaultMaxAliases   = 25
	defaultMinFrequency = 3
)

type Options struct {
	HistoryFiles []string
	Shell        model.Shell
	OutputDir    string
	MaxAliases   int
	MinFrequency int
	NoAliasFile  bool
	JSON         bool
	Verbose      bool
	ShowVersion  bool
	ShowHelp     bool
}

type parseResult struct {
	Options Options
	Help    bool
}

type repeatedStringFlag []string

func (f *repeatedStringFlag) String() string {
	return strings.Join(*f, ",")
}

func (f *repeatedStringFlag) Set(value string) error {
	*f = append(*f, value)
	return nil
}

func parseFlags(args []string, stderr io.Writer) (parseResult, error) {
	return parseFlagsWithEnv(args, stderr, discovery.DefaultEnv())
}

func parseFlagsWithEnv(args []string, stderr io.Writer, env discovery.Env) (parseResult, error) {
	opts := Options{
		Shell:        model.ShellAuto,
		MaxAliases:   defaultMaxAliases,
		MinFrequency: defaultMinFrequency,
	}

	fs := flag.NewFlagSet("cmd-mint", flag.ContinueOnError)
	fs.SetOutput(stderr)
	fs.Usage = func() {
		fmt.Fprint(fs.Output(), helpText())
	}

	var historyFiles repeatedStringFlag
	fs.Var(&historyFiles, "history-file", "history file to scan; repeat for multiple files")
	shell := fs.String("shell", string(opts.Shell), "shell format for explicit history files: bash, zsh, fish, or auto")
	fs.StringVar(&opts.OutputDir, "output-dir", "", "directory for generated artifacts")
	fs.IntVar(&opts.MaxAliases, "max-aliases", opts.MaxAliases, "maximum alias suggestions to write")
	fs.IntVar(&opts.MinFrequency, "min-frequency", opts.MinFrequency, "minimum command frequency for alias eligibility")
	fs.BoolVar(&opts.NoAliasFile, "no-alias-file", false, "do not generate alias snippet files")
	fs.BoolVar(&opts.JSON, "json", false, "also write safe report.json")
	fs.BoolVar(&opts.Verbose, "verbose", false, "print extra safe parsing and exclusion summary information")
	fs.BoolVar(&opts.ShowVersion, "version", false, "print version information and exit")
	fs.BoolVar(&opts.ShowHelp, "help", false, "print help and exit")
	fs.BoolVar(&opts.ShowHelp, "h", false, "print help and exit")

	if err := fs.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return parseResult{Options: opts, Help: true}, nil
		}
		return parseResult{}, err
	}
	if fs.NArg() > 0 {
		return parseResult{}, fmt.Errorf("unexpected positional argument: %s", fs.Arg(0))
	}

	opts.HistoryFiles = append([]string(nil), historyFiles...)
	opts.Shell = model.Shell(*shell)
	if opts.ShowHelp {
		return parseResult{Options: opts, Help: true}, nil
	}

	if err := validateOptionsWithEnv(opts, env); err != nil {
		return parseResult{}, err
	}
	if opts.OutputDir != "" {
		expanded, err := discovery.ExpandPath(opts.OutputDir, env)
		if err != nil {
			return parseResult{}, err
		}
		opts.OutputDir = expanded
	}

	return parseResult{Options: opts}, nil
}

func validateOptions(opts Options) error {
	return validateOptionsWithEnv(opts, discovery.DefaultEnv())
}

func validateOptionsWithEnv(opts Options, env discovery.Env) error {
	switch opts.Shell {
	case model.ShellAuto, model.ShellBash, model.ShellZsh, model.ShellFish:
	default:
		return fmt.Errorf("--shell must be one of bash, zsh, fish, or auto")
	}

	if opts.MinFrequency < 1 {
		return fmt.Errorf("--min-frequency must be >= 1")
	}

	if opts.MaxAliases < 0 {
		return fmt.Errorf("--max-aliases must be >= 0")
	}

	for _, path := range opts.HistoryFiles {
		if err := discovery.ValidateReadableHistoryFileWithEnv(path, env); err != nil {
			return fmt.Errorf("--history-file %q: %w", path, err)
		}
	}

	if opts.OutputDir != "" {
		if err := validateOutputDirPathWithEnv(opts.OutputDir, env); err != nil {
			return err
		}
	}

	return nil
}

func validateOutputDirPath(path string) error {
	return validateOutputDirPathWithEnv(path, discovery.DefaultEnv())
}

func validateOutputDirPathWithEnv(path string, env discovery.Env) error {
	expanded, err := discovery.ExpandPath(path, env)
	if err != nil {
		return outputPathError(path, err)
	}
	info, err := os.Stat(expanded)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return outputPathError(expanded, err)
	}
	if info.Mode().IsRegular() {
		return outputPathError(expanded, fmt.Errorf("must not be an existing regular file"))
	}
	return nil
}

func outputPathError(path string, err error) error {
	return &output.Error{
		Op:   "validate --output-dir",
		Path: path,
		Err:  err,
	}
}
