package analyze

import (
	"encoding/json"
	"strings"
	"testing"

	"cmd-mint/internal/model"
)

func TestClassifyRawSensitivityDetectsRequiredPatterns(t *testing.T) {
	tests := []struct {
		name   string
		raw    string
		flag   model.SensitivityFlag
		reason model.ExclusionReason
	}{
		{
			name:   "password assignment",
			raw:    `curl https://example.invalid/login -d password=fake-password`,
			flag:   model.SensitivitySecret,
			reason: model.ExclusionSensitiveSecret,
		},
		{
			name:   "token assignment",
			raw:    `export token=fake-token-value`,
			flag:   model.SensitivitySecret,
			reason: model.ExclusionSensitiveSecret,
		},
		{
			name:   "aws secret access key",
			raw:    `AWS_SECRET_ACCESS_KEY=fake-aws-secret aws sts get-caller-identity`,
			flag:   model.SensitivitySecret,
			reason: model.ExclusionSensitiveSecret,
		},
		{
			name:   "separated secret name and value",
			raw:    `heroku config:set STRIPE_SECRET sk_live_123456789`,
			flag:   model.SensitivitySecret,
			reason: model.ExclusionSensitiveSecret,
		},
		{
			name:   "separated api key name and value",
			raw:    `fly secrets set OPENAI_API_KEY sk-project-123456789`,
			flag:   model.SensitivitySecret,
			reason: model.ExclusionSensitiveSecret,
		},
		{
			name:   "authorization bearer header",
			raw:    `curl -H "Authorization: Bearer fake-bearer-token" https://example.invalid`,
			flag:   model.SensitivityAuthHeader,
			reason: model.ExclusionSensitiveAuthHeader,
		},
		{
			name:   "env file",
			raw:    `cat .env`,
			flag:   model.SensitivityCredentialFile,
			reason: model.ExclusionSensitiveCredentialFile,
		},
		{
			name:   "ssh private key",
			raw:    `ssh -i ~/.ssh/id_rsa host.example.invalid`,
			flag:   model.SensitivityPrivateKey,
			reason: model.ExclusionSensitivePrivateKey,
		},
		{
			name:   "credentialed database url",
			raw:    `psql postgres://fake_user:fake_password@localhost:5432/app`,
			flag:   model.SensitivityDatabaseURL,
			reason: model.ExclusionSensitiveDatabaseURL,
		},
		{
			name:   "clipboard extraction",
			raw:    `pbpaste | base64`,
			flag:   model.SensitivityClipboardOrKeychain,
			reason: model.ExclusionSensitiveClipboardOrKeychain,
		},
		{
			name:   "package publishing credential",
			raw:    `npm token create`,
			flag:   model.SensitivitySecret,
			reason: model.ExclusionSensitiveSecret,
		},
		{
			name:   "refresh token assignment",
			raw:    `curl -d refresh_token=fake-refresh-token https://example.invalid`,
			flag:   model.SensitivitySecret,
			reason: model.ExclusionSensitiveSecret,
		},
		{
			name:   "vault secret read",
			raw:    `vault kv get secret/app`,
			flag:   model.SensitivitySecret,
			reason: model.ExclusionSensitiveSecret,
		},
		{
			name:   "service account credential file",
			raw:    `cat service-account.json`,
			flag:   model.SensitivityCredentialFile,
			reason: model.ExclusionSensitiveCredentialFile,
		},
		{
			name:   "kubeconfig flag with separate value",
			raw:    `kubectl --kubeconfig /tmp/prod.yaml get pods`,
			flag:   model.SensitivityCredentialFile,
			reason: model.ExclusionSensitiveCredentialFile,
		},
		{
			name:   "kubeconfig flag with equals value",
			raw:    `kubectl --kubeconfig=/tmp/prod.yaml get pods`,
			flag:   model.SensitivityCredentialFile,
			reason: model.ExclusionSensitiveCredentialFile,
		},
		{
			name:   "docker auth config",
			raw:    `cat ~/.docker/config.json`,
			flag:   model.SensitivityCredentialFile,
			reason: model.ExclusionSensitiveCredentialFile,
		},
		{
			name:   "private key extension",
			raw:    `openssl rsa -in ./certs/client.key -check`,
			flag:   model.SensitivityPrivateKey,
			reason: model.ExclusionSensitivePrivateKey,
		},
		{
			name:   "certificate file",
			raw:    `curl --cert ./certs/client.crt https://example.invalid`,
			flag:   model.SensitivityCredentialFile,
			reason: model.ExclusionSensitiveCredentialFile,
		},
		{
			name:   "gpg decrypt clipboard-like extraction",
			raw:    `gpg --decrypt secrets.asc`,
			flag:   model.SensitivityClipboardOrKeychain,
			reason: model.ExclusionSensitiveClipboardOrKeychain,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ClassifyRawSensitivity(tt.raw)
			if !got.Sensitive() {
				t.Fatalf("ClassifyRawSensitivity(%q).Sensitive() = false, want true", tt.raw)
			}
			if !containsFlag(got.Flags, tt.flag) {
				t.Fatalf("flags = %#v, want %q", got.Flags, tt.flag)
			}
			if !containsReason(got.Reasons, tt.reason) {
				t.Fatalf("reasons = %#v, want %q", got.Reasons, tt.reason)
			}
		})
	}
}

func TestClassifyNormalizedSensitivityCanTriggerFromTokens(t *testing.T) {
	normalized := `curl -H auth-header fake-token https://example.invalid`
	tokens := []string{"curl", "-H", "Authorization:", "Bearer", "fake-token", "https://example.invalid"}

	got := ClassifyNormalizedSensitivity(normalized, tokens)

	if !containsFlag(got.Flags, model.SensitivityAuthHeader) {
		t.Fatalf("flags = %#v, want auth header flag", got.Flags)
	}
	if !containsReason(got.Reasons, model.ExclusionSensitiveAuthHeader) {
		t.Fatalf("reasons = %#v, want auth header reason", got.Reasons)
	}
}

func TestAnalyzeRecordExcludesSensitiveCommandsFromSafeFields(t *testing.T) {
	const raw = `curl -H "Authorization: Bearer fake-bearer-token" https://example.invalid`

	got := AnalyzeRecord(model.CommandRecord{
		SourceShell: model.ShellBash,
		RawCommand:  raw,
		ParseStatus: model.ParseStatusParsed,
	})

	if !IsSensitiveRecord(got) {
		t.Fatalf("AnalyzeRecord() did not mark record sensitive: %#v", got)
	}
	if got.DisplayCommand != "" || got.NormalizedCommand != "" || len(got.Tokens) != 0 || got.Tool != "" {
		t.Fatalf("sensitive record retained safe output fields: %#v", got)
	}

	data, err := json.Marshal(got)
	if err != nil {
		t.Fatalf("Marshal(CommandRecord) error = %v", err)
	}
	for _, forbidden := range []string{raw, "fake-bearer-token", "Authorization: Bearer"} {
		if strings.Contains(string(data), forbidden) {
			t.Fatalf("sensitive record JSON contains %q: %s", forbidden, data)
		}
	}
}

func TestAnalyzeRecordKeepsSafeDisplayCommand(t *testing.T) {
	got := AnalyzeRecord(model.CommandRecord{
		SourceShell: model.ShellZsh,
		RawCommand:  "  git   status  ",
		ParseStatus: model.ParseStatusParsed,
	})

	if IsSensitiveRecord(got) {
		t.Fatalf("safe record marked sensitive: %#v", got)
	}
	if got.DisplayCommand != "git status" {
		t.Fatalf("DisplayCommand = %q, want git status", got.DisplayCommand)
	}
}

func TestClassifyRawSensitivityDoesNotTreatOrdinaryTokenWordsAsSecrets(t *testing.T) {
	for _, raw := range []string{
		"go test ./internal/analyze/tokenize",
		"vim tokenization-notes.md",
	} {
		if got := ClassifyRawSensitivity(raw); got.Sensitive() {
			t.Fatalf("ClassifyRawSensitivity(%q).Sensitive() = true, want false: %#v", raw, got)
		}
	}
}

func containsFlag(flags []model.SensitivityFlag, want model.SensitivityFlag) bool {
	for _, flag := range flags {
		if flag == want {
			return true
		}
	}
	return false
}

func containsReason(reasons []model.ExclusionReason, want model.ExclusionReason) bool {
	for _, reason := range reasons {
		if reason == want {
			return true
		}
	}
	return false
}
