# Getting Started

`codeguard` is a standalone Go service and CLI for repository checks around code quality, design boundaries, security, CI/CD hygiene, and AI prompt governance.

## Install

```bash
go install github.com/devr-tools/codeguard/cmd/codeguard@latest
```

Or from this repository:

```bash
make build
```

## Quick Start

```bash
codeguard init
codeguard validate -config codeguard.yaml
codeguard scan -config codeguard.yaml
```

`codeguard init` writes `codeguard.yaml` by default.

If you prefer a JSON example, start from [examples/codeguard.json](/Users/alex/Documents/GitHub/codeguard/examples/codeguard.json:1).

## Current scope

- Go-first runtime support
- Standalone reusable package layout
- Config, runner, report, and CLI boundaries separated

Language-specific engines can be added later under the `codeguard/` package tree without changing the repo shape.
