
<p align="center">
  <img src="./img/cg.png" alt="codeguard placeholder banner" width="420">
</p>

`codeguard` is a standalone Go service and CLI for repository checks across code quality, design boundaries, security, CI/CD hygiene, and AI prompt governance.

## Installation

```bash
go install github.com/devr-tools/codeguard/cmd/codeguard@latest
```

Or build from source:

```bash
make build
```

## Quick Start

```bash
codeguard init
codeguard validate -config codeguard.yaml
codeguard scan -config codeguard.yaml
```

The default config path is `codeguard.yaml`. If you want a JSON starting point instead, use [examples/codeguard.json](/Users/alex/Documents/GitHub/codeguard/examples/codeguard.json:1).

## Docs

- [Getting started](/Users/alex/Documents/GitHub/codeguard/docs/getting-started.md:1)
- [Checks reference](/Users/alex/Documents/GitHub/codeguard/docs/checks.md:1)
- [Architecture](/Users/alex/Documents/GitHub/codeguard/docs/architecture.md:1)
