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

- `quality` enforces parseability, `gofmt` cleanliness, AST-derived maintainability thresholds, cyclomatic complexity, and dependency-direction checks
- `design` enforces configurable layer boundaries between `cmd/`, `internal/`, and reusable `codeguard/` packages
- `prompts` discovers prompt-oriented files and enforces configurable checks for secret interpolation and unsafe instruction patterns
- `ci` enforces configurable repository policy for workflow directories, workflow files, release files, and automation entrypoints
- `security` runs local heuristic scanning first and can optionally run `govulncheck` in `off`, `auto`, or `required` mode with per-vulnerability findings when output is available
