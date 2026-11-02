SHELL := /bin/bash

GO ?= go
GOFMT ?= gofmt
ifeq ($(origin GOROOT),environment)
GO := env -u GOROOT $(GO)
GOFMT := env -u GOROOT $(GOFMT)
else ifeq ($(origin GOROOT),environment override)
GO := env -u GOROOT $(GO)
GOFMT := env -u GOROOT $(GOFMT)
endif
VERSION ?= dev
REPOSITORY ?= devr-tools/codeguard
GOCACHE ?= $(CURDIR)/.gocache
GOMODCACHE ?= $(CURDIR)/.gomodcache
CONFIG ?= examples/codeguard.json
CI_CONFIG ?= .codeguard/codeguard.yaml
BASE_REF ?= main
CODEGUARD_BIN ?= ./dist/codeguard
GOFILE_DIRS := cmd internal pkg tests
# MENU_VERSION defaults to the current release-please version so `make menu`
# previews the banner with the real version (like a release build) instead of
# the source default; override with MENU_VERSION=x.y.z.
MANIFEST_VERSION := $(shell grep -oE '[0-9]+\.[0-9]+\.[0-9]+' .release-please-manifest.json 2>/dev/null | head -1)
MENU_VERSION ?= $(if $(MANIFEST_VERSION),$(MANIFEST_VERSION),$(VERSION))
MENU_LDFLAGS := -X github.com/devr-tools/codeguard/internal/version.Number=v$(MENU_VERSION)
# GRAMMAR_TAGS restricts the gotreesitter grammar registry to the languages
# codeguard parses (docs/treesitter-spike.md §5.3): the subset build embeds
# only the TypeScript/TSX/JavaScript/Python/C++ grammar blobs instead of all
# ~206 (~+22 MB). Builds without these tags (plain `go build`, `go install`)
# still work; they just embed every grammar.
GRAMMAR_TAGS := grammar_subset,grammar_subset_typescript,grammar_subset_tsx,grammar_subset_javascript,grammar_subset_python,grammar_subset_cpp
# make bench-cpu knobs (see the bench-cpu target).
BENCH ?= BenchmarkRealRepositoryFullScan
BENCH_REPOS ?=
BENCH_CONFIGS ?=
BENCH_COUNT ?= 5
BENCH_TIME ?= 1x
BENCH_PROFILE_DIR ?= $(CURDIR)/.bench

export GOCACHE
export GOMODCACHE

.DEFAULT_GOAL := help

.PHONY: help fmt fmt-check lint lint-strict test bench-cpu codeguard-ci check ci build release release-snapshot release-check deploy commit menu table table-diff table-interactive clean

help:
	@printf "\ncodeguard make targets\n\n"
	@printf "  make fmt        Format Go files\n"
	@printf "  make fmt-check  Verify Go files are formatted\n"
	@printf "  make lint       Run go vet\n"
	@printf "  make lint-strict  Run golangci-lint (enforced in CI)\n"
	@printf "  make test       Run the Go test suite\n"
	@printf "  make bench-cpu  Profile a full scan of real repo snapshots (BENCH_CONFIGS=/abs/codeguard.yaml,...)\n"
	@printf "  make codeguard-ci  Validate and scan this repository with codeguard\n"
	@printf "  make check      Run fmt-check, lint, test, and codeguard-ci\n"
	@printf "  make ci         Run the full local CI gate, including golangci-lint\n"
	@printf "  make build      Build the codeguard CLI\n"
	@printf "  make release    Build snapshot release artifacts with GoReleaser\n"
	@printf "  make release-check  Validate GoReleaser config without publishing\n"
	@printf "  make release-snapshot  Build local snapshot release artifacts\n"
	@printf "  make deploy     Alias for make release\n"
	@printf "  make commit     Create an interactive conventional commit\n"
	@printf "  make menu       Preview the What's New banner and usage menu\n"
	@printf "  make table      Run a full scan and print the terminal table\n"
	@printf "  make table-diff Run a diff scan and print the terminal table\n"
	@printf "  make table-interactive  Launch interactive terminal scanning\n"
	@printf "  make clean      Remove local build caches and dist/\n\n"

fmt:
	@find $(GOFILE_DIRS) -type f -name '*.go' -print -quit 2>/dev/null | grep -q . || (echo "no Go files found" && exit 1)
	find $(GOFILE_DIRS) -type f -name '*.go' -exec $(GOFMT) -w {} +

fmt-check:
	@unformatted="$$(find $(GOFILE_DIRS) -type f -name '*.go' -exec $(GOFMT) -l {} +)"; \
	if [ -n "$$unformatted" ]; then \
		echo "unformatted Go files:"; \
		printf '%s\n' "$$unformatted"; \
		exit 1; \
	fi

lint:
	$(GO) vet ./...

lint-strict:
	env -u GOROOT golangci-lint run

test:
	@set -o pipefail; $(GO) test ./... 2>&1 | grep -v '\[no test files\]'

# bench-cpu profiles a full scan of real repository snapshots. Point BENCH_REPOS
# at frozen snapshots (`git archive HEAD | tar -x -C <dir>`), never a live
# working tree: a tree that mutates under the walk gives different findings run
# to run. Paths must be absolute — a relative target path resolves against the
# config directory, not the shell's cwd.
#
# BENCH_CONFIGS scans with a codeguard.yaml verbatim, so the profile covers the
# same rule set as the `codeguard scan` you time with it; BENCH_REPOS uses the
# default `codeguard init` check set. Prefer BENCH_CONFIGS — the check set moves
# full-scan cost by more than 2x.
#
#   make bench-cpu BENCH_CONFIGS=/tmp/cfg/szr/codeguard.yaml
#   make bench-cpu BENCH_REPOS=/tmp/snap/szr,/tmp/snap/sentinel
#   make bench-cpu BENCH_REPOS=/tmp/snap/szr BENCH='.*/cold'
#   go tool pprof -top -cum -nodecount=10 .bench/cpu.out
bench-cpu:
	@test -n "$(BENCH_REPOS)$(BENCH_CONFIGS)" || { echo "set BENCH_CONFIGS=/abs/codeguard.yaml[,...] or BENCH_REPOS=/abs/snapshot[,...]"; exit 1; }
	@mkdir -p $(BENCH_PROFILE_DIR)
	@set -o pipefail; CODEGUARD_BENCH_REPOS='$(BENCH_REPOS)' CODEGUARD_BENCH_CONFIGS='$(BENCH_CONFIGS)' $(GO) test ./tests/codeguard \
		-tags $(GRAMMAR_TAGS) -run '^$$' -bench '$(BENCH)' -benchmem \
		-count=$(BENCH_COUNT) -benchtime=$(BENCH_TIME) \
		-cpuprofile $(BENCH_PROFILE_DIR)/cpu.out \
		-memprofile $(BENCH_PROFILE_DIR)/mem.out \
		-o $(BENCH_PROFILE_DIR)/bench.test 2>&1 | tee $(BENCH_PROFILE_DIR)/bench.txt
	@echo "profiles: $(BENCH_PROFILE_DIR)/cpu.out $(BENCH_PROFILE_DIR)/mem.out (binary $(BENCH_PROFILE_DIR)/bench.test)"

codeguard-ci: build
	$(CODEGUARD_BIN) validate -config $(CI_CONFIG)
	$(CODEGUARD_BIN) scan -config $(CI_CONFIG)

check: fmt-check lint test codeguard-ci

ci: fmt-check lint lint-strict test codeguard-ci

build:
	@mkdir -p dist
	$(GO) build -trimpath -tags $(GRAMMAR_TAGS) -o $(CODEGUARD_BIN) ./cmd/codeguard

release: release-snapshot

release-check:
	goreleaser check

release-snapshot:
	goreleaser release --snapshot --clean

deploy: release

commit:
	./scripts/commit.sh

menu:
	@printf "# codeguard menu preview (v%s); set CODEGUARD_NO_UPDATE_CHECK=1 to skip the update check\n\n" "$(MENU_VERSION)"
	$(GO) run -ldflags "$(MENU_LDFLAGS)" ./cmd/codeguard

table:
	$(GO) run ./cmd/codeguard scan -config $(CONFIG)

table-diff:
	$(GO) run ./cmd/codeguard scan -config $(CONFIG) -mode diff -base-ref $(BASE_REF)

table-interactive:
	$(GO) run ./cmd/codeguard scan -interactive

clean:
	rm -rf .gocache .gomodcache dist
