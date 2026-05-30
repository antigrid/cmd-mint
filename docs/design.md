# Design Notes

This document captures the maintainer-facing contracts for the local-only MVP.

## Core Pipeline

The analyzer is intentionally staged so unsafe history-derived data is filtered before it can reach output models:

1. Parse CLI flags.
2. Discover history sources.
3. Discover existing aliases from allowlisted shell config files, read-only.
4. Parse history files into command records.
5. Run sensitivity checks against raw commands.
6. Normalize and tokenize commands.
7. Run sensitivity and risk checks against normalized commands and tokens.
8. Build safe aggregate tables.
9. Build exact-command alias candidates.
10. Build pattern groups for the cheat sheet only.
11. Apply alias naming and conflict detection.
12. Score and rank aliases.
13. Generate the terminal summary.
14. Write artifacts with restrictive permissions.

Sensitive commands must be excluded from all generated command output if either the raw pass or normalized/token pass detects high-confidence sensitivity. They may contribute only to aggregate skipped counts and categories.

## Raw Command Lifetime

Raw command strings may exist in memory while parsing, filtering, and normalization run. They must not be written to:

- logs,
- debug files,
- crash reports,
- generated Markdown,
- generated alias files,
- generated JSON,
- persistent caches,
- temporary files.

Output renderers should consume safe, post-filtered report models rather than raw parser records.

## Parsing and Normalization

Parsers should extract one command string per history entry when possible without implementing a full shell parser. They must not execute, expand, evaluate, glob, resolve, or otherwise interpret shell commands.

Normalization should stay conservative:

- Trim leading and trailing whitespace.
- Collapse repeated unquoted whitespace.
- Remove shell history metadata.
- Preserve quoted strings and argument order.
- Preserve `sudo` in normalized commands and alias expansions.
- Do not rewrite paths, branch names, file names, namespaces, image names, or similar user-specific values for exact-command ranking.
- Do not strip comments unless they are clearly identifiable outside quotes.

For grouping, harmless wrappers such as `time`, `env`, `noglob`, `command`, and `builtin` can be skipped to identify the underlying tool. `sudo` may be ignored for grouping, but it must remain in the command and risk analysis.

## Alias Suggestions

Alias suggestions are exact-command aliases only. Do not emit parameterized shell functions as alias artifacts.

A command is eligible only when it is:

- at or above `--min-frequency`,
- long enough and savings-positive enough to justify an alias,
- non-sensitive,
- non-risky,
- single-line,
- successfully parsed,
- free of existing alias-name and common-command conflicts,
- not highly project-specific unless repeated enough to be clearly useful.

Prefer a small conservative convention table for common commands, then deterministic fallback abbreviations. Low-confidence suggestions can appear in the cheat sheet when useful, but should not be written to alias files by default.

## Risk and Sensitivity Boundaries

Sensitivity filtering protects generated command output. High-confidence secrets, credential files, auth headers, private keys, database URLs with credentials, kubeconfig secrets, clipboard/keychain extraction, package publishing credentials, and private URLs containing credentials must be excluded entirely.

Risk filtering is stricter for aliases than for the cheat sheet. Destructive or production-like commands may be shown only as observed risky commands when non-sensitive, and must not become aliases.

## Deterministic Ordering

Generated output ordering must be deterministic for the same inputs and flags. When ranking aliases with equal scores, use stable tie breakers such as:

1. Score descending.
2. Frequency descending.
3. Estimated total characters saved descending.
4. Estimated characters saved per use descending.
5. Tool name ascending.
6. Normalized command ascending.
7. Alias name ascending.

The default generation timestamp and default timestamped output directory are intentionally run-specific. Use both `--output-dir` and `--generated-at` for byte-for-byte reproducible artifacts.

## Extension Guardrails

Keep these boundaries unless there is an explicit product decision to change them:

- No telemetry or network calls.
- No command execution from shell history.
- No shell history mutation.
- No shell config mutation.
- No automatic alias installation.
- No config file, cache, database, GUI, TUI, cloud path, or account system in the MVP.
- Tests should accompany parser, privacy, risk, ordering, permissions, and output-contract changes.
