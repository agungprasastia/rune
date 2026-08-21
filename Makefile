# Rune build/test/lint targets. AGENTS.md says to build and run quality checks
# with `make` — these targets back those instructions.
.DEFAULT_GOAL := build
GO_VERSION = $(word 2,$(shell git grep -G -h "^go[[:space:]]" -- go.mod))
GO_TOOLCHAIN = go$(GO_VERSION)
DEADCODE_VERSION := v0.46.0
GOLANGCI_LINT_VERSION := v2.12.2
GOVULNCHECK_VERSION := v1.3.0

.PHONY: build build-all test test-race vet fmt fmt-check lint lint-static deadcode vulncheck tidy clean baseline help

# Build the main CLI binary into ./rune.
build:
	go build -o rune ./cmd/rune

# Build every command in cmd/.
build-all:
	go build ./...

# Run the full test suite with the race detector (matches CI expectations).
test:
	go test ./... -race -count=1

# Faster, no race detector.
test-quick:
	go test ./...

vet:
	go vet ./...

fmt:
	gofmt -w $(shell git ls-files '*.go')

# Fail if any tracked Go file is not gofmt-clean.
fmt-check:
	@out="$$(gofmt -l $$(git ls-files '*.go'))"; \
	if [ -n "$$out" ]; then echo "gofmt needed:"; echo "$$out"; exit 1; fi

# Lint = formatting check + vet (no extra tooling required).
lint: fmt-check vet

# Versioned tools select the toolchain from their own modules when invoked with
# package@version. Read this module's go directive directly, without invoking
# the possibly stale Go toolchain or consulting a multi-module GOWORK. git grep
# works with both POSIX shells and cmd.exe, including GNU Make 3.81 on macOS.
# The target-specific export is shell-independent.
lint-static deadcode vulncheck: export GOTOOLCHAIN = $(GO_TOOLCHAIN)

lint-static:
	go run github.com/golangci/golangci-lint/v2/cmd/golangci-lint@$(GOLANGCI_LINT_VERSION) run --enable-only unused,ineffassign,staticcheck ./...

deadcode:
	go run golang.org/x/tools/cmd/deadcode@$(DEADCODE_VERSION) -test=false ./...

vulncheck:
	go run golang.org/x/vuln/cmd/govulncheck@$(GOVULNCHECK_VERSION) ./...

tidy:
	go mod tidy

clean:
	rm -f rune
	go clean ./...

# Run the per-turn benchmark harness over the checked-in baseline manifest and
# write the JSON result to internal/perfbench/reports/baseline.json. Requires a
# built `rune` binary and a model; set RUNE_BENCH_MODEL (required) and
# RUNE_BENCH_BINARY (defaults to ./rune) to configure the run. The report is
# machine-specific and regenerated, not hand-edited.
baseline: build
	@if [ -z "$(RUNE_BENCH_MODEL)" ]; then echo "Set RUNE_BENCH_MODEL (and optionally RUNE_BENCH_BINARY) before running 'make baseline'"; exit 2; fi
	@RUNE_BIN="$${RUNE_BENCH_BINARY:-./rune}"; \
	go run ./cmd/rune-perf-bench turn \
		--suite internal/perfbench/manifests/baseline.json \
		--model $(RUNE_BENCH_MODEL) \
		--binary "$$RUNE_BIN" \
		--output internal/perfbench/reports/baseline.json

help:
	@echo "Targets: build (default), build-all, test, test-quick, vet, fmt, fmt-check, lint, lint-static, deadcode, vulncheck, tidy, clean, baseline"
