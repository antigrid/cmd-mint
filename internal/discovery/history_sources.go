package discovery

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"cmd-mint/internal/model"
)

var ErrNoUsableSources = errors.New("no usable history sources")

type Options struct {
	HistoryFiles []string
	Shell        model.Shell
}

type Result struct {
	Sources []model.HistorySource
}

type sourceKind int

const (
	sourceDefault sourceKind = iota
	sourceHistfile
	sourceExplicit
)

type candidate struct {
	path  string
	shell model.Shell
}

func DiscoverHistorySources(opts Options) (Result, error) {
	home, _ := os.UserHomeDir()
	var candidates []candidate

	if home != "" {
		candidates = append(candidates,
			candidate{path: filepath.Join(home, ".zsh_history"), shell: model.ShellZsh},
			candidate{path: filepath.Join(home, ".bash_history"), shell: model.ShellBash},
			candidate{path: filepath.Join(home, ".local", "share", "fish", "fish_history"), shell: model.ShellFish},
		)
	}

	if histfile := strings.TrimSpace(os.Getenv("HISTFILE")); histfile != "" {
		path := expandHome(histfile, home)
		candidates = append(candidates, candidate{
			path:  path,
			shell: inferShell(path, opts.Shell, os.Getenv("SHELL"), sourceHistfile),
		})
	}

	for _, explicit := range opts.HistoryFiles {
		path := expandHome(explicit, home)
		candidates = append(candidates, candidate{
			path:  path,
			shell: inferShell(path, opts.Shell, os.Getenv("SHELL"), sourceExplicit),
		})
	}

	result := Result{
		Sources: make([]model.HistorySource, 0, len(candidates)),
	}
	seen := make(map[string]struct{}, len(candidates))
	for _, candidate := range candidates {
		source, key, ok := sourceFromCandidate(candidate)
		if !ok {
			continue
		}
		if _, exists := seen[key]; exists {
			continue
		}
		seen[key] = struct{}{}
		result.Sources = append(result.Sources, source)
	}

	if len(result.Sources) == 0 {
		return result, ErrNoUsableSources
	}
	return result, nil
}

func NoSourcesMessage() string {
	return "No supported shell history files found.\n" +
		"Checked: ~/.zsh_history, ~/.bash_history, ~/.local/share/fish/fish_history, HISTFILE.\n" +
		"Use --history-file PATH to analyze a custom file.\n"
}

func sourceFromCandidate(candidate candidate) (model.HistorySource, string, bool) {
	absPath, ok := absoluteCleanPath(candidate.path)
	if !ok || !isExistingRegularFile(absPath) {
		return model.HistorySource{}, "", false
	}

	return model.HistorySource{
		SourceShell: candidate.shell,
		SourceFile:  absPath,
	}, canonicalKey(absPath), true
}

func expandHome(path string, home string) string {
	if path == "~" {
		if home == "" {
			return path
		}
		return home
	}
	if strings.HasPrefix(path, "~/") {
		if home == "" {
			return path
		}
		return filepath.Join(home, path[2:])
	}
	return path
}

func absoluteCleanPath(path string) (string, bool) {
	if strings.TrimSpace(path) == "" {
		return "", false
	}
	absPath, err := filepath.Abs(path)
	if err != nil {
		return filepath.Clean(path), true
	}
	return filepath.Clean(absPath), true
}

func canonicalKey(path string) string {
	canonical, err := filepath.EvalSymlinks(path)
	if err == nil {
		return filepath.Clean(canonical)
	}
	if absPath, ok := absoluteCleanPath(path); ok {
		return absPath
	}
	return filepath.Clean(path)
}

func isExistingRegularFile(path string) bool {
	info, err := os.Stat(path)
	if err != nil || !info.Mode().IsRegular() {
		return false
	}
	return true
}

func inferShell(path string, fallback model.Shell, currentShell string, kind sourceKind) model.Shell {
	if kind == sourceExplicit && fallback != model.ShellAuto {
		return fallback
	}

	if kind == sourceHistfile {
		if shell := inferCurrentShell(currentShell); shell != model.ShellAuto {
			return shell
		}
	}

	if shell := inferShellFromPath(path); shell != model.ShellAuto {
		return shell
	}

	if fallback != "" {
		return fallback
	}
	return model.ShellAuto
}

func inferShellFromPath(path string) model.Shell {
	lower := strings.ToLower(filepath.ToSlash(path))
	base := filepath.Base(lower)

	switch {
	case base == ".zsh_history", strings.Contains(base, "zsh"):
		return model.ShellZsh
	case base == ".bash_history", strings.Contains(base, "bash"):
		return model.ShellBash
	case base == "fish_history", strings.Contains(base, "fish"), strings.Contains(lower, "/fish/"):
		return model.ShellFish
	default:
		return model.ShellAuto
	}
}

func inferCurrentShell(shellPath string) model.Shell {
	base := strings.ToLower(filepath.Base(strings.TrimSpace(shellPath)))
	switch base {
	case "bash":
		return model.ShellBash
	case "zsh":
		return model.ShellZsh
	case "fish":
		return model.ShellFish
	default:
		return model.ShellAuto
	}
}

func ValidateReadableHistoryFile(path string) error {
	home, _ := os.UserHomeDir()
	expanded := expandHome(path, home)
	absPath, ok := absoluteCleanPath(expanded)
	if !ok {
		return fmt.Errorf("path must not be empty")
	}

	info, err := os.Stat(absPath)
	if err != nil {
		return err
	}
	if !info.Mode().IsRegular() {
		return fmt.Errorf("must be a readable regular file")
	}

	file, err := os.Open(absPath)
	if err != nil {
		return err
	}
	return file.Close()
}
