# cmd-mint

`cmd-mint` is a local, privacy-first CLI tool for turning shell history into a practical command reference.

It is designed for terminal-heavy developers who want useful shortcuts without sending shell history anywhere or letting a tool edit their shell setup.

## What cmd-mint Does

`cmd-mint` reads supported local shell history files, identifies repeated safe commands and command patterns, and writes a personalized developer cheat sheet plus safe alias suggestions. It groups commands by tools such as `git`, `docker`, `npm`, and `kubectl`, summarizes sensitive or risky exclusions without exposing raw skipped commands, and leaves all adoption decisions to you.

## Privacy Model

`cmd-mint` keeps the MVP local-only:

- Runs on your machine.
- Does not send shell history anywhere.
- Does not make network calls.
- Does not collect telemetry.
- Does not execute commands from shell history.
- Does not modify shell history files.
- Does not modify shell config files such as `.zshrc`, `.bashrc`, `.bash_aliases`, or `config.fish`.
- Does not install aliases automatically.
- Skips sensitive-looking commands from generated outputs.

Generated reports are still derived from your local shell history. They may reveal private workflows, project names, hosts, branch names, namespaces, or operational habits. Review generated files before sharing, committing, or pasting them anywhere.

## Installation

The MVP is distributed as a standalone binary for macOS and Linux. Download the binary for your platform from a release, then install it manually:

```sh
chmod +x cmd-mint
mv cmd-mint /usr/local/bin/cmd-mint
cmd-mint --version
```

If `/usr/local/bin` is not writable on your system, move the binary to another directory already on your `PATH`, or run it by path:

```sh
./cmd-mint --version
```

Package-manager installation is not required for the MVP.

## Quick Start

Run `cmd-mint` with no flags:

```sh
cmd-mint
```

By default, it:

1. Auto-discovers supported shell history files.
2. Reads shell config files only to detect existing alias-name conflicts.
3. Analyzes safe parsed commands.
4. Creates a timestamped report directory in the current directory.
5. Writes `cheatsheet.md` and alias suggestion files.
6. Prints a concise terminal summary.

Default output directory format:

```text
./shell-history-report-YYYY-MM-DD-HHMMSS/
```

Use an explicit history file and output directory when you want a reproducible test run:

```sh
cmd-mint \
  --history-file ~/.zsh_history \
  --shell zsh \
  --output-dir ./cmd-mint-report \
  --json
```

## CLI Flags

| Flag | Default | Description |
|---|---:|---|
| `--history-file PATH` | none | Adds an explicit history file to scan. Repeat the flag for multiple files. |
| `--shell bash\|zsh\|fish\|auto` | `auto` | Interprets explicit history files. Auto-discovered files use known shell type where possible. |
| `--output-dir PATH` | timestamped directory in current directory | Writes artifacts to the given directory. Creates it when needed. Fails if the path is an existing regular file. |
| `--max-aliases N` | `25` | Maximum alias suggestions written to alias files. Use `0` to write only alias file headers. |
| `--min-frequency N` | `3` | Minimum exact normalized command frequency for alias eligibility. Must be at least `1`. |
| `--no-alias-file` | `false` | Skips alias snippet file generation. The cheat sheet can still include alias suggestions. |
| `--json` | `false` | Also writes safe `report.json`. |
| `--verbose` | `false` | Prints more parsing and exclusion summary information without raw sensitive commands. |
| `--version` | `false` | Prints version, commit, and build date, then exits. |
| `--help` | `false` | Prints help, then exits. |

## Generated Files

`cmd-mint` writes generated files into the report directory with owner-only permissions where the platform supports them.

| File | When generated | Purpose |
|---|---|---|
| `cheatsheet.md` | Always | Human-readable summary grouped by source, tool, aliases, patterns, and exclusions. |
| `aliases.suggested.sh` | By default | Bash/zsh-compatible alias suggestions. Not installed automatically. |
| `aliases.suggested.fish` | Fish input is detected or `--shell fish` is used | Fish-compatible alias suggestions. Not installed automatically. |
| `report.json` | Only with `--json` | Safe machine-readable report for tests, debugging, and future integrations. |

The report summarizes skipped sensitive or risky commands only in aggregate. It should not contain raw sensitive skipped commands.

## Reviewing and Adopting Aliases

Alias files are suggestions, not installation scripts. Review them first and manually copy only the aliases you understand and want to keep.

For bash or zsh:

```sh
# Review first
cat shell-history-report-YYYY-MM-DD-HHMMSS/aliases.suggested.sh

# Manually copy selected aliases into ~/.zshrc, ~/.bashrc, or ~/.bash_aliases
```

For fish:

```fish
cat shell-history-report-YYYY-MM-DD-HHMMSS/aliases.suggested.fish
# Manually copy selected aliases into ~/.config/fish/config.fish
```

Do not blindly source generated alias files. Check that each alias is safe, useful, and not too specific to a temporary project or environment.

## Supported Shells

The MVP supports these history sources:

| Shell | Auto-discovered path | Format support |
|---|---|---|
| zsh | `~/.zsh_history` | Plain and extended zsh history. |
| bash | `~/.bash_history` | Plain history and bash timestamp markers. |
| fish | `~/.local/share/fish/fish_history` | Fish YAML-like history records. |

`HISTFILE` is also inspected when available. Native Windows shell history support is out of scope for the MVP; WSL is best-effort.

## Limitations and Non-Goals

The MVP intentionally does not include:

- Hosted service, accounts, cloud sync, or team sharing.
- Network analysis, LLM/cloud analysis, or telemetry.
- GUI, TUI, or interactive alias review.
- Config files or persistent cache/database.
- Automatic alias installation.
- Shell config modification.
- Shell history modification.
- Command execution or command rewriting.
- Native Windows shell history support.
- Homebrew formula, shell completions, or package-manager installation as MVP features.

Alias suggestions are exact-command aliases only. Frequent patterns may appear in `cheatsheet.md`, but parameterized aliases or shell functions are not generated in the MVP.

## Troubleshooting

`cmd-mint` exits with these codes:

| Code | Meaning |
|---:|---|
| `0` | Completed successfully. At least one history source was parsed or partially parsed. |
| `1` | No readable or parseable history files found. |
| `2` | Invalid CLI arguments. |
| `3` | Output directory could not be created or written. |
| `4` | Unexpected internal error. |

Common fixes:

- No history found: pass one or more files with `--history-file PATH`.
- Wrong parser for a custom file: pass `--shell bash`, `--shell zsh`, or `--shell fish`.
- Output path error: choose a directory path, not an existing regular file.
- Too few aliases: lower `--min-frequency` or check `cheatsheet.md` for exclusion summaries.
- No alias file wanted: use `--no-alias-file`.
- Need machine-readable output: use `--json`.

Warnings do not fail a run unless no usable input remains or output cannot be written.

## Security Notes

`cmd-mint` uses conservative filtering before rendering output. Commands that look like they contain passwords, tokens, API keys, auth headers, private keys, credential files, `.env` files, database URLs with credentials, kubeconfig secrets, clipboard/keychain extraction, or similar sensitive material are excluded from generated command output.

Alias suggestions are stricter than the cheat sheet. Destructive or high-risk commands, production-like deploy/delete actions, multiline commands, low-confidence candidates, and alias-name conflicts are excluded from alias files.

The tool is still analyzing personal history. Treat generated reports as private until reviewed.

## Release and Maintenance Docs

- [Privacy notes](docs/privacy.md)
- [Troubleshooting](docs/troubleshooting.md)
- [Release process](docs/release.md)
- [Maintenance notes](docs/maintenance.md)
