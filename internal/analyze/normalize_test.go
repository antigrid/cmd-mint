package analyze

import (
	"reflect"
	"testing"

	"cmd-mint/internal/model"
)

func TestNormalizeCommandTrimsAndCollapsesUnquotedWhitespace(t *testing.T) {
	got := NormalizeCommand(" \t git   status \t --short  ")

	if got.Command != "git status --short" {
		t.Fatalf("NormalizeCommand().Command = %q, want %q", got.Command, "git status --short")
	}
	if got.IsMultiline {
		t.Fatal("NormalizeCommand().IsMultiline = true, want false")
	}
}

func TestNormalizeCommandPreservesQuotedWhitespace(t *testing.T) {
	input := `  git   commit   -m "keep   this"  --author='Ada   Lovelace'  `
	want := `git commit -m "keep   this" --author='Ada   Lovelace'`

	got := NormalizeCommand(input)

	if got.Command != want {
		t.Fatalf("NormalizeCommand().Command = %q, want %q", got.Command, want)
	}
}

func TestNormalizeCommandPreservesSudo(t *testing.T) {
	got := NormalizeCommand("  sudo   docker   ps  ")

	if got.Command != "sudo docker ps" {
		t.Fatalf("NormalizeCommand().Command = %q, want %q", got.Command, "sudo docker ps")
	}
}

func TestAnalyzeCommandDetectsMultilineCommands(t *testing.T) {
	tests := []struct {
		name string
		raw  string
	}{
		{name: "embedded newline", raw: "printf 'one'\nprintf 'two'"},
		{name: "heredoc operator", raw: "cat <<EOF"},
		{name: "heredoc dashed operator", raw: "cat <<-EOF"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := AnalyzeCommand(tt.raw)
			if !got.IsMultiline {
				t.Fatalf("AnalyzeCommand(%q).IsMultiline = false, want true", tt.raw)
			}
		})
	}
}

func TestAnalyzeRecordPopulatesModelFields(t *testing.T) {
	record := model.CommandRecord{
		SourceShell: model.ShellZsh,
		RawCommand:  "  sudo   docker   compose   up  ",
		ParseStatus: model.ParseStatusParsed,
	}

	got := AnalyzeRecord(record)

	if got.NormalizedCommand != "sudo docker compose up" {
		t.Fatalf("NormalizedCommand = %q, want %q", got.NormalizedCommand, "sudo docker compose up")
	}
	if !reflect.DeepEqual(got.Tokens, []string{"sudo", "docker", "compose", "up"}) {
		t.Fatalf("Tokens = %#v", got.Tokens)
	}
	if got.Tool != "docker compose" {
		t.Fatalf("Tool = %q, want %q", got.Tool, "docker compose")
	}
	if got.Subcommand != "up" {
		t.Fatalf("Subcommand = %q, want %q", got.Subcommand, "up")
	}
	if got.RawCommand != record.RawCommand {
		t.Fatalf("RawCommand changed to %q", got.RawCommand)
	}
}
