# Architecture

This repo now follows the same broad structure as `cleanr`:

- `cmd/` for executable entrypoints
- `internal/` for CLI wiring and version metadata
- `codeguard/` for reusable service code
- `tests/` for black-box and package-oriented tests
- `docs/` for operator and developer documentation
- `examples/` for sample configs

## Package boundaries

- `codeguard/config`: load, write, validate configuration
- `codeguard/core`: shared domain types
- `codeguard/runner`: orchestration for repository scans
- `codeguard/report`: presentation and serialization
- `codeguard/checks/*`: one package per check family with local policy and tooling integration

This keeps the CLI thin and makes the service reusable from other Go projects.

## Current check behavior

- `quality` enforces parseability, `gofmt` cleanliness, file-size thresholds, function-size thresholds, and parameter-count thresholds
- `security` runs local heuristic scanning first and can optionally run `govulncheck` in `off`, `auto`, or `required` mode
