package model

type ExclusionReason string

const (
	ExclusionSensitiveSecret              ExclusionReason = "sensitive_secret"
	ExclusionSensitiveCredentialFile      ExclusionReason = "sensitive_credential_file"
	ExclusionSensitiveDatabaseURL         ExclusionReason = "sensitive_database_url"
	ExclusionSensitiveAuthHeader          ExclusionReason = "sensitive_auth_header"
	ExclusionSensitivePrivateKey          ExclusionReason = "sensitive_private_key"
	ExclusionSensitiveClipboardOrKeychain ExclusionReason = "sensitive_clipboard_or_keychain"
	ExclusionRiskyDestructive             ExclusionReason = "risky_destructive"
	ExclusionRiskyProductionAction        ExclusionReason = "risky_production_action"
	ExclusionMultiline                    ExclusionReason = "multiline"
	ExclusionParseFailed                  ExclusionReason = "parse_failed"
	ExclusionTooShort                     ExclusionReason = "too_short"
	ExclusionLowFrequency                 ExclusionReason = "low_frequency"
	ExclusionLowSavings                   ExclusionReason = "low_savings"
	ExclusionAliasConflict                ExclusionReason = "alias_conflict"
	ExclusionUnsupportedShell             ExclusionReason = "unsupported_shell"
	ExclusionUnreadableSource             ExclusionReason = "unreadable_source"
	ExclusionMalformedEntry               ExclusionReason = "malformed_entry"
)

func MVPExclusionReasons() []ExclusionReason {
	return []ExclusionReason{
		ExclusionSensitiveSecret,
		ExclusionSensitiveCredentialFile,
		ExclusionSensitiveDatabaseURL,
		ExclusionSensitiveAuthHeader,
		ExclusionSensitivePrivateKey,
		ExclusionSensitiveClipboardOrKeychain,
		ExclusionRiskyDestructive,
		ExclusionRiskyProductionAction,
		ExclusionMultiline,
		ExclusionParseFailed,
		ExclusionTooShort,
		ExclusionLowFrequency,
		ExclusionLowSavings,
		ExclusionAliasConflict,
		ExclusionUnsupportedShell,
		ExclusionUnreadableSource,
		ExclusionMalformedEntry,
	}
}
