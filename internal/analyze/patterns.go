package analyze

import (
	"net/url"
	"sort"
	"strconv"
	"strings"
	"unicode"

	"cmd-mint/internal/model"
)

const maxPatternExamples = 3

type patternAggregate struct {
	pattern  string
	count    int
	examples map[string]int
}

func GeneratePatterns(result AggregateResult) []model.PatternSummary {
	return GeneratePatternsFromCommands(result.Commands)
}

func GeneratePatternsFromCommands(commands []CommandAggregate) []model.PatternSummary {
	patterns := make(map[string]*patternAggregate)

	for _, command := range commands {
		if !patternCommandEligible(command) {
			continue
		}
		pattern, ok := PatternForCommand(command.NormalizedCommand)
		if !ok {
			continue
		}

		aggregate := patterns[pattern]
		if aggregate == nil {
			aggregate = &patternAggregate{
				pattern:  pattern,
				examples: make(map[string]int),
			}
			patterns[pattern] = aggregate
		}
		aggregate.count += command.Count
		aggregate.examples[command.DisplayCommand] += command.Count
	}

	summaries := make([]model.PatternSummary, 0, len(patterns))
	for _, aggregate := range patterns {
		summaries = append(summaries, model.PatternSummary{
			Pattern:  aggregate.pattern,
			Count:    aggregate.count,
			Examples: topPatternExamples(aggregate.examples),
		})
	}

	sort.SliceStable(summaries, func(i, j int) bool {
		if summaries[i].Count != summaries[j].Count {
			return summaries[i].Count > summaries[j].Count
		}
		return summaries[i].Pattern < summaries[j].Pattern
	})

	return summaries
}

func PatternForCommand(normalized string) (string, bool) {
	tokens := Tokenize(normalized)
	if len(tokens) == 0 {
		return "", false
	}

	patternTokens := make([]string, len(tokens))
	copy(patternTokens, tokens)
	changed := false

	for i, token := range tokens {
		if replacement, ok := patternReplacement(tokens, i, token); ok {
			patternTokens[i] = replacement
			changed = true
		}
	}

	if !changed {
		return "", false
	}
	return strings.Join(patternTokens, " "), true
}

func patternCommandEligible(command CommandAggregate) bool {
	if command.NormalizedCommand == "" || command.DisplayCommand == "" {
		return false
	}
	if command.IsMultiline || len(command.RiskFlags) > 0 {
		return false
	}
	for _, reason := range command.ExclusionReasons {
		if reason == model.ExclusionRiskyDestructive ||
			reason == model.ExclusionRiskyProductionAction ||
			isSensitiveExclusion(reason) {
			return false
		}
	}
	return true
}

func patternReplacement(tokens []string, index int, token string) (string, bool) {
	switch {
	case isLongOptionValue(token, "--namespace") && hasTool(tokens, "kubectl"):
		return "--namespace=<namespace>", true
	case isLongOptionValue(token, "--port") && isValidPort(strings.TrimPrefix(token, "--port=")):
		return "--port=<port>", true
	case isURLWithoutCredentials(token):
		return "<url>", true
	case isPortMapping(token):
		return replacePortMapping(token), true
	case isKubernetesNamespaceToken(tokens, index):
		return "<namespace>", true
	case isNPMRunScriptToken(tokens, index):
		return "<script>", true
	case isNumericPortToken(tokens, index):
		return "<port>", true
	case isGitBranchToken(tokens, index):
		return "<branch>", true
	case isDockerServiceToken(tokens, index):
		return "<service>", true
	case isDockerImageToken(tokens, index):
		return "<image>", true
	case isFilePathToken(token):
		return "<path>", true
	default:
		return "", false
	}
}

func topPatternExamples(examples map[string]int) []string {
	type exampleCount struct {
		example string
		count   int
	}

	values := make([]exampleCount, 0, len(examples))
	for example, count := range examples {
		if example == "" {
			continue
		}
		values = append(values, exampleCount{example: example, count: count})
	}

	sort.SliceStable(values, func(i, j int) bool {
		if values[i].count != values[j].count {
			return values[i].count > values[j].count
		}
		return values[i].example < values[j].example
	})

	limit := maxPatternExamples
	if len(values) < limit {
		limit = len(values)
	}
	result := make([]string, 0, limit)
	for i := 0; i < limit; i++ {
		result = append(result, values[i].example)
	}
	return result
}

func isKubernetesNamespaceToken(tokens []string, index int) bool {
	if !hasTool(tokens, "kubectl") || index == 0 {
		return false
	}
	previous := tokens[index-1]
	return previous == "-n" || previous == "--namespace"
}

func isNPMRunScriptToken(tokens []string, index int) bool {
	npmIndex := indexOfTool(tokens, "npm")
	return npmIndex >= 0 &&
		index == npmIndex+2 &&
		tokens[npmIndex+1] == "run" &&
		isSimplePlaceholderToken(tokens[index])
}

func isGitBranchToken(tokens []string, index int) bool {
	gitIndex := indexOfTool(tokens, "git")
	if gitIndex < 0 || index <= gitIndex+1 || indexAfterDoubleDash(tokens, index) {
		return false
	}

	verb := tokens[gitIndex+1]
	if verb != "checkout" && verb != "switch" && verb != "merge" && verb != "rebase" {
		return false
	}
	token := tokens[index]
	if strings.HasPrefix(token, "-") || strings.Contains(token, "=") {
		return false
	}
	if strings.HasPrefix(token, "/") || strings.HasPrefix(token, "./") || strings.HasPrefix(token, "../") || strings.HasPrefix(token, "~/") {
		return false
	}
	if index > gitIndex+2 && !gitBranchFlagAcceptsValue(tokens[index-1]) {
		return false
	}
	return isBranchLikeToken(token)
}

func gitBranchFlagAcceptsValue(flag string) bool {
	return flag == "-b" || flag == "-B" || flag == "--orphan"
}

func isDockerServiceToken(tokens []string, index int) bool {
	dockerIndex := indexOfTool(tokens, "docker")
	if dockerIndex < 0 || dockerIndex+2 >= len(tokens) || tokens[dockerIndex+1] != "compose" {
		return false
	}
	verbIndex := dockerIndex + 2
	verb := tokens[verbIndex]
	switch verb {
	case "logs", "restart", "stop", "rm", "exec", "up":
	default:
		return false
	}
	if index <= verbIndex || strings.HasPrefix(tokens[index], "-") || optionValueToken(tokens, index) {
		return false
	}
	return firstPositionalAfter(tokens, verbIndex+1) == index && isSimplePlaceholderToken(tokens[index])
}

func isDockerImageToken(tokens []string, index int) bool {
	dockerIndex := indexOfTool(tokens, "docker")
	if dockerIndex < 0 || index <= dockerIndex {
		return false
	}
	if dockerIndex+1 < len(tokens) && tokens[dockerIndex+1] == "compose" {
		return false
	}

	verbIndex := dockerIndex + 1
	verb := tokens[verbIndex]
	if (tokens[index-1] == "-t" || tokens[index-1] == "--tag") && verb == "build" {
		return isImageLikeToken(tokens[index])
	}
	if strings.HasPrefix(tokens[index], "--tag=") && verb == "build" {
		return true
	}

	switch verb {
	case "pull", "push", "run", "rmi", "tag":
	default:
		return false
	}
	if strings.HasPrefix(tokens[index], "-") || optionValueToken(tokens, index) {
		return false
	}
	return firstPositionalAfter(tokens, verbIndex+1) == index && isImageLikeToken(tokens[index])
}

func firstPositionalAfter(tokens []string, start int) int {
	for i := start; i < len(tokens); i++ {
		token := tokens[i]
		if strings.HasPrefix(token, "-") {
			if optionConsumesNextValue(token) && i+1 < len(tokens) {
				i++
			}
			continue
		}
		return i
	}
	return -1
}

func optionValueToken(tokens []string, index int) bool {
	if index == 0 {
		return false
	}
	return optionConsumesNextValue(tokens[index-1])
}

func optionConsumesNextValue(option string) bool {
	switch option {
	case "-f", "--file", "-p", "--publish", "--port", "-n", "--namespace", "-t", "--tag", "--name", "--project-name":
		return true
	default:
		return false
	}
}

func isNumericPortToken(tokens []string, index int) bool {
	if !isValidPort(tokens[index]) {
		return false
	}
	if index == 0 {
		return false
	}

	previous := tokens[index-1]
	if previous == "-p" || previous == "--port" || previous == "--listen" || previous == "--listen-port" || previous == "http.server" {
		return true
	}
	return strings.Contains(previous, "port")
}

func isURLWithoutCredentials(token string) bool {
	parsed, err := url.Parse(token)
	return err == nil &&
		(parsed.Scheme == "http" || parsed.Scheme == "https") &&
		parsed.Host != "" &&
		parsed.User == nil
}

func isFilePathToken(token string) bool {
	if token == "" || isURLWithoutCredentials(token) {
		return false
	}
	if strings.HasPrefix(token, "/") || strings.HasPrefix(token, "./") || strings.HasPrefix(token, "../") || strings.HasPrefix(token, "~/") {
		return true
	}
	return strings.Contains(token, "/") && !strings.Contains(token, "://") && !isPortMapping(token)
}

func isPortMapping(token string) bool {
	left, right, ok := strings.Cut(token, ":")
	return ok && isValidPort(left) && isValidPort(right)
}

func replacePortMapping(token string) string {
	left, right, ok := strings.Cut(token, ":")
	if !ok {
		return token
	}
	if isValidPort(left) && isValidPort(right) {
		return "<port>:<port>"
	}
	return token
}

func isValidPort(token string) bool {
	if token == "" {
		return false
	}
	port, err := strconv.Atoi(token)
	return err == nil && port > 0 && port <= 65535
}

func isLongOptionValue(token string, option string) bool {
	return strings.HasPrefix(token, option+"=") && strings.TrimPrefix(token, option+"=") != ""
}

func hasTool(tokens []string, tool string) bool {
	return indexOfTool(tokens, tool) >= 0
}

func indexOfTool(tokens []string, tool string) int {
	for i, token := range tokens {
		if token == tool {
			return i
		}
	}
	return -1
}

func indexAfterDoubleDash(tokens []string, index int) bool {
	for i := 0; i < index && i < len(tokens); i++ {
		if tokens[i] == "--" {
			return true
		}
	}
	return false
}

func isBranchLikeToken(token string) bool {
	if !isSimplePlaceholderToken(token) {
		return false
	}
	if strings.HasSuffix(token, "/") || strings.Contains(token, "//") {
		return false
	}
	return true
}

func isImageLikeToken(token string) bool {
	if token == "" || strings.HasPrefix(token, "-") || strings.HasPrefix(token, "/") || strings.HasPrefix(token, "./") || strings.HasPrefix(token, "../") || strings.HasPrefix(token, "~/") {
		return false
	}
	if strings.Contains(token, "://") || strings.ContainsAny(token, " \t\n\r") {
		return false
	}
	return true
}

func isSimplePlaceholderToken(token string) bool {
	if token == "" || strings.ContainsAny(token, " \t\n\r") {
		return false
	}
	for _, r := range token {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			continue
		}
		switch r {
		case '.', '_', '-', '/', ':':
			continue
		default:
			return false
		}
	}
	return true
}
