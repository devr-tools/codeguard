GO ?= go
GOFMT ?= gofmt

.PHONY: fmt test build

fmt:
	$(GOFMT) -w cmd internal codeguard tests

test:
	$(GO) test ./...

build:
	$(GO) build ./cmd/codeguard
