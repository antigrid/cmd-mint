package analyze

import (
	"reflect"
	"testing"
)

func TestTokenizePreservesQuotedStringsAsSingleTokens(t *testing.T) {
	command := `git commit -m "keep   this" --author='Ada Lovelace'`
	want := []string{"git", "commit", "-m", "keep   this", "--author=Ada Lovelace"}

	got := Tokenize(command)

	if !reflect.DeepEqual(got, want) {
		t.Fatalf("Tokenize() = %#v, want %#v", got, want)
	}
}

func TestDetectToolSkipsWrappersForGrouping(t *testing.T) {
	tests := []struct {
		name       string
		command    string
		tool       string
		subcommand string
	}{
		{name: "sudo", command: "sudo docker ps", tool: "docker", subcommand: "ps"},
		{name: "sudo options", command: "sudo -E -u root docker ps", tool: "docker", subcommand: "ps"},
		{name: "time", command: "time git status", tool: "git", subcommand: "status"},
		{name: "time options", command: "time -p git status", tool: "git", subcommand: "status"},
		{name: "env", command: "env FOO=bar npm test", tool: "npm", subcommand: "test"},
		{name: "env options", command: "env -i FOO=bar pnpm run build", tool: "pnpm", subcommand: "run"},
		{name: "leading assignment", command: "FOO=bar yarn test", tool: "yarn", subcommand: "test"},
		{name: "noglob", command: "noglob kubectl get pods", tool: "kubectl", subcommand: "get"},
		{name: "command", command: "command git status", tool: "git", subcommand: "status"},
		{name: "builtin", command: "builtin echo hello", tool: "echo", subcommand: "hello"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := DetectTool(Tokenize(tt.command))
			if got.Tool != tt.tool || got.Subcommand != tt.subcommand {
				t.Fatalf("DetectTool(%q) = %#v, want tool %q subcommand %q", tt.command, got, tt.tool, tt.subcommand)
			}
		})
	}
}

func TestDetectToolKeepsCommandWrapperWhenOptionChangesMeaning(t *testing.T) {
	got := DetectTool(Tokenize("command -v git"))

	if got.Tool != "command" || got.Subcommand != "git" {
		t.Fatalf("DetectTool(command -v git) = %#v, want command/git", got)
	}
}

func TestDetectToolExtractsToolSpecificSubcommands(t *testing.T) {
	tests := []struct {
		name       string
		command    string
		tool       string
		subcommand string
	}{
		{name: "git", command: "git -C repo status --short", tool: "git", subcommand: "status"},
		{name: "docker", command: "docker ps --all", tool: "docker", subcommand: "ps"},
		{name: "docker compose", command: "docker compose up --detach", tool: "docker compose", subcommand: "up"},
		{name: "kubectl", command: "kubectl --namespace dev get pods", tool: "kubectl", subcommand: "get"},
		{name: "npm run", command: "npm run build", tool: "npm", subcommand: "run"},
		{name: "npm verb", command: "npm test", tool: "npm", subcommand: "test"},
		{name: "pnpm run", command: "pnpm run lint", tool: "pnpm", subcommand: "run"},
		{name: "yarn", command: "yarn test", tool: "yarn", subcommand: "test"},
		{name: "fallback", command: "make test", tool: "make", subcommand: "test"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := DetectTool(Tokenize(tt.command))
			if got.Tool != tt.tool || got.Subcommand != tt.subcommand {
				t.Fatalf("DetectTool(%q) = %#v, want tool %q subcommand %q", tt.command, got, tt.tool, tt.subcommand)
			}
		})
	}
}

func TestContainsHeredoc(t *testing.T) {
	tests := []struct {
		tokens []string
		want   bool
	}{
		{tokens: []string{"cat", "<<EOF"}, want: true},
		{tokens: []string{"cat", "<<-", "EOF"}, want: true},
		{tokens: []string{"git", "status"}, want: false},
	}

	for _, tt := range tests {
		got := ContainsHeredoc(tt.tokens)
		if got != tt.want {
			t.Fatalf("ContainsHeredoc(%#v) = %v, want %v", tt.tokens, got, tt.want)
		}
	}
}
