package analyze

import (
	"encoding/json"
	"strings"
	"testing"

	"cmd-mint/internal/model"
)

func TestAnalyzeRecordExcludesRiskyCommandsFromAliasEligibility(t *testing.T) {
	tests := []struct {
		name   string
		raw    string
		flag   model.RiskFlag
		reason model.ExclusionReason
	}{
		{
			name:   "rm recursive force",
			raw:    "rm -rf build",
			flag:   model.RiskDestructive,
			reason: model.ExclusionRiskyDestructive,
		},
		{
			name:   "sudo rm",
			raw:    "sudo rm /tmp/app.sock",
			flag:   model.RiskDestructive,
			reason: model.ExclusionRiskyDestructive,
		},
		{
			name:   "chmod recursive",
			raw:    "chmod -R 777 var/cache",
			flag:   model.RiskDestructive,
			reason: model.ExclusionRiskyDestructive,
		},
		{
			name:   "chown recursive",
			raw:    "chown -R app:app var/cache",
			flag:   model.RiskDestructive,
			reason: model.ExclusionRiskyDestructive,
		},
		{
			name:   "kill sigkill",
			raw:    "kill -9 12345",
			flag:   model.RiskDestructive,
			reason: model.ExclusionRiskyDestructive,
		},
		{
			name:   "docker system prune",
			raw:    "docker system prune -af",
			flag:   model.RiskDestructive,
			reason: model.ExclusionRiskyDestructive,
		},
		{
			name:   "kubectl delete",
			raw:    "kubectl --context live delete deployment api",
			flag:   model.RiskDestructive,
			reason: model.ExclusionRiskyDestructive,
		},
		{
			name:   "terraform destroy",
			raw:    "terraform destroy -auto-approve",
			flag:   model.RiskDestructive,
			reason: model.ExclusionRiskyDestructive,
		},
		{
			name:   "drop database",
			raw:    `psql -c "drop database scratch"`,
			flag:   model.RiskDestructive,
			reason: model.ExclusionRiskyDestructive,
		},
		{
			name:   "truncate table",
			raw:    `mysql -e "truncate table sessions"`,
			flag:   model.RiskDestructive,
			reason: model.ExclusionRiskyDestructive,
		},
		{
			name:   "production deploy",
			raw:    "deploy production",
			flag:   model.RiskProductionAction,
			reason: model.ExclusionRiskyProductionAction,
		},
		{
			name:   "production delete combination",
			raw:    "kubectl delete pods -n prod",
			flag:   model.RiskProductionAction,
			reason: model.ExclusionRiskyProductionAction,
		},
		{
			name:   "production apply combination",
			raw:    "terraform apply -var env=mainnet",
			flag:   model.RiskProductionAction,
			reason: model.ExclusionRiskyProductionAction,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := AnalyzeRecord(model.CommandRecord{
				SourceShell: model.ShellBash,
				RawCommand:  tt.raw,
				ParseStatus: model.ParseStatusParsed,
			})

			if !IsRiskyRecord(got) {
				t.Fatalf("AnalyzeRecord(%q) did not mark record risky: %#v", tt.raw, got)
			}
			if !containsRiskFlag(got.RiskFlags, tt.flag) {
				t.Fatalf("RiskFlags = %#v, want %q", got.RiskFlags, tt.flag)
			}
			if !containsReason(got.ExclusionReasons, tt.reason) {
				t.Fatalf("ExclusionReasons = %#v, want %q", got.ExclusionReasons, tt.reason)
			}
			if got.DisplayCommand == "" || got.NormalizedCommand == "" {
				t.Fatalf("risky non-sensitive record lost safe aggregate fields: %#v", got)
			}
		})
	}
}

func TestRiskyNonSensitiveRecordRemainsAvailableToSafeAggregates(t *testing.T) {
	record := AnalyzeRecord(model.CommandRecord{
		SourceShell: model.ShellZsh,
		RawCommand:  "kubectl delete pod old-worker -n staging",
		ParseStatus: model.ParseStatusParsed,
	})

	if !IsRiskyRecord(record) {
		t.Fatalf("record is not risky: %#v", record)
	}

	summary, ok := model.SafeCommandFromRecord(record, 3, []model.Shell{model.ShellZsh})
	if !ok {
		t.Fatal("SafeCommandFromRecord rejected risky non-sensitive record; want aggregate summary eligibility")
	}
	if summary.Command != "kubectl delete pod old-worker -n staging" {
		t.Fatalf("summary.Command = %q", summary.Command)
	}
}

func TestAnalyzeRecordGivesSensitivityPrecedenceOverRisk(t *testing.T) {
	const raw = "rm -rf .env"

	got := AnalyzeRecord(model.CommandRecord{
		SourceShell: model.ShellBash,
		RawCommand:  raw,
		ParseStatus: model.ParseStatusParsed,
	})

	if !IsSensitiveRecord(got) {
		t.Fatalf("record is not sensitive: %#v", got)
	}
	if IsRiskyRecord(got) {
		t.Fatalf("sensitive record should not receive risk alias-exclusion reasons: %#v", got)
	}
	if got.DisplayCommand != "" || got.NormalizedCommand != "" || len(got.Tokens) != 0 {
		t.Fatalf("sensitive record retained safe output fields: %#v", got)
	}

	data, err := json.Marshal(got)
	if err != nil {
		t.Fatalf("Marshal(CommandRecord) error = %v", err)
	}
	if strings.Contains(string(data), ".env") || strings.Contains(string(data), raw) {
		t.Fatalf("sensitive command JSON contains raw sensitive content: %s", data)
	}
}

func containsRiskFlag(flags []model.RiskFlag, want model.RiskFlag) bool {
	for _, flag := range flags {
		if flag == want {
			return true
		}
	}
	return false
}
