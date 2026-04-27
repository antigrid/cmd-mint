package discovery

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"cmd-mint/internal/model"
)

func TestDiscoverShellConfigAliasesDetectsSimpleBashAndZshAliases(t *testing.T) {
	home := tempHome(t)
	zshrc := writeConfig(t, home, ".zshrc", `
# comment
alias gs='git status'
alias gco="git checkout"
source ~/.zsh_aliases
`)
	bashrc := writeConfig(t, home, ".bashrc", `alias ll='ls -la'`)
	bashAliases := writeConfig(t, home, ".bash_aliases", `alias gst='git stash'`)

	result := DiscoverShellConfigAliases()

	want := map[string]model.ExistingAliasDefinition{
		"gs": {
			Name:        "gs",
			SourceShell: model.ShellZsh,
			SourceFile:  zshrc,
			Kind:        model.ExistingAliasKindShellAlias,
		},
		"gco": {
			Name:        "gco",
			SourceShell: model.ShellZsh,
			SourceFile:  zshrc,
			Kind:        model.ExistingAliasKindShellAlias,
		},
		"ll": {
			Name:        "ll",
			SourceShell: model.ShellBash,
			SourceFile:  bashrc,
			Kind:        model.ExistingAliasKindShellAlias,
		},
		"gst": {
			Name:        "gst",
			SourceShell: model.ShellBash,
			SourceFile:  bashAliases,
			Kind:        model.ExistingAliasKindShellAlias,
		},
	}
	if !reflect.DeepEqual(result.Aliases, want) {
		t.Fatalf("Aliases = %#v, want %#v", result.Aliases, want)
	}
	if len(result.Warnings) != 0 {
		t.Fatalf("Warnings = %#v, want none", result.Warnings)
	}
}

func TestDiscoverShellConfigAliasesDetectsFishAliasesAndAbbreviations(t *testing.T) {
	home := tempHome(t)
	fishConfig := writeConfig(t, home, ".config/fish/config.fish", `
alias gs='git status'
abbr -a gco 'git checkout'
abbr --add dps 'docker ps'
`)

	result := DiscoverShellConfigAliases()

	want := map[string]model.ExistingAliasDefinition{
		"gs": {
			Name:        "gs",
			SourceShell: model.ShellFish,
			SourceFile:  fishConfig,
			Kind:        model.ExistingAliasKindShellAlias,
		},
		"gco": {
			Name:        "gco",
			SourceShell: model.ShellFish,
			SourceFile:  fishConfig,
			Kind:        model.ExistingAliasKindFishAbbreviation,
		},
		"dps": {
			Name:        "dps",
			SourceShell: model.ShellFish,
			SourceFile:  fishConfig,
			Kind:        model.ExistingAliasKindFishAbbreviation,
		},
	}
	if !reflect.DeepEqual(result.Aliases, want) {
		t.Fatalf("Aliases = %#v, want %#v", result.Aliases, want)
	}
}

func TestDiscoverShellConfigAliasesDoesNotReadSourcedFiles(t *testing.T) {
	home := tempHome(t)
	writeConfig(t, home, ".zshrc", `
. ~/.zsh_aliases
source ~/.more_zsh_aliases
alias gs='git status'
`)
	writeConfig(t, home, ".zsh_aliases", `alias sourced='should not be discovered'`)
	writeConfig(t, home, ".more_zsh_aliases", `alias more='should not be discovered'`)

	result := DiscoverShellConfigAliases()

	if _, ok := result.Aliases["gs"]; !ok {
		t.Fatalf("Aliases missing gs: %#v", result.Aliases)
	}
	for _, name := range []string{"sourced", "more"} {
		if _, ok := result.Aliases[name]; ok {
			t.Fatalf("Aliases contains sourced alias %q: %#v", name, result.Aliases)
		}
	}
}

func TestDiscoverShellConfigAliasesDoesNotModifyConfigFiles(t *testing.T) {
	home := tempHome(t)
	files := []string{
		writeConfig(t, home, ".zshrc", `alias gs='git status'`),
		writeConfig(t, home, ".bashrc", `alias ll='ls -la'`),
		writeConfig(t, home, ".bash_aliases", `alias gst='git stash'`),
		writeConfig(t, home, ".config/fish/config.fish", `abbr -a gco 'git checkout'`),
	}
	before := readFiles(t, files)

	_ = DiscoverShellConfigAliases()

	after := readFiles(t, files)
	if !reflect.DeepEqual(after, before) {
		t.Fatalf("config files changed after discovery\nbefore: %#v\nafter: %#v", before, after)
	}
}

func writeConfig(t *testing.T, home string, name string, data string) string {
	t.Helper()

	path := filepath.Join(home, filepath.FromSlash(name))
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatalf("MkdirAll(%q) error = %v", filepath.Dir(path), err)
	}
	if err := os.WriteFile(path, []byte(data), 0o600); err != nil {
		t.Fatalf("WriteFile(%q) error = %v", path, err)
	}
	absPath, err := filepath.Abs(path)
	if err != nil {
		t.Fatalf("Abs(%q) error = %v", path, err)
	}
	return filepath.Clean(absPath)
}

func readFiles(t *testing.T, paths []string) map[string]string {
	t.Helper()

	contents := make(map[string]string, len(paths))
	for _, path := range paths {
		data, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("ReadFile(%q) error = %v", path, err)
		}
		contents[path] = string(data)
	}
	return contents
}
