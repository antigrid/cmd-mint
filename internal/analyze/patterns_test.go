package analyze

import (
	"reflect"
	"testing"

	"cmd-mint/internal/model"
)

func TestPatternForCommandUsesRequiredPlaceholders(t *testing.T) {
	tests := []struct {
		name    string
		command string
		want    string
	}{
		{
			name:    "git branch",
			command: "git checkout feature/search",
			want:    "git checkout <branch>",
		},
		{
			name:    "kubectl namespace short flag",
			command: "kubectl get pods -n staging",
			want:    "kubectl get pods -n <namespace>",
		},
		{
			name:    "kubectl namespace long flag",
			command: "kubectl get pods --namespace=production",
			want:    "kubectl get pods --namespace=<namespace>",
		},
		{
			name:    "npm script",
			command: "npm run build",
			want:    "npm run <script>",
		},
		{
			name:    "url without credentials",
			command: "curl https://example.com/api/v1",
			want:    "curl <url>",
		},
		{
			name:    "file path",
			command: "cat ./internal/analyze/patterns.go",
			want:    "cat <path>",
		},
		{
			name:    "numeric port",
			command: "python -m http.server 8080",
			want:    "python -m http.server <port>",
		},
		{
			name:    "docker image",
			command: "docker run --rm redis:7",
			want:    "docker run --rm <image>",
		},
		{
			name:    "docker compose service",
			command: "docker compose logs api",
			want:    "docker compose logs <service>",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, ok := PatternForCommand(tt.command)
			if !ok {
				t.Fatalf("PatternForCommand(%q) returned false", tt.command)
			}
			if got != tt.want {
				t.Fatalf("PatternForCommand(%q) = %q, want %q", tt.command, got, tt.want)
			}
		})
	}
}

func TestGeneratePatternsAggregatesAndSortsDeterministically(t *testing.T) {
	commands := []CommandAggregate{
		patternCommand("npm run test", 2),
		patternCommand("git checkout main", 1),
		patternCommand("npm run build", 4),
		patternCommand("git checkout feature/search", 3),
	}

	var want []model.PatternSummary
	for i := 0; i < 20; i++ {
		got := GeneratePatternsFromCommands(commands)
		if i == 0 {
			want = got
			continue
		}
		if !reflect.DeepEqual(got, want) {
			t.Fatalf("pattern order run %d = %#v, want %#v", i, got, want)
		}
	}

	expected := []model.PatternSummary{
		{
			Pattern:  "npm run <script>",
			Count:    6,
			Examples: []string{"npm run build", "npm run test"},
		},
		{
			Pattern:  "git checkout <branch>",
			Count:    4,
			Examples: []string{"git checkout feature/search", "git checkout main"},
		},
	}
	if !reflect.DeepEqual(want, expected) {
		t.Fatalf("GeneratePatternsFromCommands() = %#v, want %#v", want, expected)
	}
}

func TestAggregateRecordsBuildsPatternsFromSafeCommands(t *testing.T) {
	result := AggregateRecords([]model.CommandRecord{
		rawRecord(model.ShellZsh, "zsh", "git checkout main"),
		rawRecord(model.ShellZsh, "zsh", "git checkout feature/search"),
		rawRecord(model.ShellZsh, "zsh", "kubectl get pods -n staging"),
	}, nil, AggregateOptions{})

	got := patternNames(result.Patterns)
	want := []string{"git checkout <branch>", "kubectl get pods -n <namespace>"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("patterns = %#v, want %#v", got, want)
	}
}

func TestPatternsStaySeparateFromAliasSuggestions(t *testing.T) {
	commands := []CommandAggregate{
		patternCommand("git checkout feature/search", 5),
		patternCommand("kubectl get pods -n staging", 5),
	}

	patterns := GeneratePatternsFromCommands(commands)
	aliases := GenerateAliasSuggestionsFromCommands(commands, AliasOptions{MinFrequency: 3, MaxAliases: 25})

	if len(patterns) == 0 {
		t.Fatal("GeneratePatternsFromCommands() returned no patterns")
	}
	for _, suggestion := range aliases.Suggestions {
		if suggestion.Command == "git checkout <branch>" ||
			suggestion.Command == "kubectl get pods -n <namespace>" {
			t.Fatalf("alias suggestion used a pattern as command: %#v", suggestion)
		}
		if suggestion.Name == "function" || suggestion.Name == "git checkout <branch>" {
			t.Fatalf("alias suggestion looks function/pattern-derived: %#v", suggestion)
		}
	}
	for _, suggestion := range aliases.AliasFileSuggestions {
		if suggestion.Command == "git checkout <branch>" ||
			suggestion.Command == "kubectl get pods -n <namespace>" {
			t.Fatalf("alias file suggestion used a pattern as command: %#v", suggestion)
		}
	}
}

func TestPatternForCommandKeepsUnsafeOrUncertainURLTokensOriginal(t *testing.T) {
	got, ok := PatternForCommand("curl https://user:pass@example.com/api")
	if ok && got == "curl <url>" {
		t.Fatalf("credential URL was generalized as safe URL: %q", got)
	}
}

func patternCommand(command string, count int) CommandAggregate {
	analysis := AnalyzeCommand(command)
	return CommandAggregate{
		NormalizedCommand: analysis.NormalizedCommand,
		DisplayCommand:    analysis.NormalizedCommand,
		Tool:              analysis.Tool,
		Subcommand:        analysis.Subcommand,
		Count:             count,
	}
}

func patternNames(patterns []model.PatternSummary) []string {
	names := make([]string, len(patterns))
	for i, pattern := range patterns {
		names[i] = pattern.Pattern
	}
	return names
}
