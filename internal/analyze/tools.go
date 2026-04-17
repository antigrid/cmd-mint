package analyze

import "strings"

type ToolInfo struct {
	Tool       string
	Subcommand string
}

func DetectTool(tokens []string) ToolInfo {
	toolIndex := groupingToolIndex(tokens)
	if toolIndex < 0 || toolIndex >= len(tokens) {
		return ToolInfo{}
	}

	tool := tokens[toolIndex]
	subcommandIndex := nextCommandToken(tokens, toolIndex+1)

	switch tool {
	case "git":
		return ToolInfo{Tool: tool, Subcommand: valueAt(tokens, gitSubcommandIndex(tokens, toolIndex+1))}
	case "docker":
		subcommandIndex = dockerSubcommandIndex(tokens, toolIndex+1)
		if subcommandIndex >= 0 && tokens[subcommandIndex] == "compose" {
			return ToolInfo{Tool: "docker compose", Subcommand: valueAt(tokens, dockerSubcommandIndex(tokens, subcommandIndex+1))}
		}
		return ToolInfo{Tool: tool, Subcommand: valueAt(tokens, subcommandIndex)}
	case "kubectl":
		return ToolInfo{Tool: tool, Subcommand: valueAt(tokens, kubectlSubcommandIndex(tokens, toolIndex+1))}
	case "npm", "pnpm":
		return ToolInfo{Tool: tool, Subcommand: npmLikeSubcommand(tokens, subcommandIndex)}
	case "yarn":
		return ToolInfo{Tool: tool, Subcommand: valueAt(tokens, subcommandIndex)}
	default:
		return ToolInfo{Tool: tool, Subcommand: valueAt(tokens, subcommandIndex)}
	}
}

func groupingToolIndex(tokens []string) int {
	for i := 0; i < len(tokens); i++ {
		token := tokens[i]

		if isAssignment(token) {
			continue
		}

		switch token {
		case "sudo":
			i = skipSudoOptions(tokens, i+1) - 1
			continue
		case "time":
			i = skipFlags(tokens, i+1) - 1
			continue
		case "env":
			i = skipEnvPrefix(tokens, i+1) - 1
			continue
		case "noglob":
			continue
		case "command", "builtin":
			if i+1 < len(tokens) && strings.HasPrefix(tokens[i+1], "-") {
				return i
			}
			continue
		default:
			return i
		}
	}
	return -1
}

func skipSudoOptions(tokens []string, index int) int {
	for index < len(tokens) {
		token := tokens[index]
		if isAssignment(token) {
			index++
			continue
		}
		if !strings.HasPrefix(token, "-") || token == "-" {
			return index
		}
		index++
		if sudoFlagTakesValue(token) && index < len(tokens) {
			index++
		}
	}
	return index
}

func sudoFlagTakesValue(token string) bool {
	switch token {
	case "-u", "--user", "-g", "--group", "-h", "--host", "-p", "--prompt", "-C", "--close-from", "-T", "--command-timeout":
		return true
	default:
		return false
	}
}

func skipEnvPrefix(tokens []string, index int) int {
	for index < len(tokens) {
		token := tokens[index]
		if isAssignment(token) {
			index++
			continue
		}
		if token == "--" {
			return index + 1
		}
		if strings.HasPrefix(token, "-") {
			index++
			if envFlagTakesValue(token) && index < len(tokens) {
				index++
			}
			continue
		}
		return index
	}
	return index
}

func envFlagTakesValue(token string) bool {
	switch token {
	case "-u", "--unset", "-C", "--chdir", "-S", "--split-string":
		return true
	default:
		return false
	}
}

func skipFlags(tokens []string, index int) int {
	for index < len(tokens) && strings.HasPrefix(tokens[index], "-") && tokens[index] != "-" {
		index++
	}
	return index
}

func nextCommandToken(tokens []string, index int) int {
	for index < len(tokens) {
		token := tokens[index]
		if token == "--" {
			index++
			continue
		}
		if strings.HasPrefix(token, "-") && token != "-" {
			index++
			continue
		}
		return index
	}
	return -1
}

func gitSubcommandIndex(tokens []string, index int) int {
	for index < len(tokens) {
		token := tokens[index]
		if token == "--" {
			return -1
		}
		if token == "-C" || token == "-c" || token == "--git-dir" || token == "--work-tree" || token == "--namespace" {
			index += 2
			continue
		}
		if strings.HasPrefix(token, "-") && token != "-" {
			index++
			continue
		}
		return index
	}
	return -1
}

func dockerSubcommandIndex(tokens []string, index int) int {
	for index < len(tokens) {
		token := tokens[index]
		if token == "--" {
			return -1
		}
		if token == "--context" || token == "--config" || token == "-c" || token == "--host" || token == "-H" || token == "--log-level" {
			index += 2
			continue
		}
		if strings.HasPrefix(token, "-") && token != "-" {
			index++
			continue
		}
		return index
	}
	return -1
}

func kubectlSubcommandIndex(tokens []string, index int) int {
	for index < len(tokens) {
		token := tokens[index]
		if token == "--" {
			return -1
		}
		if token == "-n" || token == "--namespace" || token == "--context" || token == "--kubeconfig" || token == "--cluster" || token == "--user" {
			index += 2
			continue
		}
		if strings.HasPrefix(token, "-") && token != "-" {
			index++
			continue
		}
		return index
	}
	return -1
}

func npmLikeSubcommand(tokens []string, index int) string {
	if index < 0 || index >= len(tokens) {
		return ""
	}
	if tokens[index] == "run" || tokens[index] == "run-script" {
		return "run"
	}
	return tokens[index]
}

func valueAt(tokens []string, index int) string {
	if index < 0 || index >= len(tokens) {
		return ""
	}
	return tokens[index]
}

func isAssignment(token string) bool {
	equals := strings.IndexByte(token, '=')
	if equals <= 0 {
		return false
	}

	name := token[:equals]
	for i, r := range name {
		switch {
		case r == '_':
		case r >= 'A' && r <= 'Z':
		case r >= 'a' && r <= 'z':
		case i > 0 && r >= '0' && r <= '9':
		default:
			return false
		}
	}
	return true
}
