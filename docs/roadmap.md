# Roadmap Draft

This document is a parking place for post-MVP feature ideas. Items here are not current behavior, committed scope, or release promises.

Any feature that changes the local-only privacy model, writes shell configuration, sends data outside the machine, or persists history-derived raw data needs explicit product and security review before implementation.

## Post-MVP Candidates

- Interactive alias review before writing or adopting suggestions.
- Safe one-click alias installation into shell config files, with backups, previews, and rollback.
- Config files for thresholds, ignored tools, output preferences, and project-specific settings.
- Smarter redaction modes that can retain useful structure without exposing sensitive values.
- Shell functions for parameterized command patterns.
- Project-aware reports that can separate global habits from repository-local workflows.
- Recency and trend analysis across runs.
- TUI mode for reviewing commands, exclusions, aliases, and generated artifacts.
- Plugin rule packs for tools such as Terraform, AWS, Kubernetes, Docker Compose, package managers, and cloud CLIs.
- Richer alias and command conflict detection across shells, PATH commands, functions, and sourced config files.
- CI-safe JSON exports with stable schemas for automated checks.
- Homebrew, apt, and other package-manager distribution.
- Additional shell support, including PowerShell.
- Shell completion scripts.
- Optional local-only LLM explanations for command summaries and alias rationale.

## Planning Notes

- Keep alias installation opt-in and reviewable. The MVP guarantee that `cmd-mint` does not mutate shell config should remain the default unless a dedicated install flow is explicitly requested.
- Keep cloud, hosted sync, accounts, telemetry, and remote LLM analysis out of scope unless the privacy model is redesigned.
- Treat redaction as a separate feature from filtering. The MVP skips sensitive commands; a redaction mode would need tests proving secrets do not leak through Markdown, JSON, alias files, terminal output, logs, or debug paths.
- Prefer feature flags or separate commands for workflows that increase risk, such as shell config writes or persistent cross-run trend storage.
