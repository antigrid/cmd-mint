BINARY ?= cmd-mint
VERSION ?= dev
COMMIT ?= $(shell git rev-parse --short HEAD 2>/dev/null || echo unknown)
DATE ?= $(shell date -u +%Y-%m-%dT%H:%M:%SZ)
LDFLAGS := -X cmd-mint/internal/version.Version=$(VERSION) -X cmd-mint/internal/version.Commit=$(COMMIT) -X cmd-mint/internal/version.Date=$(DATE)

.PHONY: build test bench smoke

build:
	go build -trimpath -ldflags "$(LDFLAGS)" -o $(BINARY) ./cmd/cmd-mint

test:
	go test ./...

bench:
	go test -run '^$$' -bench BenchmarkBuildReportSynthetic100k -benchmem ./internal/analyze

smoke: build
	mkdir -p .tmp/smoke-home
	HOME=$(CURDIR)/.tmp/smoke-home HISTFILE= SHELL= ./$(BINARY) \
		--history-file internal/testdata/e2e/mvp/zsh_history \
		--history-file internal/testdata/e2e/mvp/bash_history \
		--history-file internal/testdata/e2e/mvp/fish_history \
		--shell auto \
		--output-dir .tmp/smoke-report \
		--json \
		--verbose
