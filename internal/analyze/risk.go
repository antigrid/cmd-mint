package analyze

import (
	"regexp"
	"strings"

	"cmd-mint/internal/model"
)

type RiskClassification struct {
	Flags   []model.RiskFlag
	Reasons []model.ExclusionReason
}

func (classification RiskClassification) Risky() bool {
	return len(classification.Flags) > 0 || len(classification.Reasons) > 0
}

var destructiveSQLRE = regexp.MustCompile(`(?i)\b(drop\s+database|truncate\s+table)\b`)

func ClassifyRisk(command string, tokens []string) RiskClassification {
	classification := RiskClassification{}
	check := func(flag model.RiskFlag, reason model.ExclusionReason, matched bool) {
		if !matched {
			return
		}
		classification.Flags = appendUniqueRiskFlag(classification.Flags, flag)
		classification.Reasons = appendUniqueReason(classification.Reasons, reason)
	}

	check(model.RiskDestructive, model.ExclusionRiskyDestructive, destructiveSQLRE.MatchString(command))
	check(model.RiskDestructive, model.ExclusionRiskyDestructive, commandLooksDestructive(tokens))
	check(model.RiskProductionAction, model.ExclusionRiskyProductionAction, combinesProductionAndRiskyAction(tokens))

	return classification
}

func IsRiskyRecord(record model.CommandRecord) bool {
	if len(record.RiskFlags) > 0 {
		return true
	}
	for _, reason := range record.ExclusionReasons {
		if reason == model.ExclusionRiskyDestructive || reason == model.ExclusionRiskyProductionAction {
			return true
		}
	}
	return false
}

func commandLooksDestructive(tokens []string) bool {
	commandIndex := groupingToolIndex(tokens)
	if commandIndex < 0 || commandIndex >= len(tokens) {
		return false
	}

	switch tokens[commandIndex] {
	case "rm":
		return hasSudoPrefix(tokens, commandIndex) || rmHasRecursiveForce(tokens[commandIndex+1:])
	case "chmod", "chown":
		return hasRecursiveFlag(tokens[commandIndex+1:])
	case "kill":
		return hasKill9(tokens[commandIndex+1:])
	case "docker":
		return dockerSystemPrune(tokens, commandIndex)
	case "kubectl":
		return valueAt(tokens, kubectlSubcommandIndex(tokens, commandIndex+1)) == "delete"
	case "terraform":
		return valueAt(tokens, nextCommandToken(tokens, commandIndex+1)) == "destroy"
	default:
		return false
	}
}

func hasSudoPrefix(tokens []string, commandIndex int) bool {
	for i := 0; i < commandIndex; i++ {
		if tokens[i] == "sudo" {
			return true
		}
	}
	return false
}

func rmHasRecursiveForce(tokens []string) bool {
	hasRecursive := false
	hasForce := false

	for _, token := range tokens {
		if token == "--" {
			break
		}
		if !strings.HasPrefix(token, "-") || token == "-" {
			continue
		}
		switch token {
		case "--recursive", "--dir":
			hasRecursive = true
		case "--force":
			hasForce = true
		default:
			if strings.HasPrefix(token, "--") {
				continue
			}
			for _, r := range token[1:] {
				switch r {
				case 'r', 'R':
					hasRecursive = true
				case 'f':
					hasForce = true
				}
			}
		}
	}

	return hasRecursive && hasForce
}

func hasRecursiveFlag(tokens []string) bool {
	for _, token := range tokens {
		if token == "--" {
			return false
		}
		if token == "-R" || token == "--recursive" {
			return true
		}
	}
	return false
}

func hasKill9(tokens []string) bool {
	for i, token := range tokens {
		switch token {
		case "--":
			return false
		case "-9", "-KILL", "-SIGKILL", "--signal=KILL", "--signal=SIGKILL":
			return true
		case "-s", "--signal":
			if i+1 < len(tokens) {
				next := strings.ToUpper(tokens[i+1])
				if next == "KILL" || next == "SIGKILL" || next == "9" {
					return true
				}
			}
		}
	}
	return false
}

func dockerSystemPrune(tokens []string, commandIndex int) bool {
	subcommandIndex := dockerSubcommandIndex(tokens, commandIndex+1)
	if valueAt(tokens, subcommandIndex) != "system" {
		return false
	}
	actionIndex := dockerSubcommandIndex(tokens, subcommandIndex+1)
	return valueAt(tokens, actionIndex) == "prune"
}

func combinesProductionAndRiskyAction(tokens []string) bool {
	hasProduction := false
	hasAction := false

	for _, token := range tokens {
		for _, word := range tokenWords(token) {
			if isProductionToken(word) {
				hasProduction = true
			}
			if isRiskyActionToken(word) {
				hasAction = true
			}
			if hasProduction && hasAction {
				return true
			}
		}
	}
	return false
}

func tokenWords(token string) []string {
	lower := strings.ToLower(token)
	return strings.FieldsFunc(lower, func(r rune) bool {
		return (r < 'a' || r > 'z') && (r < '0' || r > '9')
	})
}

func isProductionToken(token string) bool {
	switch token {
	case "prod", "production", "live", "mainnet", "customer":
		return true
	default:
		return false
	}
}

func isRiskyActionToken(token string) bool {
	switch token {
	case "deploy", "delete", "destroy", "remove", "purge", "prune", "reset", "rollback", "migrate", "apply":
		return true
	default:
		return false
	}
}

func appendUniqueRiskFlag(flags []model.RiskFlag, flag model.RiskFlag) []model.RiskFlag {
	for _, existing := range flags {
		if existing == flag {
			return flags
		}
	}
	return append(flags, flag)
}
