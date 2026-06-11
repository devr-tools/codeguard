# Architecture

This repo is organized around a public `pkg/codeguard` facade plus focused internal packages:

- `cmd/` for executable entrypoints
- `internal/` for CLI wiring and version metadata
- `pkg/codeguard/` as the stable public SDK facade
- `internal/codeguard/core/` for shared domain types
- `internal/codeguard/config/` for config defaults, load/save, and validation
- `internal/codeguard/checks/` for built-in check implementations grouped by category (`quality`, `design`, `security`, `prompts`, `ci`)
- `internal/codeguard/runner/` for scan orchestration and runner-specific subpackages such as `checks`, `custom`, `govulncheck`, and `support`
- `internal/codeguard/report/` for text, JSON, SARIF, and GitHub annotation serialization
- `internal/codeguard/rules/` for rule metadata catalog and explainability
- `tests/` for black-box and package-oriented tests
- `docs/` for operator and developer documentation
- `examples/` for sample configs

The public SDK stays stable for consumers at `github.com/devr-tools/codeguard/pkg/codeguard`, while the implementation is split into smaller internal units that are easier to maintain and extend.

## Check package layout

- `internal/codeguard/checks/quality/` holds maintainability and dependency checks
- `internal/codeguard/checks/design/` holds architecture and layering checks
- `internal/codeguard/checks/security/` holds heuristic and vulnerability checks
- `internal/codeguard/checks/prompts/` holds prompt-safety checks
- `internal/codeguard/checks/ci/` holds repository and workflow policy checks
- `internal/codeguard/checks/support/` holds the shared adapter surface used by the check packages

This split keeps a single implementation path while leaving room to add language-specific rules under each category as the scanner expands beyond Go.

## Runner package layout

- `internal/codeguard/runner/checks/` wires built-in check sections into the scan flow
- `internal/codeguard/runner/custom/` executes config-driven custom rules
- `internal/codeguard/runner/govulncheck/` isolates external vulnerability scan integration
- `internal/codeguard/runner/support/` holds runner helpers for cache, diff scope, suppressions, findings, and shared scan context

This keeps the runner tree organized with the same directory-first style as `internal/codeguard/checks/`, instead of accumulating more flat `runner_*.go` files.

## Current check behavior

- `quality` enforces parseability, `gofmt` cleanliness, AST-derived maintainability thresholds, cyclomatic complexity, and dependency-direction checks
- `design` enforces configurable layer boundaries between `cmd/`, `internal/`, and reusable `pkg/codeguard/` packages, plus principle checks for separation of concerns, clean-code naming, and SOLID-oriented heuristics
- `prompts` discovers prompt-oriented files and enforces configurable checks for secret interpolation and unsafe instruction patterns
- `ci` enforces configurable repository policy for workflow directories, workflow files, workflow contents, release files, and automation entrypoints
- `security` runs local heuristic scanning first and can optionally run `govulncheck` in `off`, `auto`, or `required` mode with per-vulnerability findings when output is available
- custom rule packs add config-driven path and content policies without changing the Go scanner
- policy profiles apply preset defaults for thresholds and scan posture
- exclusions remove files or paths from scanning before checks run
- waivers and inline suppressions allow time-bounded exceptions
- baselines suppress known findings so new regressions are the only gate failures
- cached file findings are keyed by file hash plus config hash so repeat scans skip unchanged files
- diff mode can scope file findings down to changed lines derived from `git diff`
- report serialization supports plain text, structured JSON, SARIF, and GitHub workflow annotations
