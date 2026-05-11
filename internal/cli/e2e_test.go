package cli

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"cmd-mint/internal/output"
	"cmd-mint/internal/version"
)

func TestEndToEndSecurityRegressionScansEveryGeneratedSurface(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("HISTFILE", "")
	t.Setenv("SHELL", "")

	configPath := filepath.Join(home, ".zshrc")
	if err := os.WriteFile(configPath, []byte("alias gs='git status'\n"), 0o600); err != nil {
		t.Fatalf("WriteFile(%q) error = %v", configPath, err)
	}

	histories := e2eFixturePaths(t)
	beforeHistories := readFileBytes(t, histories)
	beforeConfig := readFileBytes(t, []string{configPath})

	outputDir := filepath.Join(t.TempDir(), "report")
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	code := Run([]string{
		"--history-file", histories[0],
		"--history-file", histories[1],
		"--history-file", histories[2],
		"--history-file", histories[0],
		"--shell", "auto",
		"--output-dir", outputDir,
		"--min-frequency", "2",
		"--max-aliases", "3",
		"--json",
		"--verbose",
	}, &stdout, &stderr, version.BuildInfo{})

	if code != ExitOK {
		t.Fatalf("Run(e2e security) exit code = %d, want %d\nstderr:\n%s", code, ExitOK, stderr.String())
	}
	if stderr.Len() != 0 {
		t.Fatalf("stderr = %q, want empty", stderr.String())
	}

	for _, name := range []string{
		output.ArtifactCheatsheet,
		output.ArtifactAliasesSH,
		output.ArtifactAliasesFish,
		output.ArtifactReportJSON,
	} {
		path := filepath.Join(outputDir, name)
		if _, err := os.Stat(path); err != nil {
			t.Fatalf("expected generated artifact %s: %v", name, err)
		}
		assertMode(t, path, 0o600)
	}
	assertMode(t, outputDir, 0o700)

	surfaces := []string{stdout.String(), stderr.String()}
	for _, name := range []string{
		output.ArtifactCheatsheet,
		output.ArtifactAliasesSH,
		output.ArtifactAliasesFish,
		output.ArtifactReportJSON,
	} {
		data, err := os.ReadFile(filepath.Join(outputDir, name))
		if err != nil {
			t.Fatalf("ReadFile(%s) error = %v", name, err)
		}
		surfaces = append(surfaces, string(data))
	}

	for _, text := range surfaces {
		for _, forbidden := range []string{
			"Authorization: Bearer",
			"e2e-raw-secret-token",
			"AWS_SECRET_ACCESS_KEY",
			"e2e-secret",
			"password=e2e-password",
			".env",
			"id_rsa",
		} {
			if strings.Contains(text, forbidden) {
				t.Fatalf("generated surface contains forbidden sensitive text %q:\n%s", forbidden, text)
			}
		}
	}

	summary := stdout.String()
	for _, want := range []string{
		"Sensitive-looking commands skipped: 4",
		"Risky commands excluded from aliases: 2",
		"Alias conflicts skipped: 1",
		"Warnings:",
		output.ArtifactCheatsheet,
		output.ArtifactAliasesSH,
		output.ArtifactAliasesFish,
		output.ArtifactReportJSON,
	} {
		if !strings.Contains(summary, want) {
			t.Fatalf("terminal summary missing %q:\n%s", want, summary)
		}
	}

	aliasSH := surfaces[3]
	aliasFish := surfaces[4]
	for _, aliasText := range []string{aliasSH, aliasFish} {
		for _, forbidden := range []string{"alias gs=", "rm -rf", "kubectl delete"} {
			if strings.Contains(aliasText, forbidden) {
				t.Fatalf("alias artifact contains forbidden alias text %q:\n%s", forbidden, aliasText)
			}
		}
	}

	reportJSON := surfaces[5]
	if !strings.Contains(reportJSON, `"sensitive_commands_skipped": 4`) {
		t.Fatalf("report JSON missing sensitive aggregate:\n%s", reportJSON)
	}

	afterHistories := readFileBytes(t, histories)
	afterConfig := readFileBytes(t, []string{configPath})
	assertBytesUnchanged(t, beforeHistories, afterHistories)
	assertBytesUnchanged(t, beforeConfig, afterConfig)
}

func TestEndToEndDeterministicOutputSnapshot(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("HISTFILE", "")
	t.Setenv("SHELL", "")

	if err := os.WriteFile(filepath.Join(home, ".zshrc"), []byte("alias gs='git status'\n"), 0o600); err != nil {
		t.Fatalf("WriteFile(.zshrc) error = %v", err)
	}

	histories := e2eFixturePaths(t)
	fixtureRoot := e2eFixtureRoot(t)
	var want string
	for i := 0; i < 3; i++ {
		runRoot := t.TempDir()
		outputDir := filepath.Join(runRoot, "report")
		var stdout bytes.Buffer
		var stderr bytes.Buffer
		code := Run([]string{
			"--history-file", histories[0],
			"--history-file", histories[1],
			"--history-file", histories[2],
			"--output-dir", outputDir,
			"--min-frequency", "2",
			"--max-aliases", "5",
			"--json",
		}, &stdout, &stderr, version.BuildInfo{})

		if code != ExitOK {
			t.Fatalf("Run(deterministic %d) exit code = %d, want %d\nstderr:\n%s", i, code, ExitOK, stderr.String())
		}
		got := normalizeE2ESnapshot(t, collectE2ESnapshot(t, outputDir, stdout.String(), stderr.String()), runRoot, fixtureRoot)
		if i == 0 {
			want = got
			continue
		}
		if got != want {
			t.Fatalf("normalized snapshot changed on run %d\nwant:\n%s\ngot:\n%s", i, want, got)
		}
	}
}

func TestEndToEndFlagMatrix(t *testing.T) {
	t.Run("json and no alias file", func(t *testing.T) {
		home := t.TempDir()
		t.Setenv("HOME", home)
		t.Setenv("HISTFILE", "")
		t.Setenv("SHELL", "")

		outputDir := filepath.Join(t.TempDir(), "report")
		var stdout bytes.Buffer
		var stderr bytes.Buffer
		code := Run([]string{
			"--history-file", e2eFixturePath(t, "bash_history"),
			"--output-dir", outputDir,
			"--json",
			"--no-alias-file",
		}, &stdout, &stderr, version.BuildInfo{})

		if code != ExitOK {
			t.Fatalf("Run(--json --no-alias-file) exit code = %d, want %d\nstderr:\n%s", code, ExitOK, stderr.String())
		}
		assertExists(t, filepath.Join(outputDir, output.ArtifactCheatsheet))
		assertExists(t, filepath.Join(outputDir, output.ArtifactReportJSON))
		assertNotExists(t, filepath.Join(outputDir, output.ArtifactAliasesSH))
		assertNotExists(t, filepath.Join(outputDir, output.ArtifactAliasesFish))
	})

	t.Run("max aliases zero keeps only header", func(t *testing.T) {
		home := t.TempDir()
		t.Setenv("HOME", home)
		t.Setenv("HISTFILE", "")
		t.Setenv("SHELL", "")

		outputDir := filepath.Join(t.TempDir(), "report")
		var stdout bytes.Buffer
		var stderr bytes.Buffer
		code := Run([]string{
			"--history-file", e2eFixturePath(t, "bash_history"),
			"--output-dir", outputDir,
			"--max-aliases", "0",
		}, &stdout, &stderr, version.BuildInfo{})

		if code != ExitOK {
			t.Fatalf("Run(--max-aliases 0) exit code = %d, want %d\nstderr:\n%s", code, ExitOK, stderr.String())
		}
		aliases := readText(t, filepath.Join(outputDir, output.ArtifactAliasesSH))
		if got := countAliasLines(aliases); got != 0 {
			t.Fatalf("alias line count = %d, want 0:\n%s", got, aliases)
		}
		if !strings.Contains(aliases, "# No safe alias suggestions were found.") {
			t.Fatalf("alias file missing no-suggestions comment:\n%s", aliases)
		}
	})

	t.Run("min frequency filters alias suggestions", func(t *testing.T) {
		home := t.TempDir()
		t.Setenv("HOME", home)
		t.Setenv("HISTFILE", "")
		t.Setenv("SHELL", "")

		outputDir := filepath.Join(t.TempDir(), "report")
		var stdout bytes.Buffer
		var stderr bytes.Buffer
		code := Run([]string{
			"--history-file", e2eFixturePath(t, "bash_history"),
			"--output-dir", outputDir,
			"--min-frequency", "5",
		}, &stdout, &stderr, version.BuildInfo{})

		if code != ExitOK {
			t.Fatalf("Run(--min-frequency 5) exit code = %d, want %d\nstderr:\n%s", code, ExitOK, stderr.String())
		}
		aliases := readText(t, filepath.Join(outputDir, output.ArtifactAliasesSH))
		if got := countAliasLines(aliases); got != 0 {
			t.Fatalf("alias line count = %d, want 0:\n%s", got, aliases)
		}
	})

	t.Run("shell flag parses custom fish history and emits fish aliases", func(t *testing.T) {
		home := t.TempDir()
		t.Setenv("HOME", home)
		t.Setenv("HISTFILE", "")
		t.Setenv("SHELL", "")

		history := filepath.Join(t.TempDir(), "custom_history")
		data := []byte("- cmd: docker ps\n  when: 1800000300\n- cmd: docker ps\n  when: 1800000301\n- cmd: docker ps\n  when: 1800000302\n")
		if err := os.WriteFile(history, data, 0o600); err != nil {
			t.Fatalf("WriteFile(%q) error = %v", history, err)
		}

		outputDir := filepath.Join(t.TempDir(), "report")
		var stdout bytes.Buffer
		var stderr bytes.Buffer
		code := Run([]string{
			"--history-file", history,
			"--shell", "fish",
			"--output-dir", outputDir,
		}, &stdout, &stderr, version.BuildInfo{})

		if code != ExitOK {
			t.Fatalf("Run(--shell fish) exit code = %d, want %d\nstderr:\n%s", code, ExitOK, stderr.String())
		}
		assertExists(t, filepath.Join(outputDir, output.ArtifactAliasesSH))
		assertExists(t, filepath.Join(outputDir, output.ArtifactAliasesFish))
	})
}

func TestEndToEndUnreadableDefaultHistoryWarnsWithoutCrashing(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("HISTFILE", "")
	t.Setenv("SHELL", "")

	unreadable := filepath.Join(home, ".zsh_history")
	if err := os.WriteFile(unreadable, []byte("git status\n"), 0o600); err != nil {
		t.Fatalf("WriteFile(%q) error = %v", unreadable, err)
	}
	if err := os.Chmod(unreadable, 0); err != nil {
		t.Fatalf("Chmod(%q) error = %v", unreadable, err)
	}
	t.Cleanup(func() {
		_ = os.Chmod(unreadable, 0o600)
	})
	if file, err := os.Open(unreadable); err == nil {
		_ = file.Close()
		t.Skip("chmod 000 file is still readable on this platform")
	}

	readable := filepath.Join(home, ".bash_history")
	if err := os.WriteFile(readable, []byte("git status\ngit status\ngit status\n"), 0o600); err != nil {
		t.Fatalf("WriteFile(%q) error = %v", readable, err)
	}

	outputDir := filepath.Join(t.TempDir(), "report")
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	code := Run([]string{
		"--output-dir", outputDir,
		"--verbose",
	}, &stdout, &stderr, version.BuildInfo{})

	if code != ExitOK {
		t.Fatalf("Run(unreadable default) exit code = %d, want %d\nstderr:\n%s", code, ExitOK, stderr.String())
	}
	if stderr.Len() != 0 {
		t.Fatalf("stderr = %q, want empty", stderr.String())
	}
	for _, want := range []string{
		"Warnings:",
		"parsed 0 / skipped 1",
		"parsed 3 / skipped 0",
	} {
		if !strings.Contains(stdout.String(), want) {
			t.Fatalf("summary missing %q:\n%s", want, stdout.String())
		}
	}
	markdown := readText(t, filepath.Join(outputDir, output.ArtifactCheatsheet))
	if !strings.Contains(markdown, "history source could not be parsed") {
		t.Fatalf("markdown missing unreadable-source warning:\n%s", markdown)
	}
}

// Run with:
// go test -run '^$' -bench BenchmarkRunLargeSyntheticHistory100000 -benchmem ./internal/cli
func BenchmarkRunLargeSyntheticHistory100000(b *testing.B) {
	home := b.TempDir()
	b.Setenv("HOME", home)
	b.Setenv("HISTFILE", "")
	b.Setenv("SHELL", "")

	history := filepath.Join(home, "large_bash_history")
	writeLargeBenchmarkHistory(b, history, 100_000)

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		outputDir := filepath.Join(home, fmt.Sprintf("report-%d", i))
		var stdout bytes.Buffer
		var stderr bytes.Buffer
		code := Run([]string{
			"--history-file", history,
			"--shell", "bash",
			"--output-dir", outputDir,
			"--min-frequency", "3",
			"--max-aliases", "10",
			"--json",
		}, &stdout, &stderr, version.BuildInfo{})
		if code != ExitOK {
			b.Fatalf("Run(large history) exit code = %d, want %d\nstderr:\n%s", code, ExitOK, stderr.String())
		}
	}
}

func e2eFixturePaths(t *testing.T) []string {
	t.Helper()
	return []string{
		e2eFixturePath(t, "zsh_history"),
		e2eFixturePath(t, "bash_history"),
		e2eFixturePath(t, "fish_history"),
	}
}

func e2eFixturePath(t *testing.T, name string) string {
	t.Helper()
	return filepath.Join(e2eFixtureRoot(t), name)
}

func e2eFixtureRoot(t *testing.T) string {
	t.Helper()
	path := filepath.Join("..", "..", "internal", "testdata", "e2e", "mvp")
	abs, err := filepath.Abs(path)
	if err != nil {
		t.Fatalf("Abs(%q) error = %v", path, err)
	}
	return filepath.Clean(abs)
}

func collectE2ESnapshot(t *testing.T, outputDir string, stdout string, stderr string) string {
	t.Helper()

	var snapshot strings.Builder
	snapshot.WriteString("STDOUT\n")
	snapshot.WriteString(stdout)
	snapshot.WriteString("\nSTDERR\n")
	snapshot.WriteString(stderr)
	for _, name := range []string{
		output.ArtifactCheatsheet,
		output.ArtifactAliasesSH,
		output.ArtifactAliasesFish,
		output.ArtifactReportJSON,
	} {
		snapshot.WriteString("\n")
		snapshot.WriteString(name)
		snapshot.WriteString("\n")
		snapshot.WriteString(readText(t, filepath.Join(outputDir, name)))
	}
	return snapshot.String()
}

func normalizeE2ESnapshot(t *testing.T, snapshot string, runRoot string, fixtureRoot string) string {
	t.Helper()

	normalized := strings.ReplaceAll(snapshot, filepath.Clean(runRoot), "<RUN>")
	normalized = strings.ReplaceAll(normalized, fixtureRoot, "<FIXTURE>")
	normalized = regexp.MustCompile(`Generated locally by cmd-mint on [0-9]{4}-[0-9]{2}-[0-9]{2} [0-9]{2}:[0-9]{2}:[0-9]{2}\.`).ReplaceAllString(normalized, "Generated locally by cmd-mint on <TIME>.")
	normalized = regexp.MustCompile(`"generated_at": "[^"]+"`).ReplaceAllString(normalized, `"generated_at": "<TIME>"`)
	return normalized
}

func readFileBytes(t *testing.T, paths []string) map[string][]byte {
	t.Helper()

	contents := make(map[string][]byte, len(paths))
	for _, path := range paths {
		data, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("ReadFile(%q) error = %v", path, err)
		}
		contents[path] = append([]byte(nil), data...)
	}
	return contents
}

func assertBytesUnchanged(t *testing.T, before map[string][]byte, after map[string][]byte) {
	t.Helper()

	if len(before) != len(after) {
		t.Fatalf("file count changed: before %d after %d", len(before), len(after))
	}
	for path, beforeData := range before {
		afterData, ok := after[path]
		if !ok {
			t.Fatalf("file %q missing after run", path)
		}
		if !bytes.Equal(beforeData, afterData) {
			t.Fatalf("file %q changed after run", path)
		}
	}
}

func readText(t *testing.T, path string) string {
	t.Helper()

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile(%q) error = %v", path, err)
	}
	return string(data)
}

func assertExists(t *testing.T, path string) {
	t.Helper()

	if _, err := os.Stat(path); err != nil {
		t.Fatalf("expected %q to exist: %v", path, err)
	}
}

func assertNotExists(t *testing.T, path string) {
	t.Helper()

	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatalf("expected %q not to exist, stat error = %v", path, err)
	}
}

func assertMode(t *testing.T, path string, want os.FileMode) {
	t.Helper()

	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("Stat(%q) error = %v", path, err)
	}
	if got := info.Mode().Perm(); got != want {
		t.Fatalf("mode(%q) = %o, want %o", path, got, want)
	}
}

func countAliasLines(text string) int {
	count := 0
	for _, line := range strings.Split(text, "\n") {
		if strings.HasPrefix(line, "alias ") {
			count++
		}
	}
	return count
}

func writeLargeBenchmarkHistory(b *testing.B, path string, entries int) {
	b.Helper()

	var builder strings.Builder
	builder.Grow(entries * len("git status\n"))
	commands := []string{
		"git status\n",
		"docker ps\n",
		"npm run build\n",
		"kubectl get pods -n dev\n",
	}
	for i := 0; i < entries; i++ {
		builder.WriteString(commands[i%len(commands)])
	}
	if err := os.WriteFile(path, []byte(builder.String()), 0o600); err != nil {
		b.Fatalf("WriteFile(%q) error = %v", path, err)
	}
}
