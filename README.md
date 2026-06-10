# codeguard

<p align="center">
  <img src="./img/cg.png" alt="codeguard placeholder banner" width="720">
</p>

`codeguard` is a standalone Go service and CLI for repository checks across code quality, design boundaries, security, CI/CD hygiene, and AI prompt governance.

This repo is now structured to follow the same broad layout as [`cleanr`](https://github.com/devr-tools/cleanr):

- `cmd/` for binaries
- `internal/` for CLI wiring and version state
- `codeguard/` for reusable service packages
- `tests/` for black-box coverage
- `docs/`, `examples/`, `scripts/`, and `private/` for supporting assets

## Why this shape

The goal is to split `codeguard` out from logic currently living inside `cleanr` and make it reusable across other repositories without binding check execution to a single product.

The current scaffold is Go-first by design. Additional languages can be added later as new engines under the `codeguard/` package tree.

## Commands

```bash
make test
make build
./codeguard init
./codeguard validate -config codeguard.json
./codeguard scan -config codeguard.json
```
