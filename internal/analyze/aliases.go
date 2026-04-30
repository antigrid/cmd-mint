package analyze

import (
	"sort"
	"strings"
	"unicode"

	"cmd-mint/internal/model"
)

const (
	minAliasCommandLength = 12
	minAliasSavings       = 6
	projectRepeatMinimum  = 5
	kubectlAliasMinimum   = 10
	maxFallbackAliasLen   = 12
)

type AliasOptions struct {
	MinFrequency    int
	MaxAliases      int
	ExistingAliases map[string]model.ExistingAliasDefinition
}

type AliasGenerationResult struct {
	Suggestions          []model.AliasSuggestion
	AliasFileSuggestions []model.AliasSuggestion
	Exclusions           model.ExclusionSummary
}

type aliasCandidate struct {
	suggestion      model.AliasSuggestion
	score           int
	convention      bool
	projectSpecific bool
}

var conventionAliases = map[string]string{
	"git status":          "gs",
	"git add":             "ga",
	"git commit":          "gc",
	"git checkout":        "gco",
	"git branch":          "gb",
	"git pull":            "gl",
	"git push":            "gp",
	"git log":             "glog",
	"docker ps":           "dps",
	"docker compose":      "dc",
	"docker compose up":   "dcup",
	"docker compose down": "dcdown",
	"kubectl":             "k",
	"kubectl get pods":    "kgp",
	"kubectl get svc":     "kgs",
	"npm run build":       "nrbuild",
	"npm run test":        "nrtest",
	"npm test":            "nt",
	"pnpm run build":      "prbuild",
	"yarn test":           "yt",
}

var commonSystemCommands = map[string]struct{}{
	"alias": {}, "awk": {}, "bg": {}, "builtin": {}, "cat": {}, "cd": {}, "chmod": {}, "chown": {},
	"command": {}, "cp": {}, "curl": {}, "docker": {}, "echo": {}, "emacs": {}, "env": {}, "export": {},
	"false": {}, "fg": {}, "find": {}, "git": {}, "go": {}, "grep": {}, "gunzip": {}, "gzip": {},
	"head": {}, "history": {}, "jobs": {}, "kill": {}, "kubectl": {}, "less": {}, "ls": {}, "make": {},
	"mkdir": {}, "more": {}, "mv": {}, "nano": {}, "npm": {}, "pnpm": {}, "printf": {}, "ps": {},
	"pwd": {}, "rm": {}, "rmdir": {}, "scp": {}, "sed": {}, "source": {}, "ssh": {}, "sudo": {},
	"tail": {}, "tar": {}, "test": {}, "top": {}, "true": {}, "unalias": {}, "unzip": {}, "vi": {},
	"vim": {}, "wget": {}, "xargs": {}, "yarn": {}, "zip": {},
}

func GenerateAliasSuggestions(result AggregateResult, options AliasOptions) AliasGenerationResult {
	return GenerateAliasSuggestionsFromCommands(result.Commands, options)
}

func GenerateAliasSuggestionsFromCommands(commands []CommandAggregate, options AliasOptions) AliasGenerationResult {
	minFrequency := options.MinFrequency
	if minFrequency < 1 {
		minFrequency = 1
	}
	maxAliases := options.MaxAliases
	if maxAliases < 0 {
		maxAliases = 0
	}

	generated := make(map[string]struct{})
	exclusions := model.ExclusionSummary{
		ByReason: make(map[model.ExclusionReason]int),
	}
	var candidates []aliasCandidate

	for _, command := range commands {
		candidate, ok := buildAliasCandidate(command, minFrequency, options.ExistingAliases, generated, &exclusions)
		if !ok {
			continue
		}
		generated[candidate.suggestion.Name] = struct{}{}
		candidates = append(candidates, candidate)
	}

	sortAliasCandidates(candidates)

	suggestions := make([]model.AliasSuggestion, 0, len(candidates))
	fileSuggestions := make([]model.AliasSuggestion, 0, minInt(maxAliases, len(candidates)))
	for _, candidate := range candidates {
		suggestion := candidate.suggestion
		if suggestion.Confidence != model.ConfidenceLow && len(fileSuggestions) < maxAliases {
			suggestion.AliasFileEligible = true
			fileSuggestions = append(fileSuggestions, suggestion)
		}
		suggestions = append(suggestions, suggestion)
	}

	return AliasGenerationResult{
		Suggestions:          suggestions,
		AliasFileSuggestions: fileSuggestions,
		Exclusions:           exclusions,
	}
}

func buildAliasCandidate(command CommandAggregate, minFrequency int, existing map[string]model.ExistingAliasDefinition, generated map[string]struct{}, exclusions *model.ExclusionSummary) (aliasCandidate, bool) {
	if command.Count < minFrequency {
		countAliasExclusion(exclusions, model.ExclusionLowFrequency, &exclusions.LowFrequencyCommandCount)
		return aliasCandidate{}, false
	}
	if aliasHardExcluded(command) {
		return aliasCandidate{}, false
	}

	conventionName, convention := conventionAliases[command.NormalizedCommand]
	if command.NormalizedCommand == "kubectl" && command.Count < kubectlAliasMinimum {
		convention = false
		conventionName = ""
	}

	aliasName := conventionName
	if aliasName == "" {
		aliasName = fallbackAliasName(command.NormalizedCommand)
	}
	if aliasName == "" {
		countAliasExclusion(exclusions, model.ExclusionTooShort, &exclusions.TooShortCommandCount)
		return aliasCandidate{}, false
	}

	if convention && aliasConflicts(aliasName, existing, generated) {
		countAliasConflict(aliasName, exclusions)
		return aliasCandidate{}, false
	}
	if !convention {
		if aliasIsCommonSystemCommand(aliasName) {
			countAliasConflict(aliasName, exclusions)
			return aliasCandidate{}, false
		}
		resolved, ok := resolveFallbackConflict(aliasName, existing, generated)
		if !ok {
			countAliasConflict(aliasName, exclusions)
			return aliasCandidate{}, false
		}
		aliasName = resolved
	}

	savedPerUse := len(command.NormalizedCommand) - len(aliasName)
	if !convention && len(command.NormalizedCommand) < minAliasCommandLength {
		countAliasExclusion(exclusions, model.ExclusionTooShort, &exclusions.TooShortCommandCount)
		return aliasCandidate{}, false
	}
	if !convention && savedPerUse < minAliasSavings {
		countAliasExclusion(exclusions, model.ExclusionLowSavings, nil)
		return aliasCandidate{}, false
	}

	projectSpecific := commandLooksProjectSpecific(command.NormalizedCommand)
	if projectSpecific && command.Count < projectRepeatMinimum {
		countAliasExclusion(exclusions, model.ExclusionLowFrequency, &exclusions.LowFrequencyCommandCount)
		return aliasCandidate{}, false
	}

	confidence := aliasConfidence(command, convention, projectSpecific)
	estimatedTotal := savedPerUse * command.Count
	suggestion := model.AliasSuggestion{
		Name:                 aliasName,
		Command:              command.DisplayCommand,
		Tool:                 command.Tool,
		Frequency:            command.Count,
		EstimatedSavedPerUse: savedPerUse,
		EstimatedSavedTotal:  estimatedTotal,
		Confidence:           confidence,
		Reason:               aliasReason(convention, projectSpecific),
		SourceShells:         append([]model.Shell(nil), command.SourceShells...),
	}
	sortShells(suggestion.SourceShells)

	return aliasCandidate{
		suggestion:      suggestion,
		score:           aliasScore(command.Count, estimatedTotal, convention, confidence, projectSpecific),
		convention:      convention,
		projectSpecific: projectSpecific,
	}, true
}

func aliasHardExcluded(command CommandAggregate) bool {
	if command.NormalizedCommand == "" || command.DisplayCommand == "" {
		return true
	}
	if command.IsMultiline || command.AliasExcluded || len(command.RiskFlags) > 0 {
		return true
	}
	for _, reason := range command.ExclusionReasons {
		switch reason {
		case model.ExclusionRiskyDestructive,
			model.ExclusionRiskyProductionAction,
			model.ExclusionMultiline,
			model.ExclusionParseFailed,
			model.ExclusionUnsupportedShell,
			model.ExclusionUnreadableSource,
			model.ExclusionMalformedEntry,
			model.ExclusionSensitiveSecret,
			model.ExclusionSensitiveCredentialFile,
			model.ExclusionSensitiveDatabaseURL,
			model.ExclusionSensitiveAuthHeader,
			model.ExclusionSensitivePrivateKey,
			model.ExclusionSensitiveClipboardOrKeychain:
			return true
		}
	}
	return false
}

func aliasConfidence(command CommandAggregate, convention bool, projectSpecific bool) model.Confidence {
	if projectSpecific {
		return model.ConfidenceLow
	}
	if convention || command.Count >= 5 {
		return model.ConfidenceHigh
	}
	return model.ConfidenceMedium
}

func aliasReason(convention bool, projectSpecific bool) string {
	switch {
	case convention:
		return "known conservative convention for a frequent exact command"
	case projectSpecific:
		return "frequent exact command with project-specific tokens; review before using"
	default:
		return "deterministic abbreviation for a frequent exact command"
	}
}

func sortAliasCandidates(candidates []aliasCandidate) {
	sort.SliceStable(candidates, func(i, j int) bool {
		left := candidates[i]
		right := candidates[j]
		switch {
		case left.score != right.score:
			return left.score > right.score
		case left.suggestion.Frequency != right.suggestion.Frequency:
			return left.suggestion.Frequency > right.suggestion.Frequency
		case left.suggestion.EstimatedSavedTotal != right.suggestion.EstimatedSavedTotal:
			return left.suggestion.EstimatedSavedTotal > right.suggestion.EstimatedSavedTotal
		case left.suggestion.EstimatedSavedPerUse != right.suggestion.EstimatedSavedPerUse:
			return left.suggestion.EstimatedSavedPerUse > right.suggestion.EstimatedSavedPerUse
		case left.suggestion.Tool != right.suggestion.Tool:
			return left.suggestion.Tool < right.suggestion.Tool
		case left.suggestion.Command != right.suggestion.Command:
			return left.suggestion.Command < right.suggestion.Command
		default:
			return left.suggestion.Name < right.suggestion.Name
		}
	})
}

func fallbackAliasName(command string) string {
	tokens := Tokenize(command)
	if len(tokens) == 0 {
		return ""
	}

	if name := packageRunAlias(tokens); name != "" {
		return name
	}
	if name := dockerComposeAlias(tokens); name != "" {
		return name
	}

	var builder strings.Builder
	switch tokens[0] {
	case "kubectl":
		builder.WriteByte('k')
		appendInitials(&builder, tokens[1:])
	case "git":
		builder.WriteByte('g')
		appendInitials(&builder, tokens[1:])
	case "docker":
		builder.WriteByte('d')
		appendInitials(&builder, tokens[1:])
	default:
		for _, token := range tokens {
			part := aliasTokenPart(token, false)
			if part == "" {
				continue
			}
			if builder.Len() == 0 {
				builder.WriteString(part)
				continue
			}
			builder.WriteString(truncateString(part, 3))
		}
	}

	return trimAliasName(builder.String())
}

func packageRunAlias(tokens []string) string {
	if len(tokens) < 3 {
		return ""
	}
	prefixes := map[string]string{
		"npm":  "nr",
		"pnpm": "pr",
		"yarn": "yr",
	}
	prefix, ok := prefixes[tokens[0]]
	if !ok || tokens[1] != "run" {
		return ""
	}
	script := aliasTokenPart(tokens[2], false)
	if script == "" {
		return ""
	}
	return trimAliasName(prefix + script)
}

func dockerComposeAlias(tokens []string) string {
	if len(tokens) < 3 || tokens[0] != "docker" || tokens[1] != "compose" {
		return ""
	}
	var builder strings.Builder
	builder.WriteString("dc")
	for _, token := range tokens[2:] {
		part := aliasTokenPart(token, true)
		if part == "" {
			continue
		}
		if strings.HasPrefix(token, "-") {
			builder.WriteString(part)
			continue
		}
		builder.WriteString(truncateString(part, 5))
	}
	return trimAliasName(builder.String())
}

func appendInitials(builder *strings.Builder, tokens []string) {
	for _, token := range tokens {
		part := aliasTokenPart(token, true)
		if part == "" {
			continue
		}
		builder.WriteByte(part[0])
	}
}

func aliasTokenPart(token string, keepShortFlags bool) string {
	token = strings.TrimSpace(token)
	if token == "" || token == "--" {
		return ""
	}
	if strings.HasPrefix(token, "-") {
		if !keepShortFlags {
			return ""
		}
		stripped := strings.TrimLeft(token, "-")
		if len(stripped) == 0 || len(stripped) > 3 || strings.Contains(stripped, "=") {
			return ""
		}
		return sanitizeAliasPart(stripped)
	}
	if strings.Contains(token, "=") && isAssignment(token) {
		return ""
	}
	return sanitizeAliasPart(token)
}

func sanitizeAliasPart(value string) string {
	var builder strings.Builder
	for _, r := range strings.ToLower(value) {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') {
			builder.WriteRune(r)
		}
	}
	return builder.String()
}

func trimAliasName(name string) string {
	name = sanitizeAliasPart(name)
	if len(name) < 3 {
		return ""
	}
	return truncateString(name, maxFallbackAliasLen)
}

func truncateString(value string, limit int) string {
	if len(value) <= limit {
		return value
	}
	return value[:limit]
}

func aliasConflicts(name string, existing map[string]model.ExistingAliasDefinition, generated map[string]struct{}) bool {
	if _, ok := existing[name]; ok {
		return true
	}
	if _, ok := generated[name]; ok {
		return true
	}
	return aliasIsCommonSystemCommand(name)
}

func aliasIsCommonSystemCommand(name string) bool {
	if _, ok := commonSystemCommands[name]; ok {
		return true
	}
	return false
}

func resolveFallbackConflict(name string, existing map[string]model.ExistingAliasDefinition, generated map[string]struct{}) (string, bool) {
	if !aliasConflicts(name, existing, generated) {
		return name, true
	}
	for suffix := 2; suffix <= 9; suffix++ {
		candidate := name + string(rune('0'+suffix))
		if len(candidate) > maxFallbackAliasLen {
			return "", false
		}
		if !aliasConflicts(candidate, existing, generated) {
			return candidate, true
		}
	}
	return "", false
}

func commandLooksProjectSpecific(command string) bool {
	tokens := Tokenize(command)
	for _, token := range tokens {
		lower := strings.ToLower(token)
		switch {
		case strings.HasPrefix(lower, "./"), strings.HasPrefix(lower, "../"), strings.HasPrefix(lower, "~/"):
			return true
		case strings.Contains(lower, "/"):
			return true
		case strings.HasPrefix(lower, "http://"), strings.HasPrefix(lower, "https://"):
			return true
		case looksLikeFileName(lower):
			return true
		}
	}
	return false
}

func looksLikeFileName(token string) bool {
	if strings.HasPrefix(token, "-") || strings.HasPrefix(token, ".") {
		return false
	}
	dot := strings.LastIndexByte(token, '.')
	if dot <= 0 || dot == len(token)-1 {
		return false
	}
	ext := token[dot+1:]
	if len(ext) < 2 || len(ext) > 5 {
		return false
	}
	for _, r := range ext {
		if !unicode.IsLetter(r) && !unicode.IsDigit(r) {
			return false
		}
	}
	return true
}

func countAliasConflict(name string, exclusions *model.ExclusionSummary) {
	countAliasExclusion(exclusions, model.ExclusionAliasConflict, nil)
	exclusions.ExistingAliasConflicts = appendUniqueString(exclusions.ExistingAliasConflicts, name)
	sort.Strings(exclusions.ExistingAliasConflicts)
}

func countAliasExclusion(exclusions *model.ExclusionSummary, reason model.ExclusionReason, counter *int) {
	if exclusions.ByReason == nil {
		exclusions.ByReason = make(map[model.ExclusionReason]int)
	}
	exclusions.ByReason[reason]++
	if counter != nil {
		*counter++
	}
}
