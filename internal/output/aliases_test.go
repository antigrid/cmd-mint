package output

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"cmd-mint/internal/model"
)

func TestRenderAliasesSHIncludesRequiredHeaderAndQuotesSingleQuotes(t *testing.T) {
	output := string(RenderAliasesSH([]model.AliasSuggestion{
		{
			Name:                 "gcm",
			Command:              "git commit -m 'fix it'",
			Frequency:            4,
			EstimatedSavedTotal:  80,
			EstimatedSavedPerUse: 20,
			Confidence:           model.ConfidenceHigh,
			AliasFileEligible:    true,
		},
	}, 25))

	if !strings.HasPrefix(output, aliasesSHHeader) {
		t.Fatalf("alias output header = %q, want prefix %q", output, aliasesSHHeader)
	}
	want := "alias gcm='git commit -m '\\''fix it'\\'''\n"
	if !strings.Contains(output, want) {
		t.Fatalf("alias output missing safely quoted line %q:\n%s", want, output)
	}
}

func TestRenderAliasesFishIncludesRequiredHeaderAndFishQuote(t *testing.T) {
	output := string(RenderAliasesFish([]model.AliasSuggestion{
		{
			Name:              "gcm",
			Command:           "git commit -m 'fix it'",
			Frequency:         4,
			Confidence:        model.ConfidenceMedium,
			AliasFileEligible: true,
		},
	}, 25))

	if !strings.HasPrefix(output, aliasesFishHeader) {
		t.Fatalf("fish alias output header = %q, want prefix %q", output, aliasesFishHeader)
	}
	want := "alias gcm='git commit -m \\'fix it\\''\n"
	if !strings.Contains(output, want) {
		t.Fatalf("fish alias output missing safely quoted line %q:\n%s", want, output)
	}
}

func TestRenderAliasesMaxAliasesZeroWritesOnlyHeader(t *testing.T) {
	output := string(RenderAliasesSH([]model.AliasSuggestion{
		{
			Name:              "gs",
			Command:           "git status",
			Frequency:         20,
			Confidence:        model.ConfidenceHigh,
			AliasFileEligible: true,
		},
	}, 0))

	if strings.Contains(output, "alias gs=") {
		t.Fatalf("alias output contains alias when max aliases is zero:\n%s", output)
	}
	if strings.Contains(output, "# No safe alias suggestions were found.") {
		t.Fatalf("alias output should be header-only when max aliases is zero:\n%s", output)
	}
}

func TestRenderAliasesFiltersIneligibleLowConfidenceAndSensitiveSuggestions(t *testing.T) {
	output := string(RenderAliasesSH([]model.AliasSuggestion{
		{
			Name:              "safe",
			Command:           "git status",
			Frequency:         20,
			Confidence:        model.ConfidenceHigh,
			AliasFileEligible: true,
		},
		{
			Name:              "low",
			Command:           "git checkout feature/example",
			Frequency:         20,
			Confidence:        model.ConfidenceLow,
			AliasFileEligible: true,
		},
		{
			Name:              "ineligible",
			Command:           "docker ps",
			Frequency:         20,
			Confidence:        model.ConfidenceHigh,
			AliasFileEligible: false,
		},
		{
			Name:              "secret",
			Command:           fakeSensitiveCommand,
			Frequency:         20,
			Confidence:        model.ConfidenceHigh,
			AliasFileEligible: true,
		},
		{
			Name:              "splitsecret",
			Command:           "heroku config:set STRIPE_SECRET sk_live_123456789",
			Frequency:         20,
			Confidence:        model.ConfidenceHigh,
			AliasFileEligible: true,
		},
	}, 25))

	for _, want := range []string{"alias safe='git status'\n"} {
		if !strings.Contains(output, want) {
			t.Fatalf("alias output missing %q:\n%s", want, output)
		}
	}
	for _, forbidden := range []string{"alias low=", "alias ineligible=", fakeSensitiveCommand, "raw-secret-token", "STRIPE_SECRET", "sk_live_123456789"} {
		if strings.Contains(output, forbidden) {
			t.Fatalf("alias output contains forbidden %q:\n%s", forbidden, output)
		}
	}
}

func TestRenderAliasAndJSONArtifactsRespectsFlagsAndFishConditions(t *testing.T) {
	report := sampleReport()
	report.AliasSuggestions[0].AliasFileEligible = true

	tests := []struct {
		name    string
		report  model.Report
		options ArtifactOptions
		want    []string
	}{
		{
			name:    "default shell alias only",
			report:  report,
			options: ArtifactOptions{MaxAliases: 25},
			want:    []string{ArtifactAliasesSH},
		},
		{
			name:   "fish source also writes fish alias file",
			report: reportWithFishSource(report),
			options: ArtifactOptions{
				MaxAliases: 25,
			},
			want: []string{ArtifactAliasesSH, ArtifactAliasesFish},
		},
		{
			name:    "requested fish writes fish alias file",
			report:  report,
			options: ArtifactOptions{MaxAliases: 25, Shell: model.ShellFish},
			want:    []string{ArtifactAliasesSH, ArtifactAliasesFish},
		},
		{
			name:    "no alias file suppresses aliases",
			report:  reportWithFishSource(report),
			options: ArtifactOptions{NoAliasFile: true, MaxAliases: 25},
			want:    []string{},
		},
		{
			name:    "json requested writes json",
			report:  report,
			options: ArtifactOptions{NoAliasFile: true, JSON: true, MaxAliases: 25},
			want:    []string{ArtifactReportJSON},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			artifacts, err := RenderAliasAndJSONArtifacts(tt.report, tt.options)
			if err != nil {
				t.Fatalf("RenderAliasAndJSONArtifacts() error = %v", err)
			}
			got := artifactNames(artifacts)
			if !reflect.DeepEqual(got, tt.want) {
				t.Fatalf("artifact names = %#v, want %#v", got, tt.want)
			}
		})
	}
}

func TestWriteAliasAndJSONArtifactsUsesSafeWriterAndSuppressesUnrequestedFiles(t *testing.T) {
	dir := t.TempDir()
	report := reportWithFishSource(sampleReport())
	report.AliasSuggestions[0].AliasFileEligible = true

	paths, err := WriteAliasAndJSONArtifacts(dir, report, ArtifactOptions{
		NoAliasFile: true,
		JSON:        true,
		MaxAliases:  25,
	})
	if err != nil {
		t.Fatalf("WriteAliasAndJSONArtifacts() error = %v", err)
	}
	if want := []string{filepath.Join(dir, ArtifactReportJSON)}; !reflect.DeepEqual(paths, want) {
		t.Fatalf("paths = %#v, want %#v", paths, want)
	}

	if _, err := os.Stat(filepath.Join(dir, ArtifactAliasesSH)); !os.IsNotExist(err) {
		t.Fatalf("aliases.suggested.sh exists or stat error is not IsNotExist: %v", err)
	}
	if _, err := os.Stat(filepath.Join(dir, ArtifactAliasesFish)); !os.IsNotExist(err) {
		t.Fatalf("aliases.suggested.fish exists or stat error is not IsNotExist: %v", err)
	}
	assertFileMode(t, filepath.Join(dir, ArtifactReportJSON), 0o600)
}

func artifactNames(artifacts []Artifact) []string {
	names := make([]string, 0, len(artifacts))
	for _, artifact := range artifacts {
		names = append(names, artifact.Name)
	}
	return names
}

func reportWithFishSource(report model.Report) model.Report {
	report.Sources = append(append([]model.SourceSummary(nil), report.Sources...), model.SourceSummary{
		SourceShell:   model.ShellFish,
		SourceFile:    "/home/alex/.local/share/fish/fish_history",
		EntriesRead:   2,
		EntriesParsed: 2,
	})
	return report
}
