# Privacy Notes

`cmd-mint` is built around local-only shell history analysis.

## What Stays Local

- Shell history files are read from disk on the local machine.
- Alias conflict checks read only a small allowlist of shell config files.
- Aggregates, rankings, Markdown, alias snippets, and optional JSON are produced locally.
- No network calls are made by the application.

## What cmd-mint Never Does

- It does not execute commands from history.
- It does not upload history.
- It does not create accounts.
- It does not collect telemetry.
- It does not modify shell history.
- It does not modify shell config.
- It does not install aliases.
- It does not write raw parser debug dumps.

## Sensitive Command Handling

The MVP does not redact individual sensitive commands. Instead, high-confidence sensitive commands are skipped entirely from generated Markdown, alias files, JSON, warnings, and terminal output.

Sensitive-looking commands contribute only to aggregate counts such as:

```text
Sensitive-looking commands skipped: 18
```

Examples of sensitive signals include password or token assignments, bearer auth headers, private keys, `.env` files, kubeconfig references, database URLs with embedded credentials, private URLs with credentials, and clipboard/keychain extraction.

## Safe Output Is Still Private

Even safe commands can reveal private information, such as repository names, tools, namespaces, local paths, branch names, hostnames, or work habits. Treat generated reports as private unless reviewed.

## File Permissions

`cmd-mint` creates report directories with `0700` permissions and generated files with `0600` permissions where the platform supports POSIX permissions.
