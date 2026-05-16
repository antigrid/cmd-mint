package analyze

import (
	"testing"
	"time"

	"cmd-mint/internal/model"
)

// Run with:
// go test -run '^$' -bench BenchmarkBuildReportSynthetic100k -benchmem ./internal/analyze
func BenchmarkBuildReportSynthetic100k(b *testing.B) {
	records := syntheticBenchmarkRecords(100_000)
	sources := []model.SourceSummary{
		{
			SourceShell:   model.ShellBash,
			SourceFile:    "synthetic",
			EntriesRead:   len(records),
			EntriesParsed: len(records),
		},
	}
	options := ReportOptions{
		GeneratedAt:  time.Unix(0, 0).UTC(),
		MinFrequency: 3,
		MaxAliases:   25,
	}

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = BuildReport(records, sources, options)
	}
}

func syntheticBenchmarkRecords(entries int) []model.CommandRecord {
	commands := []string{
		"git status",
		"git checkout feature-x",
		"npm run build",
		"docker ps",
		"kubectl get pods -n default",
		"terraform destroy",
		"echo token=fake-token",
	}

	records := make([]model.CommandRecord, entries)
	for i := range records {
		records[i] = model.CommandRecord{
			SourceShell: model.ShellBash,
			SourceFile:  "synthetic",
			EntryIndex:  i + 1,
			RawCommand:  commands[i%len(commands)],
			ParseStatus: model.ParseStatusParsed,
		}
	}
	return records
}
