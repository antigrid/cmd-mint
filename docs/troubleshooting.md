# Troubleshooting

## No Supported Shell History Files Found

`cmd-mint` checks:

- `~/.zsh_history`
- `~/.bash_history`
- `~/.local/share/fish/fish_history`
- `HISTFILE`, when set

Use an explicit file if your history is elsewhere:

```sh
cmd-mint --history-file ~/.custom_history --shell zsh
```

Use `--shell auto` when you are unsure of the format:

```sh
cmd-mint --history-file ./old_history --shell auto
```

## A History File Was Skipped

A file can be skipped if it is not readable, is a directory, is not a regular file, or uses an unsupported format. Explicit `--history-file` paths must be readable regular files.

## Unexpected Positional Argument

`cmd-mint` accepts flags only. Pass history paths with `--history-file PATH`; a bare path is rejected as an invalid argument.

```sh
cmd-mint --history-file ~/.zsh_history --shell zsh
```

## No Alias Suggestions Were Generated

This can happen when commands are below the frequency threshold, commands are too short, aliases would not save enough typing, suggestions conflict with existing aliases or common commands, or commands are risky/destructive.

Try:

```sh
cmd-mint --min-frequency 2
```

The cheat sheet can still contain useful command groups and patterns even when alias files contain only headers.

## Fish Multiline Commands

Fish history block scalars such as `- cmd: |` and `- cmd: >` are parsed as multiline commands. They can appear in the cheat sheet as safe aggregates after filtering, but multiline commands are excluded from alias files.

## Fish Aliases Were Not Generated

`aliases.suggested.fish` is generated only when fish history is detected or `--shell fish` is used, unless `--no-alias-file` is set.

```sh
cmd-mint --history-file ~/.local/share/fish/fish_history --shell fish
```

## Output Directory Error

`--output-dir` must not point to an existing regular file. It may point to an existing directory. Known artifact files in that directory can be overwritten atomically.

```sh
cmd-mint --output-dir ./cmd-mint-report
```

## Sensitive Commands Are Missing From Reports

That is expected. The MVP skips sensitive-looking commands entirely instead of redacting them. Aggregate skip counts are shown in the terminal summary and cheat sheet.

## Reports Contain Private Workflow Details

This is expected because the tool summarizes shell history. Review generated files before sharing or committing them.

## Exit Codes

| Code | Meaning |
|---:|---|
| `0` | Completed successfully. |
| `1` | No readable or parseable history files found. |
| `2` | Invalid CLI arguments. |
| `3` | Output directory could not be created or written. |
| `4` | Unexpected internal error. |
