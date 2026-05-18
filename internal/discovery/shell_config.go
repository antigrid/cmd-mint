package discovery

import (
	"bufio"
	"os"
	"path/filepath"
	"strings"

	"cmd-mint/internal/model"
)

const unreadableShellConfigWarningCode = "unreadable_shell_config"

type ShellConfigResult struct {
	Aliases  map[string]model.ExistingAliasDefinition
	Warnings []model.Warning
}

type shellConfigCandidate struct {
	path  string
	shell model.Shell
}

func DiscoverShellConfigAliases() ShellConfigResult {
	return DiscoverShellConfigAliasesWithEnv(DefaultEnv())
}

func DiscoverShellConfigAliasesWithEnv(env Env) ShellConfigResult {
	home, _ := env.homeDir()
	result := ShellConfigResult{
		Aliases: make(map[string]model.ExistingAliasDefinition),
	}
	if home == "" {
		return result
	}

	candidates := []shellConfigCandidate{
		{path: filepath.Join(home, ".zshrc"), shell: model.ShellZsh},
		{path: filepath.Join(home, ".bashrc"), shell: model.ShellBash},
		{path: filepath.Join(home, ".bash_aliases"), shell: model.ShellBash},
		{path: filepath.Join(home, ".config", "fish", "config.fish"), shell: model.ShellFish},
	}

	for _, candidate := range candidates {
		definitions, warning := readShellConfigAliases(candidate)
		if warning != nil {
			result.Warnings = append(result.Warnings, *warning)
			continue
		}
		for _, definition := range definitions {
			if _, exists := result.Aliases[definition.Name]; exists {
				continue
			}
			result.Aliases[definition.Name] = definition
		}
	}

	return result
}

func readShellConfigAliases(candidate shellConfigCandidate) ([]model.ExistingAliasDefinition, *model.Warning) {
	info, err := os.Stat(candidate.path)
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil || !info.Mode().IsRegular() {
		return nil, shellConfigWarning(candidate.path)
	}

	file, err := os.Open(candidate.path)
	if err != nil {
		return nil, shellConfigWarning(candidate.path)
	}
	defer file.Close()

	var definitions []model.ExistingAliasDefinition
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		if parsed := parseShellAliasDefinitions(line, candidate); len(parsed) > 0 {
			definitions = append(definitions, parsed...)
			continue
		}
		if candidate.shell == model.ShellFish {
			if definition, ok := parseFishAbbreviation(line, candidate); ok {
				definitions = append(definitions, definition)
			}
		}
	}
	if err := scanner.Err(); err != nil {
		return definitions, shellConfigWarning(candidate.path)
	}

	return definitions, nil
}

func shellConfigWarning(path string) *model.Warning {
	return &model.Warning{
		Code:       unreadableShellConfigWarningCode,
		Message:    "shell config could not be read for alias conflict discovery",
		SourceFile: path,
	}
}

func parseShellAliasDefinitions(line string, candidate shellConfigCandidate) []model.ExistingAliasDefinition {
	if !strings.HasPrefix(line, "alias ") && line != "alias" {
		return nil
	}

	rest := strings.TrimSpace(strings.TrimPrefix(line, "alias"))
	var definitions []model.ExistingAliasDefinition
	for rest != "" {
		rest = strings.TrimLeft(rest, " \t")
		nameEnd := strings.IndexAny(rest, "= \t")
		if nameEnd <= 0 {
			break
		}
		name := rest[:nameEnd]
		if !isAliasConflictName(name) {
			break
		}

		rest = rest[nameEnd:]
		if !strings.HasPrefix(rest, "=") {
			break
		}
		definitions = append(definitions, model.ExistingAliasDefinition{
			Name:        name,
			SourceShell: candidate.shell,
			SourceFile:  candidate.path,
			Kind:        model.ExistingAliasKindShellAlias,
		})
		rest = skipShellWord(strings.TrimPrefix(rest, "="))
	}

	return definitions
}

func parseFishAbbreviation(line string, candidate shellConfigCandidate) (model.ExistingAliasDefinition, bool) {
	if !strings.HasPrefix(line, "abbr ") {
		return model.ExistingAliasDefinition{}, false
	}

	fields := splitShellFields(line)
	if len(fields) < 3 || fields[0] != "abbr" {
		return model.ExistingAliasDefinition{}, false
	}

	addIndex := -1
	for i := 1; i < len(fields); i++ {
		if fields[i] == "-a" || fields[i] == "--add" {
			addIndex = i
			break
		}
	}
	if addIndex == -1 {
		return model.ExistingAliasDefinition{}, false
	}

	for _, field := range fields[addIndex+1:] {
		if field == "--" || strings.HasPrefix(field, "-") {
			continue
		}
		if !isAliasConflictName(field) {
			return model.ExistingAliasDefinition{}, false
		}
		return model.ExistingAliasDefinition{
			Name:        field,
			SourceShell: candidate.shell,
			SourceFile:  candidate.path,
			Kind:        model.ExistingAliasKindFishAbbreviation,
		}, true
	}

	return model.ExistingAliasDefinition{}, false
}

func skipShellWord(value string) string {
	inSingleQuote := false
	inDoubleQuote := false
	escaped := false

	for i, r := range value {
		if escaped {
			escaped = false
			continue
		}
		if r == '\\' {
			escaped = true
			continue
		}
		switch r {
		case '\'':
			if !inDoubleQuote {
				inSingleQuote = !inSingleQuote
			}
		case '"':
			if !inSingleQuote {
				inDoubleQuote = !inDoubleQuote
			}
		case ' ', '\t':
			if !inSingleQuote && !inDoubleQuote {
				return value[i+1:]
			}
		}
	}
	return ""
}

func splitShellFields(line string) []string {
	var fields []string
	var builder strings.Builder
	inSingleQuote := false
	inDoubleQuote := false
	escaped := false

	flush := func() {
		if builder.Len() == 0 {
			return
		}
		fields = append(fields, builder.String())
		builder.Reset()
	}

	for _, r := range line {
		if escaped {
			builder.WriteRune(r)
			escaped = false
			continue
		}
		if r == '\\' {
			escaped = true
			continue
		}
		switch r {
		case '\'':
			if !inDoubleQuote {
				inSingleQuote = !inSingleQuote
				continue
			}
		case '"':
			if !inSingleQuote {
				inDoubleQuote = !inDoubleQuote
				continue
			}
		case ' ', '\t':
			if !inSingleQuote && !inDoubleQuote {
				flush()
				continue
			}
		}
		builder.WriteRune(r)
	}
	flush()

	return fields
}

func isAliasConflictName(name string) bool {
	if name == "" || strings.HasPrefix(name, "-") {
		return false
	}
	for _, r := range name {
		switch {
		case r >= 'a' && r <= 'z':
		case r >= 'A' && r <= 'Z':
		case r >= '0' && r <= '9':
		case r == '_' || r == '-' || r == '.':
		default:
			return false
		}
	}
	return true
}
