package discovery

import (
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"cmd-mint/internal/model"
)

func TestDefaultSourceDiscoveryIncludesOnlyPresentReadableFiles(t *testing.T) {
	home := tempHome(t)
	zsh := writeHistory(t, home, ".zsh_history")
	fish := writeHistory(t, home, ".local/share/fish/fish_history")

	result, err := DiscoverHistorySources(Options{Shell: model.ShellAuto})
	if err != nil {
		t.Fatalf("DiscoverHistorySources returned error: %v", err)
	}

	want := []model.HistorySource{
		{SourceShell: model.ShellZsh, SourceFile: zsh},
		{SourceShell: model.ShellFish, SourceFile: fish},
	}
	if !reflect.DeepEqual(result.Sources, want) {
		t.Fatalf("Sources = %#v, want %#v", result.Sources, want)
	}
}

func TestExplicitFilesAreAddedInAdditionToDefaults(t *testing.T) {
	home := tempHome(t)
	zsh := writeHistory(t, home, ".zsh_history")
	explicit := writeHistory(t, home, "old_bash_history")

	result, err := DiscoverHistorySources(Options{
		HistoryFiles: []string{explicit},
		Shell:        model.ShellAuto,
	})
	if err != nil {
		t.Fatalf("DiscoverHistorySources returned error: %v", err)
	}

	want := []model.HistorySource{
		{SourceShell: model.ShellZsh, SourceFile: zsh},
		{SourceShell: model.ShellBash, SourceFile: explicit},
	}
	if !reflect.DeepEqual(result.Sources, want) {
		t.Fatalf("Sources = %#v, want %#v", result.Sources, want)
	}
}

func TestExplicitShellOverridesFileNameHints(t *testing.T) {
	home := tempHome(t)
	explicit := writeHistory(t, home, "old_bash_history")

	result, err := DiscoverHistorySources(Options{
		HistoryFiles: []string{explicit},
		Shell:        model.ShellZsh,
	})
	if err != nil {
		t.Fatalf("DiscoverHistorySources returned error: %v", err)
	}

	want := []model.HistorySource{
		{SourceShell: model.ShellZsh, SourceFile: explicit},
	}
	if !reflect.DeepEqual(result.Sources, want) {
		t.Fatalf("Sources = %#v, want %#v", result.Sources, want)
	}
}

func TestHistfileUsesCurrentShellWhenReliable(t *testing.T) {
	home := tempHome(t)
	histfile := writeHistory(t, home, "history")
	t.Setenv("HISTFILE", histfile)
	t.Setenv("SHELL", "/bin/fish")

	result, err := DiscoverHistorySources(Options{Shell: model.ShellAuto})
	if err != nil {
		t.Fatalf("DiscoverHistorySources returned error: %v", err)
	}

	want := []model.HistorySource{
		{SourceShell: model.ShellFish, SourceFile: histfile},
	}
	if !reflect.DeepEqual(result.Sources, want) {
		t.Fatalf("Sources = %#v, want %#v", result.Sources, want)
	}
}

func TestDuplicatePathsAreCollapsedByCanonicalPath(t *testing.T) {
	home := tempHome(t)
	defaultPath := writeHistory(t, home, ".zsh_history")
	linkPath := filepath.Join(home, "linked_history")
	if err := os.Symlink(defaultPath, linkPath); err != nil {
		t.Skipf("Symlink not available: %v", err)
	}

	result, err := DiscoverHistorySources(Options{
		HistoryFiles: []string{linkPath, defaultPath},
		Shell:        model.ShellAuto,
	})
	if err != nil {
		t.Fatalf("DiscoverHistorySources returned error: %v", err)
	}

	want := []model.HistorySource{
		{SourceShell: model.ShellZsh, SourceFile: defaultPath},
	}
	if !reflect.DeepEqual(result.Sources, want) {
		t.Fatalf("Sources = %#v, want %#v", result.Sources, want)
	}
}

func TestNoSourcesProducesNoUsableSourcesError(t *testing.T) {
	tempHome(t)

	result, err := DiscoverHistorySources(Options{Shell: model.ShellAuto})
	if !errors.Is(err, ErrNoUsableSources) {
		t.Fatalf("DiscoverHistorySources error = %v, want ErrNoUsableSources", err)
	}
	if len(result.Sources) != 0 {
		t.Fatalf("Sources = %#v, want none", result.Sources)
	}
}

func TestTildeHistoryFilePathIsExpanded(t *testing.T) {
	home := tempHome(t)
	explicit := writeHistory(t, home, "custom_zsh_history")

	result, err := DiscoverHistorySources(Options{
		HistoryFiles: []string{"~/custom_zsh_history"},
		Shell:        model.ShellAuto,
	})
	if err != nil {
		t.Fatalf("DiscoverHistorySources returned error: %v", err)
	}

	want := []model.HistorySource{
		{SourceShell: model.ShellZsh, SourceFile: explicit},
	}
	if !reflect.DeepEqual(result.Sources, want) {
		t.Fatalf("Sources = %#v, want %#v", result.Sources, want)
	}
}

func TestEnvironmentVariableHistoryFilePathIsExpanded(t *testing.T) {
	home := tempHome(t)
	explicit := writeHistory(t, home, "custom_zsh_history")
	t.Setenv("CMD_MINT_HISTORY", explicit)

	result, err := DiscoverHistorySources(Options{
		HistoryFiles: []string{"$CMD_MINT_HISTORY"},
		Shell:        model.ShellAuto,
	})
	if err != nil {
		t.Fatalf("DiscoverHistorySources returned error: %v", err)
	}

	want := []model.HistorySource{
		{SourceShell: model.ShellZsh, SourceFile: explicit},
	}
	if !reflect.DeepEqual(result.Sources, want) {
		t.Fatalf("Sources = %#v, want %#v", result.Sources, want)
	}
}

func TestDiscoveryUsesInjectedEnvironment(t *testing.T) {
	home := t.TempDir()
	explicit := writeHistory(t, home, "history")
	env := Env{
		Getenv: func(name string) string {
			switch name {
			case "HISTFILE":
				return "$CMD_MINT_HISTORY"
			case "CMD_MINT_HISTORY":
				return explicit
			case "SHELL":
				return "/bin/fish"
			default:
				return ""
			}
		},
		HomeDir: func() (string, error) {
			return home, nil
		},
	}

	result, err := DiscoverHistorySources(Options{Shell: model.ShellAuto, Env: env})
	if err != nil {
		t.Fatalf("DiscoverHistorySources returned error: %v", err)
	}

	want := []model.HistorySource{
		{SourceShell: model.ShellFish, SourceFile: explicit},
	}
	if !reflect.DeepEqual(result.Sources, want) {
		t.Fatalf("Sources = %#v, want %#v", result.Sources, want)
	}
}

func tempHome(t *testing.T) string {
	t.Helper()

	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("HISTFILE", "")
	t.Setenv("SHELL", "")
	return home
}

func writeHistory(t *testing.T, home string, name string) string {
	t.Helper()

	path := filepath.Join(home, filepath.FromSlash(name))
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatalf("MkdirAll(%q) error = %v", filepath.Dir(path), err)
	}
	if err := os.WriteFile(path, []byte("git status\n"), 0o600); err != nil {
		t.Fatalf("WriteFile(%q) error = %v", path, err)
	}
	absPath, err := filepath.Abs(path)
	if err != nil {
		t.Fatalf("Abs(%q) error = %v", path, err)
	}
	return filepath.Clean(absPath)
}
