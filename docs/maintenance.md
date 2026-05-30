# Maintenance Notes

`cmd-mint` is intentionally modular so new shells, filters, alias conventions, and output formats can be added without weakening the local-only MVP guarantees.

See [Design Notes](design.md) for pipeline, raw command lifetime, alias eligibility, and deterministic ordering contracts. Use [Roadmap Draft](roadmap.md) for post-MVP feature planning.

## Safety Invariants

Preserve these invariants when changing the code:

- Do not add network calls.
- Do not add telemetry.
- Do not execute commands from shell history.
- Do not modify shell history files.
- Do not modify shell config files.
- Do not persist raw command records outside process memory.
- Do not print or write raw sensitive skipped commands.
- Do not add an MVP `--include-sensitive` override.
- Keep generated JSON safe and post-filtered.

If a feature needs any of those behaviors, it is outside the MVP scope and needs separate product review.

## Extending Parsers

Shell parsers should extract one command string per history entry without executing, expanding, globbing, or evaluating the command.

When adding or changing a parser:

- Preserve source metadata: shell, source file, entry counts, parsed counts, skipped counts, and warnings.
- Continue after malformed entries when possible.
- Add fixtures for plain, timestamped, malformed, empty, and multiline cases.
- Keep raw command strings in memory only long enough for safety checks and normalization.
- Avoid retaining full raw history slices for large files.

## Extending Sensitivity Rules

Sensitivity rules decide what must be excluded from all generated command output.

When adding rules:

- Prefer high-confidence patterns over broad guesses.
- Count skipped commands by safe aggregate reason.
- Do not write raw skipped commands into Markdown, JSON, alias files, logs, or verbose output.
- Add tests with fake secrets only.
- Check generated artifacts for fake secret strings in tests when practical.

Sensitive categories include passwords, tokens, API keys, auth headers, cookies, private keys, certificates, `.env` files, credential files, kubeconfig secrets, database URLs with credentials, clipboard/keychain extraction, package publishing credentials, and private URLs containing credentials.

## Extending Risk Rules

Risk rules are stricter for alias suggestions than for the cheat sheet because aliases make commands easier to run.

When adding risk rules:

- Exclude destructive and high-risk commands from alias suggestions.
- Keep non-sensitive risky commands eligible for safe aggregate cheat-sheet summaries only when appropriate.
- Treat production-like deploy/delete/destroy workflows conservatively.
- Add tests for both alias exclusion and cheat-sheet behavior.

Examples of risky commands include `rm -rf`, `sudo rm`, `chmod -R`, `docker system prune`, `kubectl delete`, `terraform destroy`, `drop database`, and production-like deploy or delete commands.

## Extending Alias Conventions

Alias suggestions are exact-command aliases in the MVP.

When changing alias conventions:

- Keep output deterministic.
- Keep aliases short but understandable.
- Preserve alias-name conflict detection.
- Exclude multiline, sensitive, destructive, low-frequency, and low-confidence candidates.
- Use safe shell quoting for alias expansions.
- Avoid project-specific aliases unless frequency and scoring clearly justify them.
- Do not add automatic installation as an MVP behavior.

## Extending Output Renderers

Output renderers must consume safe, post-filtered models. They should not have access to raw command records.

When changing renderers:

- Keep `cheatsheet.md` human-readable and explicit about privacy.
- Keep alias file headers warning users to review before copying or sourcing.
- Keep `report.json` safe and machine-readable.
- Do not include comments containing sensitive command data.
- Preserve restrictive file permissions where supported.
- Keep output ordering deterministic except for generation timestamps and default report directory names.
