# Release Process

This document describes the MVP release process for `cmd-mint`. The MVP ships as local-only macOS and Linux binaries.

## Targets

Publish prebuilt binaries for:

- `darwin/arm64`
- `darwin/amd64`
- `linux/amd64`
- `linux/arm64`

Homebrew or other package-manager distribution can be added after the initial MVP release. It is not required for MVP installation.

## Pre-Release Checks

Run the full test suite:

```sh
go test ./...
```

Check the CLI help and version output:

```sh
go run ./cmd/cmd-mint --help
go run ./cmd/cmd-mint --version
```

Confirm the README still documents every MVP flag:

- `--history-file`
- `--shell`
- `--output-dir`
- `--max-aliases`
- `--min-frequency`
- `--no-alias-file`
- `--json`
- `--verbose`
- `--version`
- `--help`

Confirm the docs still state the local-only privacy requirements:

- No network calls.
- No telemetry.
- No command execution from shell history.
- No shell config modification.
- No shell history modification.
- Generated reports may reveal private workflows.
- Users should review generated files before sharing, committing, or adopting aliases.

## Version Metadata

Release builds should set version metadata with Go linker flags. The binary should print version, commit, and build date through:

```sh
cmd-mint --version
```

Example variables:

```sh
VERSION=0.1.0
COMMIT=$(git rev-parse --short HEAD)
DATE=$(date -u +%Y-%m-%dT%H:%M:%SZ)
LDFLAGS="-s -w -X cmd-mint/internal/version.Version=$VERSION -X cmd-mint/internal/version.Commit=$COMMIT -X cmd-mint/internal/version.Date=$DATE"
```

## Build Binaries

Build from the repository root:

```sh
mkdir -p dist

GOOS=darwin GOARCH=arm64 go build -trimpath -ldflags "$LDFLAGS" -o dist/cmd-mint-darwin-arm64 ./cmd/cmd-mint
GOOS=darwin GOARCH=amd64 go build -trimpath -ldflags "$LDFLAGS" -o dist/cmd-mint-darwin-amd64 ./cmd/cmd-mint
GOOS=linux GOARCH=amd64 go build -trimpath -ldflags "$LDFLAGS" -o dist/cmd-mint-linux-amd64 ./cmd/cmd-mint
GOOS=linux GOARCH=arm64 go build -trimpath -ldflags "$LDFLAGS" -o dist/cmd-mint-linux-arm64 ./cmd/cmd-mint
```

Generate checksums:

```sh
cd dist
shasum -a 256 cmd-mint-* > checksums.txt
```

## Smoke Test Artifacts

For at least one local platform binary:

```sh
./dist/cmd-mint-linux-amd64 --version
./dist/cmd-mint-linux-amd64 --help
```

On macOS, use the matching `darwin` binary. For Linux, use the matching `linux` binary.

When testing with real shell history, keep generated reports local and review them before sharing. Do not upload generated reports as release assets.

## Publish

1. Tag the release, for example `v0.1.0`.
2. Upload the four binaries and `checksums.txt` to the release.
3. Include installation instructions that use manual binary installation:

```sh
chmod +x cmd-mint
mv cmd-mint /usr/local/bin/cmd-mint
cmd-mint --version
```

4. Note that package-manager installation is not required for the MVP.
5. Note that `cmd-mint` runs locally and does not send shell history anywhere.

## Release Notes Checklist

Every release note should include:

- Supported platforms.
- Installation steps.
- `cmd-mint --version` verification step.
- Privacy reminder for generated reports.
- Known limitations or non-goals that remain relevant.
