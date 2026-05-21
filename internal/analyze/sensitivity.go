package analyze

import (
	"regexp"
	"strings"

	"cmd-mint/internal/model"
)

type SensitivityClassification struct {
	Flags   []model.SensitivityFlag
	Reasons []model.ExclusionReason
}

func (classification SensitivityClassification) Sensitive() bool {
	return len(classification.Flags) > 0 || len(classification.Reasons) > 0
}

var (
	secretAssignmentRE = regexp.MustCompile(`(?i)(^|[^a-z0-9_])(?:password|passwd|token|api[_-]?key|apikey|secret|client[_-]?secret|access[_-]?key|refresh[_-]?token|aws_secret_access_key|npm_token|node_auth_token|_authtoken)\s*[:=]`)
	secretNameRE       = regexp.MustCompile(`(?i)\b(?:(?:[a-z0-9]+[_-])+(?:password|passwd|token|api[_-]?key|apikey|secret|secret[_-]?key|client[_-]?secret|access[_-]?key|refresh[_-]?token)(?:[_-][a-z0-9]+)*|password|passwd|token|api[_-]?key|apikey|secret|secret[_-]?key|client[_-]?secret|access[_-]?key|refresh[_-]?token|AWS_SECRET_ACCESS_KEY|NPM_TOKEN|NODE_AUTH_TOKEN|_authToken)\b`)
	authHeaderRE       = regexp.MustCompile(`(?i)\bauthorization\s*:\s*(?:bearer|basic|token)\b`)
	cookieHeaderRE     = regexp.MustCompile(`(?i)(\bcookie\s*:|--cookie(?:=|\s+))`)
	privateKeyRE       = regexp.MustCompile(`(?i)(-----BEGIN [A-Z ]*PRIVATE KEY-----|\bPRIVATE KEY\b|\bid_(?:rsa|dsa|ecdsa|ed25519)\b|\bssh_host_[a-z0-9_]*_key\b|\.(?:pem|p12|pfx)\b)`)
	credentialFileRE   = regexp.MustCompile(`(?i)(^|[/\s'"=])(?:\.env(?:[.\w-]*)?|kubeconfig|\.kube/config|credentials(?:\.(?:json|ya?ml|ini|txt))?|secrets\.json|service[-_]account|\.aws/credentials|\.npmrc|\.netrc|docker/config\.json)\b`)
	databaseURLRE      = regexp.MustCompile(`(?i)\b(?:postgres(?:ql)?|mysql|mariadb|redis|mongodb(?:\+srv)?|sqlserver)://[^\s'"<>/@:]+:[^\s'"<>/@]+@`)
	privateURLRE       = regexp.MustCompile(`(?i)\b[a-z][a-z0-9+.-]*://[^\s'"<>/@:]+:[^\s'"<>/@]+@`)
	clipboardRE        = regexp.MustCompile(`(?i)(\b(?:pbpaste|xclip|xsel|wl-paste)\b|\bsecurity\s+find-(?:generic|internet)-password\b|\bsecret-tool\s+lookup\b|\bpass\s+(?:show|grep)\b|\bgpg\s+--decrypt\b)`)
	cloudSecretRE      = regexp.MustCompile(`(?i)\b(?:aws\s+secretsmanager\s+get-secret-value|gcloud\s+secrets\s+versions\s+access|az\s+keyvault\s+secret\s+show|vault\s+(?:kv\s+get|read)|op\s+(?:read|item\s+get))\b`)
	packageSecretRE    = regexp.MustCompile(`(?i)(\bnpm\s+token\b|\bnpm\s+publish\b.*--//registry|_authToken|NODE_AUTH_TOKEN|NPM_TOKEN|\btwine\s+upload\b.*(?:-p|--password))`)
)

func ClassifyRawSensitivity(command string) SensitivityClassification {
	return classifySensitivity(command, nil)
}

func ClassifyNormalizedSensitivity(command string, tokens []string) SensitivityClassification {
	return classifySensitivity(command, tokens)
}

func IsSensitiveRecord(record model.CommandRecord) bool {
	if len(record.SensitivityFlags) > 0 {
		return true
	}
	for _, reason := range record.ExclusionReasons {
		if isSensitiveExclusion(reason) {
			return true
		}
	}
	return false
}

func classifySensitivity(command string, tokens []string) SensitivityClassification {
	classification := SensitivityClassification{}
	check := func(flag model.SensitivityFlag, reason model.ExclusionReason, matched bool) {
		if !matched {
			return
		}
		classification.Flags = appendUniqueFlag(classification.Flags, flag)
		classification.Reasons = appendUniqueReason(classification.Reasons, reason)
	}

	check(model.SensitivitySecret, model.ExclusionSensitiveSecret, secretAssignmentRE.MatchString(command))
	check(model.SensitivitySecret, model.ExclusionSensitiveSecret, secretNameRE.MatchString(command))
	check(model.SensitivitySecret, model.ExclusionSensitiveSecret, privateURLRE.MatchString(command) && !databaseURLRE.MatchString(command))
	check(model.SensitivitySecret, model.ExclusionSensitiveSecret, cloudSecretRE.MatchString(command))
	check(model.SensitivitySecret, model.ExclusionSensitiveSecret, packageSecretRE.MatchString(command))
	check(model.SensitivityAuthHeader, model.ExclusionSensitiveAuthHeader, authHeaderRE.MatchString(command))
	check(model.SensitivityAuthHeader, model.ExclusionSensitiveAuthHeader, cookieHeaderRE.MatchString(command))
	check(model.SensitivityCredentialFile, model.ExclusionSensitiveCredentialFile, credentialFileRE.MatchString(command))
	check(model.SensitivityDatabaseURL, model.ExclusionSensitiveDatabaseURL, databaseURLRE.MatchString(command))
	check(model.SensitivityPrivateKey, model.ExclusionSensitivePrivateKey, privateKeyRE.MatchString(command))
	check(model.SensitivityClipboardOrKeychain, model.ExclusionSensitiveClipboardOrKeychain, clipboardRE.MatchString(command))

	for i, token := range tokens {
		lower := strings.ToLower(token)
		check(model.SensitivitySecret, model.ExclusionSensitiveSecret, tokenLooksSecret(lower))
		check(model.SensitivitySecret, model.ExclusionSensitiveSecret, tokenLooksSecretName(lower))
		check(model.SensitivityAuthHeader, model.ExclusionSensitiveAuthHeader, tokenLooksAuthHeader(tokens, i))
		check(model.SensitivityCredentialFile, model.ExclusionSensitiveCredentialFile, tokenLooksCredentialFile(lower))
		check(model.SensitivityPrivateKey, model.ExclusionSensitivePrivateKey, tokenLooksPrivateKey(lower))
	}

	return classification
}

func tokenLooksSecret(token string) bool {
	if strings.Contains(token, "aws_secret_access_key") ||
		strings.Contains(token, "node_auth_token") ||
		strings.Contains(token, "npm_token") ||
		strings.Contains(token, "_authtoken") {
		return true
	}

	for _, key := range []string{
		"password",
		"passwd",
		"token",
		"api_key",
		"apikey",
		"secret",
		"client_secret",
		"access_key",
		"refresh_token",
	} {
		if strings.HasPrefix(token, key+"=") || strings.HasPrefix(token, key+":") ||
			strings.Contains(token, "_"+key+"=") || strings.Contains(token, "-"+key+"=") {
			return true
		}
	}
	for _, flag := range []string{"--password", "--token", "--secret", "--api-key", "--apikey"} {
		if strings.HasPrefix(token, flag) {
			return true
		}
	}
	return false
}

func tokenLooksSecretName(token string) bool {
	token = strings.Trim(token, `'"`)
	if token == "" || strings.ContainsAny(token, `/\`) || strings.HasPrefix(token, "-") {
		return false
	}
	if isAssignment(token) {
		return false
	}

	normalized := strings.NewReplacer("-", "_", ".", "_").Replace(token)
	words := strings.FieldsFunc(normalized, func(r rune) bool {
		return r == '_'
	})
	for i, word := range words {
		switch word {
		case "password", "passwd", "token", "apikey", "secret":
			return true
		case "key":
			if i > 0 && words[i-1] == "api" {
				return true
			}
		}
	}

	for _, marker := range []string{
		"apikey",
		"api_key",
		"secretkey",
		"secret_key",
		"clientsecret",
		"client_secret",
		"accesskey",
		"access_key",
		"refreshtoken",
		"refresh_token",
		"authtoken",
		"auth_token",
	} {
		if strings.Contains(normalized, marker) {
			return true
		}
	}
	return false
}

func tokenLooksAuthHeader(tokens []string, index int) bool {
	token := strings.ToLower(tokens[index])
	if strings.Contains(token, "authorization: bearer") ||
		strings.Contains(token, "authorization: basic") ||
		strings.Contains(token, "authorization: token") {
		return true
	}
	if strings.TrimSuffix(token, ":") != "authorization" {
		return false
	}
	if index+1 >= len(tokens) {
		return false
	}
	next := strings.ToLower(tokens[index+1])
	return next == "bearer" || next == "basic" || next == "token"
}

func tokenLooksCredentialFile(token string) bool {
	base := token
	if slash := strings.LastIndex(base, "/"); slash >= 0 {
		base = base[slash+1:]
	}
	return base == ".env" ||
		strings.HasPrefix(base, ".env.") ||
		base == "kubeconfig" ||
		base == "credentials" ||
		base == "secrets.json" ||
		base == "service-account" ||
		base == "service_account" ||
		base == ".npmrc" ||
		base == ".netrc" ||
		strings.HasSuffix(token, "/.kube/config") ||
		strings.HasSuffix(token, "/.aws/credentials") ||
		strings.HasSuffix(token, "/docker/config.json")
}

func tokenLooksPrivateKey(token string) bool {
	base := token
	if slash := strings.LastIndex(base, "/"); slash >= 0 {
		base = base[slash+1:]
	}
	return base == "id_rsa" ||
		base == "id_dsa" ||
		base == "id_ecdsa" ||
		base == "id_ed25519" ||
		strings.HasPrefix(base, "ssh_host_") ||
		strings.HasSuffix(base, ".pem") ||
		strings.HasSuffix(base, ".p12") ||
		strings.HasSuffix(base, ".pfx")
}

func appendUniqueFlag(flags []model.SensitivityFlag, flag model.SensitivityFlag) []model.SensitivityFlag {
	for _, existing := range flags {
		if existing == flag {
			return flags
		}
	}
	return append(flags, flag)
}

func appendUniqueReason(reasons []model.ExclusionReason, reason model.ExclusionReason) []model.ExclusionReason {
	for _, existing := range reasons {
		if existing == reason {
			return reasons
		}
	}
	return append(reasons, reason)
}

func isSensitiveExclusion(reason model.ExclusionReason) bool {
	switch reason {
	case model.ExclusionSensitiveSecret,
		model.ExclusionSensitiveCredentialFile,
		model.ExclusionSensitiveDatabaseURL,
		model.ExclusionSensitiveAuthHeader,
		model.ExclusionSensitivePrivateKey,
		model.ExclusionSensitiveClipboardOrKeychain:
		return true
	default:
		return false
	}
}
