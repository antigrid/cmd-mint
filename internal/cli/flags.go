package cli

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"cmd-mint/internal/discovery"
	"cmd-mint/internal/model"
	"cmd-mint/internal/output"
)

const (
	defaultMaxAliases   = 25
	defaultMinFrequency = 3

	outputFormatMarkdown = "markdown"
	outputFormatAliases  = "aliases"
	outputFormatJSON     = "json"

	riskToleranceBalanced     = "balanced"
	riskToleranceConservative = "conservative"
)

type Options struct {
	HistoryFiles  []string
	Shell         model.Shell
	OutputDir     string
	ConfigFile    string
	MaxAliases    int
	MinFrequency  int
	NoAliasFile   bool
	JSON          bool
	OutputFormats []string
	IgnoredTools  []string
	RiskTolerance string
	Verbose       bool
	ShowVersion   bool
	ShowHelp      bool
	GeneratedAt   *time.Time
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

type configFile struct {
	IgnoredTools  []string `json:"ignored_tools,omitempty"`
	MinFrequency  *int     `json:"min_frequency,omitempty"`
	MaxAliases    *int     `json:"max_aliases,omitempty"`
	OutputFormats []string `json:"output_formats,omitempty"`
	RiskTolerance string   `json:"risk_tolerance,omitempty"`
}

func parseFlags(args []string, stderr io.Writer) (parseResult, error) {
	return parseFlagsWithEnv(args, stderr, discovery.DefaultEnv())
}

func parseFlagsWithEnv(args []string, stderr io.Writer, env discovery.Env) (parseResult, error) {
	opts := Options{
		Shell:         model.ShellAuto,
		MaxAliases:    defaultMaxAliases,
		MinFrequency:  defaultMinFrequency,
		OutputFormats: []string{outputFormatMarkdown, outputFormatAliases},
		RiskTolerance: riskToleranceBalanced,
	}

	fs := flag.NewFlagSet("cmd-mint", flag.ContinueOnError)
	fs.SetOutput(stderr)
	fs.Usage = func() {
		fmt.Fprint(fs.Output(), helpText())
	}

	var historyFiles repeatedStringFlag
	var ignoredTools repeatedStringFlag
	var outputFormats repeatedStringFlag
	fs.Var(&historyFiles, "history-file", "history file to scan; repeat for multiple files")
	shell := fs.String("shell", string(opts.Shell), "shell format for explicit history files: bash, zsh, fish, or auto")
	fs.StringVar(&opts.OutputDir, "output-dir", "", "directory for generated artifacts")
	fs.StringVar(&opts.ConfigFile, "config", "", "local JSON config file for thresholds and preferences")
	fs.IntVar(&opts.MaxAliases, "max-aliases", opts.MaxAliases, "maximum alias suggestions to write")
	fs.IntVar(&opts.MinFrequency, "min-frequency", opts.MinFrequency, "minimum command frequency for alias eligibility")
	fs.BoolVar(&opts.NoAliasFile, "no-alias-file", false, "do not generate alias snippet files")
	fs.BoolVar(&opts.JSON, "json", false, "also write safe report.json")
	fs.Var(&outputFormats, "format", "output format to write: markdown, aliases, or json; repeat to select multiple")
	fs.Var(&ignoredTools, "ignore-tool", "tool name to exclude from generated command references; repeat for multiple tools")
	fs.StringVar(&opts.RiskTolerance, "risk-tolerance", opts.RiskTolerance, "risk filtering preference: balanced or conservative")
	fs.BoolVar(&opts.Verbose, "verbose", false, "print extra safe parsing and exclusion summary information")
	fs.BoolVar(&opts.ShowVersion, "version", false, "print version information and exit")
	fs.BoolVar(&opts.ShowHelp, "help", false, "print help and exit")
	fs.BoolVar(&opts.ShowHelp, "h", false, "print help and exit")
	generatedAt := fs.String("generated-at", "", "RFC3339 timestamp to use in generated reports")

	if err := fs.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return parseResult{Options: opts, Help: true}, nil
		}
		return parseResult{}, err
	}
	if fs.NArg() > 0 {
		return parseResult{}, fmt.Errorf("unexpected positional argument: %s", fs.Arg(0))
	}

	seen := seenFlags(fs)
	opts.HistoryFiles = append([]string(nil), historyFiles...)
	opts.Shell = model.Shell(*shell)
	opts.IgnoredTools = append([]string(nil), ignoredTools...)
	if len(outputFormats) > 0 {
		opts.OutputFormats = append([]string(nil), outputFormats...)
	}

	if strings.TrimSpace(*generatedAt) != "" {
		parsed, err := time.Parse(time.RFC3339, strings.TrimSpace(*generatedAt))
		if err != nil {
			return parseResult{}, fmt.Errorf("--generated-at must be an RFC3339 timestamp")
		}
		opts.GeneratedAt = &parsed
	}
	if opts.ShowHelp {
		return parseResult{Options: opts, Help: true}, nil
	}

	if err := applyConfigWithEnv(&opts, seen, env); err != nil {
		return parseResult{}, err
	}
	applyLegacyOutputFlags(&opts, seen)

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

func seenFlags(fs *flag.FlagSet) map[string]bool {
	seen := make(map[string]bool)
	fs.Visit(func(flag *flag.Flag) {
		seen[flag.Name] = true
	})
	return seen
}

func applyConfigWithEnv(opts *Options, seen map[string]bool, env discovery.Env) error {
	if strings.TrimSpace(opts.ConfigFile) == "" {
		return nil
	}
	expanded, err := discovery.ExpandPath(opts.ConfigFile, env)
	if err != nil {
		return fmt.Errorf("--config %q: %w", opts.ConfigFile, err)
	}
	config, err := readConfigFile(expanded)
	if err != nil {
		return fmt.Errorf("--config %q: %w", opts.ConfigFile, err)
	}
	opts.ConfigFile = expanded

	if config.MinFrequency != nil && !seen["min-frequency"] {
		opts.MinFrequency = *config.MinFrequency
	}
	if config.MaxAliases != nil && !seen["max-aliases"] {
		opts.MaxAliases = *config.MaxAliases
	}
	if len(config.OutputFormats) > 0 && !seen["format"] {
		opts.OutputFormats = append([]string(nil), config.OutputFormats...)
	}
	if len(config.IgnoredTools) > 0 && !seen["ignore-tool"] {
		opts.IgnoredTools = append([]string(nil), config.IgnoredTools...)
	}
	if strings.TrimSpace(config.RiskTolerance) != "" && !seen["risk-tolerance"] {
		opts.RiskTolerance = config.RiskTolerance
	}
	return nil
}

func readConfigFile(path string) (configFile, error) {
	file, err := os.Open(path)
	if err != nil {
		return configFile{}, err
	}
	defer file.Close()

	decoder := json.NewDecoder(file)
	decoder.DisallowUnknownFields()
	var config configFile
	if err := decoder.Decode(&config); err != nil {
		return configFile{}, err
	}
	var trailing struct{}
	if err := decoder.Decode(&trailing); err != io.EOF {
		return configFile{}, fmt.Errorf("must contain a single JSON object")
	}
	return config, nil
}

func applyLegacyOutputFlags(opts *Options, seen map[string]bool) {
	if seen["json"] && !containsString(opts.OutputFormats, outputFormatJSON) {
		opts.OutputFormats = append(opts.OutputFormats, outputFormatJSON)
	}
	opts.JSON = containsString(opts.OutputFormats, outputFormatJSON)

	if seen["no-alias-file"] {
		opts.OutputFormats = removeString(opts.OutputFormats, outputFormatAliases)
	}
	opts.NoAliasFile = !containsString(opts.OutputFormats, outputFormatAliases)
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

	if err := validateOutputFormats(opts.OutputFormats); err != nil {
		return err
	}

	switch strings.TrimSpace(opts.RiskTolerance) {
	case riskToleranceBalanced, riskToleranceConservative:
	default:
		return fmt.Errorf("--risk-tolerance must be one of balanced or conservative")
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

func validateOutputFormats(formats []string) error {
	if len(formats) == 0 {
		return fmt.Errorf("at least one output format must be selected")
	}
	seen := make(map[string]struct{})
	for _, format := range formats {
		normalized := strings.TrimSpace(format)
		switch normalized {
		case outputFormatMarkdown, outputFormatAliases, outputFormatJSON:
		default:
			return fmt.Errorf("--format must be one of markdown, aliases, or json")
		}
		if _, ok := seen[normalized]; ok {
			return fmt.Errorf("--format %q was provided more than once", normalized)
		}
		seen[normalized] = struct{}{}
	}
	return nil
}

func containsString(values []string, target string) bool {
	for _, value := range values {
		if strings.TrimSpace(value) == target {
			return true
		}
	}
	return false
}

func removeString(values []string, target string) []string {
	filtered := values[:0]
	for _, value := range values {
		if strings.TrimSpace(value) != target {
			filtered = append(filtered, value)
		}
	}
	return filtered
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
