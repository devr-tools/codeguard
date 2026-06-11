
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

Or run in Docker:

```bash
docker build -t codeguard .
docker run --rm -v "$PWD:/workspace" -w /workspace codeguard scan
```

For local release automation:

```bash
make commit
make release
make release-check
make deploy
```

The GitHub release flow follows the same branch and release-please model as `cleanr`, using `.github/workflows/cd.yml`, `.github/workflows/release.yml`, `.github/release-please-config.json`, and `.release-please-manifest.json`.

## Quick Start

```bash
codeguard init
codeguard validate -config codeguard.yaml
codeguard scan -config codeguard.yaml
```

By default, `codeguard` looks for `codeguard.yaml`, `codeguard.yml`, or `codeguard.json` in the repository root. If those are missing, it also checks `.codeguard/codeguard.yaml`, `.codeguard/codeguard.yml`, and `.codeguard/codeguard.json`.

If you point `-config` at a directory such as `.codeguard`, `codeguard` will look inside it for `codeguard.*` or `config.*` files.

Text output includes ANSI color and emoji markers by default. Set `NO_COLOR=1` if you want plain terminal output.

If you want a JSON starting point instead, use [examples/codeguard.json](/Users/alex/Documents/GitHub/codeguard/examples/codeguard.json:1).

## Docs

- [Getting started](/Users/alex/Documents/GitHub/codeguard/docs/getting-started.md:1)
- [Checks reference](/Users/alex/Documents/GitHub/codeguard/docs/checks.md:1)
- [Architecture](/Users/alex/Documents/GitHub/codeguard/docs/architecture.md:1)
