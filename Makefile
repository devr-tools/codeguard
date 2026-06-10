GO ?= go
GOFMT ?= gofmt
ifeq ($(strip $(GOROOT)),)
else ifeq ($(wildcard $(GOROOT)),)
GO := env -u GOROOT $(GO)
GOFMT := env -u GOROOT $(GOFMT)
endif
GOCACHE ?= $(CURDIR)/.gocache
GOMODCACHE ?= $(CURDIR)/.gomodcache
CONFIG ?= examples/codeguard.json
CI_CONFIG ?= codeguard-ci.json
BASE_REF ?= main
GOFILES := $(shell find cmd codeguard internal tests -type f -name '*.go' 2>/dev/null)

export GOCACHE
export GOMODCACHE

.DEFAULT_GOAL := help

.PHONY: help fmt fmt-check lint test codeguard-ci check ci build table table-diff table-interactive clean

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
	$(GO) test ./...

codeguard-ci:
	$(GO) run ./cmd/codeguard validate -config $(CI_CONFIG)
	$(GO) run ./cmd/codeguard scan -config $(CI_CONFIG)

check: fmt-check lint test codeguard-ci

ci: check build

build:
	@mkdir -p dist
	$(GO) build -trimpath -o ./dist/codeguard ./cmd/codeguard

table:
	$(GO) run ./cmd/codeguard scan -config $(CONFIG)

table-diff:
	$(GO) run ./cmd/codeguard scan -config $(CONFIG) -mode diff -base-ref $(BASE_REF)

table-interactive:
	$(GO) run ./cmd/codeguard scan -interactive

clean:
	rm -rf .gocache .gomodcache dist
