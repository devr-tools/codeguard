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
GOFILES := $(shell find cmd internal pkg tests -type f -name '*.go' 2>/dev/null)

export GOCACHE
export GOMODCACHE

.DEFAULT_GOAL := help

.PHONY: help fmt fmt-check lint test codeguard-ci check ci build release release-snapshot release-check deploy commit table table-diff table-interactive clean

help:
	@printf "\ncodeguard make targets\n\n"
	@printf "  make fmt        Format Go files\n"
	@printf "  make fmt-check  Verify Go files are formatted\n"
	@printf "  make lint       Run go vet\n"
	@printf "  make test       Run the Go test suite\n"
	@printf "  make codeguard-ci  Validate and scan this repository with codeguard\n"
	@printf "  make check      Run fmt-check, lint, test, and codeguard-ci\n"
	@printf "  make ci         Run the local CI gate\n"
	@printf "  make build      Build the codeguard CLI\n"
	@printf "  make release    Build snapshot release artifacts with GoReleaser\n"
	@printf "  make release-check  Validate GoReleaser config without publishing\n"
	@printf "  make release-snapshot  Build local snapshot release artifacts\n"
	@printf "  make deploy     Alias for make release\n"
	@printf "  make commit     Create an interactive conventional commit\n"
	@printf "  make table      Run a full scan and print the terminal table\n"
	@printf "  make table-diff Run a diff scan and print the terminal table\n"
	@printf "  make table-interactive  Launch interactive terminal scanning\n"
	@printf "  make clean      Remove local build caches and dist/\n\n"

fmt:
	@test -n "$(GOFILES)" || (echo "no Go files found" && exit 1)
	$(GOFMT) -w $(GOFILES)

fmt-check:
	@unformatted="$$( $(GOFMT) -l $(GOFILES) )"; \
	if [ -n "$$unformatted" ]; then \
		echo "unformatted Go files:"; \
		printf '%s\n' "$$unformatted"; \
		exit 1; \
	fi

lint:
	$(GO) vet ./...

test:
	@set -o pipefail; $(GO) test ./... 2>&1 | grep -v '\[no test files\]'

codeguard-ci: build
	$(CODEGUARD_BIN) validate -config $(CI_CONFIG)
	$(CODEGUARD_BIN) scan -config $(CI_CONFIG)

check: fmt-check lint test codeguard-ci

ci: check

build:
	@mkdir -p dist
	$(GO) build -trimpath -o $(CODEGUARD_BIN) ./cmd/codeguard

release: release-snapshot

release-check:
	goreleaser check

release-snapshot:
	goreleaser release --snapshot --clean

deploy: release

commit:
	./scripts/commit.sh

table:
	$(GO) run ./cmd/codeguard scan -config $(CONFIG)

table-diff:
	$(GO) run ./cmd/codeguard scan -config $(CONFIG) -mode diff -base-ref $(BASE_REF)

table-interactive:
	$(GO) run ./cmd/codeguard scan -interactive

clean:
	rm -rf .gocache .gomodcache dist
