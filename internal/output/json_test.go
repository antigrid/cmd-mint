package output

import (
	"encoding/json"
	"strings"
	"testing"

	"cmd-mint/internal/model"
)

func TestRenderReportJSONIncludesSafeTopLevelShape(t *testing.T) {
	report := sampleReport()
	report.AliasSuggestions[0].AliasFileEligible = true

	data, err := RenderReportJSON(report)
	if err != nil {
		t.Fatalf("RenderReportJSON() error = %v", err)
	}

	var decoded map[string]any
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("report JSON is invalid: %v\n%s", err, data)
	}

	for _, key := range []string{
		"schema_version",
		"generated_at",
		"summary",
		"sources",
		"top_tools",
		"alias_suggestions",
		"tool_sections",
		"patterns",
		"exclusions",
	} {
		if _, ok := decoded[key]; !ok {
			t.Fatalf("report JSON missing top-level key %q:\n%s", key, data)
		}
	}
	if decoded["schema_version"] != model.ReportSchemaVersion {
		t.Fatalf("schema_version = %#v, want %q", decoded["schema_version"], model.ReportSchemaVersion)
	}
}

func TestRenderReportJSONOmitsFakeSensitiveCommandsAndTokens(t *testing.T) {
	report := sampleReport()
	report.AliasSuggestions = append(report.AliasSuggestions, model.AliasSuggestion{
		Name:              "leak",
		Command:           fakeSensitiveCommand,
		Frequency:         99,
		Confidence:        model.ConfidenceHigh,
		AliasFileEligible: true,
	}, model.AliasSuggestion{
		Name:              "splitsecret",
		Command:           "heroku config:set STRIPE_SECRET sk_live_123456789",
		Frequency:         99,
		Confidence:        model.ConfidenceHigh,
		AliasFileEligible: true,
	})
	report.ToolSections = append(report.ToolSections, model.ToolSection{
		Tool:  "curl",
		Count: 1,
		Commands: []model.SafeCommandSummary{
			{
				Command:           fakeSensitiveCommand,
				NormalizedCommand: fakeSensitiveCommand,
				Count:             1,
				Tool:              "curl",
			},
			{
				Command:           "heroku config:set STRIPE_SECRET sk_live_123456789",
				NormalizedCommand: "heroku config:set STRIPE_SECRET sk_live_123456789",
				Count:             1,
				Tool:              "heroku",
			},
		},
	})
	report.Patterns = append(report.Patterns, model.PatternSummary{
		Pattern:  "curl <url>",
		Count:    1,
		Examples: []string{fakeSensitiveCommand},
	})
	report.Warnings = append(report.Warnings, model.Warning{Message: "skipped " + fakeSensitiveCommand})
	report.Sources[0].Warnings = append(report.Sources[0].Warnings, "skipped "+fakeSensitiveCommand)

	data, err := RenderReportJSON(report)
	if err != nil {
		t.Fatalf("RenderReportJSON() error = %v", err)
	}
	output := string(data)

	for _, forbidden := range []string{
		fakeSensitiveCommand,
		"raw-secret-token",
		"Authorization: Bearer",
		"alias leak",
		"STRIPE_SECRET",
		"sk_live_123456789",
	} {
		if strings.Contains(output, forbidden) {
			t.Fatalf("report JSON contains forbidden %q:\n%s", forbidden, output)
		}
	}
	for _, want := range []string{
		`"command": "git status"`,
		`"sensitive_auth_header": 2`,
	} {
		if !strings.Contains(output, want) {
			t.Fatalf("report JSON missing safe aggregate %q:\n%s", want, output)
		}
	}
}

func TestRenderAliasAndJSONArtifactsDoesNotRenderJSONUnlessRequested(t *testing.T) {
	report := sampleReport()

	artifacts, err := RenderAliasAndJSONArtifacts(report, ArtifactOptions{
		NoAliasFile: true,
		MaxAliases:  25,
	})
	if err != nil {
		t.Fatalf("RenderAliasAndJSONArtifacts() error = %v", err)
	}
	if len(artifacts) != 0 {
		t.Fatalf("artifacts = %#v, want none without --json or alias files", artifactNames(artifacts))
	}
}
