package cli

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"cmd-mint/internal/discovery"
	"cmd-mint/internal/model"
	"cmd-mint/internal/output"
	"cmd-mint/internal/version"
)

func TestHelp(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	code := Run([]string{"--help"}, &stdout, &stderr, version.BuildInfo{})

	if code != ExitOK {
		t.Fatalf("Run(--help) exit code = %d, want %d", code, ExitOK)
	}
	if !strings.Contains(stdout.String(), "Usage:") {
		t.Fatalf("help output missing usage: %q", stdout.String())
	}
	if stderr.Len() != 0 {
		t.Fatalf("stderr = %q, want empty", stderr.String())
	}
}

func TestVersion(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	build := version.BuildInfo{
		Version: "1.2.3",
		Commit:  "abc123",
		Date:    "2026-06-01",
	}
	code := Run([]string{"--version"}, &stdout, &stderr, build)

	if code != ExitOK {
		t.Fatalf("Run(--version) exit code = %d, want %d", code, ExitOK)
	}
	want := "cmd-mint version 1.2.3 commit abc123 built 2026-06-01\n"
	if stdout.String() != want {
		t.Fatalf("version output = %q, want %q", stdout.String(), want)
	}
	if stderr.Len() != 0 {
		t.Fatalf("stderr = %q, want empty", stderr.String())
	}
}

func TestUnknownFlag(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	code := Run([]string{"--not-a-real-flag"}, &stdout, &stderr, version.BuildInfo{})

	if code != ExitInvalidArgs {
		t.Fatalf("Run(unknown flag) exit code = %d, want %d", code, ExitInvalidArgs)
	}
	if stdout.Len() != 0 {
		t.Fatalf("stdout = %q, want empty", stdout.String())
	}
}

func TestUnexpectedPositionalArgument(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	code := Run([]string{"unexpected"}, &stdout, &stderr, version.BuildInfo{})

	if code != ExitInvalidArgs {
		t.Fatalf("Run(positional arg) exit code = %d, want %d", code, ExitInvalidArgs)
	}
	if stdout.Len() != 0 {
		t.Fatalf("stdout = %q, want empty", stdout.String())
	}
	for _, want := range []string{
		"cmd-mint: invalid arguments:",
		"unexpected positional argument: unexpected",
	} {
		if !strings.Contains(stderr.String(), want) {
			t.Fatalf("stderr = %q, want substring %q", stderr.String(), want)
		}
	}
}

func TestNoHistorySourcesExitsCode1(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	t.Setenv("HISTFILE", "")
	t.Setenv("SHELL", "")

	var stdout bytes.Buffer
	var stderr bytes.Buffer

	code := Run(nil, &stdout, &stderr, version.BuildInfo{})

	if code != ExitNoInput {
		t.Fatalf("Run(no history sources) exit code = %d, want %d", code, ExitNoInput)
	}
	if stdout.Len() != 0 {
		t.Fatalf("stdout = %q, want empty", stdout.String())
	}
	if !strings.Contains(stderr.String(), "No supported shell history files found.") {
		t.Fatalf("stderr = %q, want no-history message", stderr.String())
	}
}

func TestRepeatableHistoryFilesArePreserved(t *testing.T) {
	dir := t.TempDir()
	first := writeTempHistoryFile(t, dir, "zsh.history")
	second := writeTempHistoryFile(t, dir, "bash.history")

	result, err := parseFlags([]string{
		"--history-file", first,
		"--history-file", second,
	}, &bytes.Buffer{})
	if err != nil {
		t.Fatalf("parseFlags returned error: %v", err)
	}

	want := []string{first, second}
	if strings.Join(result.Options.HistoryFiles, "\x00") != strings.Join(want, "\x00") {
		t.Fatalf("HistoryFiles = %#v, want %#v", result.Options.HistoryFiles, want)
	}
}

func TestRunnerUsesInjectedClockForDefaultOutput(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	t.Setenv("HISTFILE", "")
	t.Setenv("SHELL", "")

	dir := t.TempDir()
	history := writeTempHistoryFile(t, dir, "bash.history")
	runRoot := t.TempDir()
	oldwd, err := os.Getwd()
	if err != nil {
		t.Fatalf("Getwd() error = %v", err)
	}
	if err := os.Chdir(runRoot); err != nil {
		t.Fatalf("Chdir(%q) error = %v", runRoot, err)
	}
	t.Cleanup(func() {
		_ = os.Chdir(oldwd)
	})

	fixed := time.Date(2026, 6, 1, 14, 30, 22, 0, time.Local)
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	code := Runner{
		Stdout: &stdout,
		Stderr: &stderr,
		Now:    func() time.Time { return fixed },
	}.Run([]string{
		"--history-file", history,
		"--shell", "bash",
	})

	if code != ExitOK {
		t.Fatalf("Runner.Run(default output) exit code = %d, want %d\nstderr:\n%s", code, ExitOK, stderr.String())
	}
	outputDir := filepath.Join(runRoot, output.DefaultDirectoryName(fixed))
	markdown, err := os.ReadFile(filepath.Join(outputDir, output.ArtifactCheatsheet))
	if err != nil {
		t.Fatalf("ReadFile(cheatsheet) error = %v", err)
	}
	if !strings.Contains(string(markdown), "Generated locally by cmd-mint on 2026-06-01 14:30:22.") {
		t.Fatalf("cheatsheet does not use injected timestamp:\n%s", markdown)
	}
	if !strings.Contains(stdout.String(), filepath.Join(".", output.DefaultDirectoryName(fixed), output.ArtifactCheatsheet)) {
		t.Fatalf("stdout missing deterministic output path:\n%s", stdout.String())
	}
}

func TestParseFlagsDefaults(t *testing.T) {
	result, err := parseFlags(nil, &bytes.Buffer{})
	if err != nil {
		t.Fatalf("parseFlags returned error: %v", err)
	}

	if result.Options.Shell != model.ShellAuto {
		t.Fatalf("Shell = %q, want %q", result.Options.Shell, model.ShellAuto)
	}
	if result.Options.MaxAliases != 25 {
		t.Fatalf("MaxAliases = %d, want 25", result.Options.MaxAliases)
	}
	if result.Options.MinFrequency != 3 {
		t.Fatalf("MinFrequency = %d, want 3", result.Options.MinFrequency)
	}
}

func TestValidationRejectsInvalidShell(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	code := Run([]string{"--shell", "powershell"}, &stdout, &stderr, version.BuildInfo{})

	if code != ExitInvalidArgs {
		t.Fatalf("Run(invalid shell) exit code = %d, want %d", code, ExitInvalidArgs)
	}
	if !strings.Contains(stderr.String(), "--shell must be one of bash, zsh, fish, or auto") {
		t.Fatalf("stderr missing shell validation error: %q", stderr.String())
	}
}

func TestValidationRejectsInvalidFrequencies(t *testing.T) {
	tests := []struct {
		name string
		args []string
		want string
	}{
		{
			name: "zero min frequency",
			args: []string{"--min-frequency", "0"},
			want: "--min-frequency must be >= 1",
		},
		{
			name: "negative max aliases",
			args: []string{"--max-aliases", "-1"},
			want: "--max-aliases must be >= 0",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var stdout bytes.Buffer
			var stderr bytes.Buffer

			code := Run(tt.args, &stdout, &stderr, version.BuildInfo{})

			if code != ExitInvalidArgs {
				t.Fatalf("Run(%v) exit code = %d, want %d", tt.args, code, ExitInvalidArgs)
			}
			if stdout.Len() != 0 {
				t.Fatalf("stdout = %q, want empty", stdout.String())
			}
			if !strings.Contains(stderr.String(), tt.want) {
				t.Fatalf("stderr = %q, want substring %q", stderr.String(), tt.want)
			}
		})
	}
}

func TestValidationRejectsUnreadableHistoryFile(t *testing.T) {
	missing := filepath.Join(t.TempDir(), "missing.history")

	_, err := parseFlags([]string{"--history-file", missing}, &bytes.Buffer{})
	if err == nil {
		t.Fatal("parseFlags returned nil error for missing history file")
	}
	if !strings.Contains(err.Error(), "--history-file") {
		t.Fatalf("error = %q, want --history-file context", err.Error())
	}
}

func TestValidationRejectsOutputDirExistingRegularFile(t *testing.T) {
	dir := t.TempDir()
	outputFile := filepath.Join(dir, "report")
	if err := os.WriteFile(outputFile, []byte("not a directory"), 0o600); err != nil {
		t.Fatalf("WriteFile(%q) error = %v", outputFile, err)
	}

	_, err := parseFlags([]string{"--output-dir", outputFile}, &bytes.Buffer{})
	if err == nil {
		t.Fatal("parseFlags returned nil error for regular-file output dir")
	}
	if !strings.Contains(err.Error(), "must not be an existing regular file") {
		t.Fatalf("error = %q, want existing regular file validation", err.Error())
	}
}

func TestParseFlagsWithEnvExpandsOutputDir(t *testing.T) {
	home := t.TempDir()
	env := discovery.Env{
		Getenv: func(name string) string {
			if name == "CMD_MINT_REPORT_ROOT" {
				return home
			}
			return ""
		},
		HomeDir: func() (string, error) {
			return home, nil
		},
	}

	result, err := parseFlagsWithEnv([]string{"--output-dir", "$CMD_MINT_REPORT_ROOT/report"}, &bytes.Buffer{}, env)
	if err != nil {
		t.Fatalf("parseFlagsWithEnv returned error: %v", err)
	}

	want := filepath.Join(home, "report")
	if result.Options.OutputDir != want {
		t.Fatalf("OutputDir = %q, want %q", result.Options.OutputDir, want)
	}
}

func TestOutputDirExistingRegularFileExitsCode3(t *testing.T) {
	dir := t.TempDir()
	outputFile := filepath.Join(dir, "report")
	if err := os.WriteFile(outputFile, []byte("not a directory"), 0o600); err != nil {
		t.Fatalf("WriteFile(%q) error = %v", outputFile, err)
	}

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	code := Run([]string{"--output-dir", outputFile}, &stdout, &stderr, version.BuildInfo{})

	if code != ExitOutputError {
		t.Fatalf("Run(output regular file) exit code = %d, want %d", code, ExitOutputError)
	}
	if stdout.Len() != 0 {
		t.Fatalf("stdout = %q, want empty", stdout.String())
	}
	if !strings.Contains(stderr.String(), "output error") {
		t.Fatalf("stderr = %q, want output error", stderr.String())
	}
}

func TestRunPipelineWithFixtureHistoriesWritesExpectedFilesAndSummary(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	t.Setenv("HISTFILE", "")
	t.Setenv("SHELL", "")

	dir := t.TempDir()
	outputDir := filepath.Join(dir, "report")
	zshHistory := filepath.Join("..", "..", "internal", "testdata", "histories", "zsh_plain_history")
	bashHistory := filepath.Join("..", "..", "internal", "testdata", "histories", "bash_plain_history")

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	code := Run([]string{
		"--history-file", zshHistory,
		"--history-file", bashHistory,
		"--output-dir", outputDir,
		"--min-frequency", "2",
		"--json",
	}, &stdout, &stderr, version.BuildInfo{})

	if code != ExitOK {
		t.Fatalf("Run(pipeline) exit code = %d, want %d\nstderr:\n%s", code, ExitOK, stderr.String())
	}
	if stderr.Len() != 0 {
		t.Fatalf("stderr = %q, want empty", stderr.String())
	}

	for _, name := range []string{
		output.ArtifactCheatsheet,
		output.ArtifactAliasesSH,
		output.ArtifactReportJSON,
	} {
		if _, err := os.Stat(filepath.Join(outputDir, name)); err != nil {
			t.Fatalf("expected generated file %s: %v", name, err)
		}
	}
	if _, err := os.Stat(filepath.Join(outputDir, output.ArtifactAliasesFish)); !os.IsNotExist(err) {
		t.Fatalf("fish aliases should not be generated for bash/zsh fixtures: %v", err)
	}

	summary := stdout.String()
	for _, want := range []string{
		"cmd-mint: analyzed shell history locally",
		"Sources scanned:",
		"parsed 3 / skipped 0",
		"Safe commands analyzed: 6",
		"Top tools:",
		"git",
		"Top alias suggestions:",
		"alias gs='git status'",
		"Generated:",
		output.ArtifactCheatsheet,
		output.ArtifactAliasesSH,
		output.ArtifactReportJSON,
		"Generated locally from shell history. Review before sharing or committing.",
	} {
		if !strings.Contains(summary, want) {
			t.Fatalf("terminal summary missing %q:\n%s", want, summary)
		}
	}
}

func TestRunPipelineOmitsRawSensitiveCommandsFromSummaryAndArtifacts(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	t.Setenv("HISTFILE", "")
	t.Setenv("SHELL", "")

	dir := t.TempDir()
	history := filepath.Join(dir, "zsh.history")
	sensitive := `curl -H "Authorization: Bearer raw-secret-token" https://example.invalid`
	content := strings.Join([]string{
		"git status",
		"git status",
		"git status",
		sensitive,
	}, "\n") + "\n"
	if err := os.WriteFile(history, []byte(content), 0o600); err != nil {
		t.Fatalf("WriteFile(%q) error = %v", history, err)
	}

	outputDir := filepath.Join(dir, "report")
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	code := Run([]string{
		"--history-file", history,
		"--shell", "zsh",
		"--output-dir", outputDir,
		"--json",
	}, &stdout, &stderr, version.BuildInfo{})

	if code != ExitOK {
		t.Fatalf("Run(sensitive pipeline) exit code = %d, want %d\nstderr:\n%s", code, ExitOK, stderr.String())
	}

	outputs := []string{stdout.String(), stderr.String()}
	for _, name := range []string{output.ArtifactCheatsheet, output.ArtifactAliasesSH, output.ArtifactReportJSON} {
		data, err := os.ReadFile(filepath.Join(outputDir, name))
		if err != nil {
			t.Fatalf("ReadFile(%s) error = %v", name, err)
		}
		outputs = append(outputs, string(data))
	}

	for _, text := range outputs {
		for _, forbidden := range []string{sensitive, "raw-secret-token", "Authorization: Bearer"} {
			if strings.Contains(text, forbidden) {
				t.Fatalf("output contains forbidden sensitive text %q:\n%s", forbidden, text)
			}
		}
	}
	if !strings.Contains(stdout.String(), "Sensitive-looking commands skipped: 1") {
		t.Fatalf("terminal summary missing sensitive aggregate:\n%s", stdout.String())
	}
}

func TestHelpTextIncludesMVPFlagsAndExcludesDeferredFlags(t *testing.T) {
	text := helpText()

	for _, flag := range []string{
		"--history-file",
		"--shell",
		"--output-dir",
		"--max-aliases",
		"--min-frequency",
		"--no-alias-file",
		"--json",
		"--verbose",
		"--version",
		"--help",
	} {
		if !strings.Contains(text, flag) {
			t.Fatalf("help text missing %s:\n%s", flag, text)
		}
	}

	for _, flag := range []string{
		"--interactive",
		"--include-sensitive",
		"--redact-sensitive",
		"--config",
		"--install-aliases",
		"--follow-sourced-config",
		"--cloud",
	} {
		if strings.Contains(text, flag) {
			t.Fatalf("help text contains deferred flag %s:\n%s", flag, text)
		}
	}
}

func writeTempHistoryFile(t *testing.T, dir string, name string) string {
	t.Helper()

	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, []byte("git status\n"), 0o600); err != nil {
		t.Fatalf("WriteFile(%q) error = %v", path, err)
	}
	return path
}
