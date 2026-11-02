# Architecture

This repo is organized around a public `pkg/codeguard` facade plus focused internal packages:

- `cmd/` for executable entrypoints
- `internal/` for CLI wiring (`internal/cli/`, including the MCP server), version metadata, release notes (`whatsnew`), the GitHub Action comment publisher, and benchmark tooling
- `pkg/codeguard/` as the stable public SDK facade
- `internal/codeguard/core/` for shared domain types
- `internal/codeguard/config/` for config defaults, load/save, and validation
- `internal/codeguard/checks/` for built-in check implementations grouped by category (see [Check package layout](#check-package-layout))
- `internal/codeguard/runner/` for scan orchestration and runner-specific subpackages such as `checks`, `cpptooling`, `custom`, `govulncheck`, and `support`
- `internal/codeguard/report/` for text, JSON, SARIF, GitHub annotation, GitHub PR comment, and CycloneDX serialization
- `internal/codeguard/rules/` for rule metadata catalog and explainability
- `internal/codeguard/initconfig/` and `internal/codeguard/contextgen/` for repository detection behind `codeguard init` and `codeguard context init|update`
- `tests/` for black-box and package-oriented tests
- `docs/` for operator and developer documentation
- `examples/` for sample configs

The public SDK stays stable for consumers at `github.com/devr-tools/codeguard/pkg/codeguard`, while the implementation is split into smaller internal units that are easier to maintain and extend.

## Check package layout

- `internal/codeguard/checks/quality/` holds maintainability and dependency checks, plus the observability and operations rules
- `internal/codeguard/checks/design/` holds architecture and layering checks
- `internal/codeguard/checks/security/` holds heuristic and vulnerability checks
- `internal/codeguard/checks/prompts/` holds prompt-safety checks
- `internal/codeguard/checks/performance/` holds performance smell and regression checks
- `internal/codeguard/checks/reliability/` and `internal/codeguard/checks/data/` hold production-readiness and data-correctness checks
- `internal/codeguard/checks/ci/` holds repository, workflow-policy, workflow-security, and delivery checks
- `internal/codeguard/checks/change/` holds diff-mode change-safety and refactor checks
- `internal/codeguard/checks/contracts/` holds API and migration compatibility checks
- `internal/codeguard/checks/agentcontext/` holds the Agent Context (agent docs) checks
- `internal/codeguard/checks/semanticreview/` holds the optional AI semantic review pass
- `internal/codeguard/checks/supplychain/` holds dependency-policy and SBOM-oriented checks
- `internal/codeguard/checks/support/` holds the shared adapter surface used by the check packages

This split keeps a single implementation path, with language-specific rules living under each category package.

## Runner package layout

- `internal/codeguard/runner/checks/` wires built-in check sections into the scan flow
- `internal/codeguard/runner/custom/` executes config-driven custom rules
- `internal/codeguard/runner/govulncheck/` isolates external vulnerability scan integration
- `internal/codeguard/runner/support/` holds runner helpers for cache, diff scope, suppressions, findings, and shared scan context

This keeps the runner tree organized with the same directory-first style as `internal/codeguard/checks/`, instead of accumulating more flat `runner_*.go` files.

## Current check behavior

- `quality` enforces parseability, `gofmt` cleanliness, AST-derived maintainability thresholds, cyclomatic complexity, and dependency-direction checks
- `design` enforces configurable layer boundaries between `cmd/`, `internal/`, and the scanned repository's public `pkg/` facade (`design_rules.sdk_facade_paths`, or discovered from the layout), plus principle checks for separation of concerns, clean-code naming, and SOLID-oriented heuristics
- `prompts` discovers prompt-oriented files, agent instruction files, Claude Code settings, and MCP configs, and enforces checks for secret interpolation, unsafe instructions, and agent/MCP configuration risk
- `ci` enforces configurable repository policy for workflow directories, workflow files, workflow contents, release files, and automation entrypoints (all off until configured), plus GitHub Actions workflow-security rules
- `security` runs local heuristic scanning first and can optionally run `govulncheck` in `off`, `auto`, or `required` mode with per-vulnerability findings when output is available
- custom rule packs add config-driven path and content policies without changing the Go scanner
- natural-language custom rules are compiled in `internal/codeguard/ai/nlrule/` and evaluated through an optional external AI runtime command
- policy profiles apply preset defaults for thresholds and scan posture
- exclusions remove files or paths from scanning before checks run
- waivers and inline suppressions allow time-bounded exceptions
- baselines suppress known findings so new regressions are the only gate failures
- cached file findings are keyed by file hash plus config hash so repeat scans skip unchanged files; evaluators that read other files declare them (`checks/support.FileScanDependencies`), and a per-directory digest of those inputs - sibling package sources, go.mod/go.work, resolved target state such as the SDK facade - is computed once per pass and stored with each entry
- a per-scan file corpus (`internal/codeguard/runner/support/corpus.go`) memoizes the target directory walk, individual file reads, and Go/tree-sitter parses so each file is walked, read, and parsed at most once per scan no matter how many sections inspect it; the quality AI checks share it through the `ListTargetFiles`/`ReadTargetFile`/`ParseGoFile` context hooks rather than re-reading the tree
- diff mode can scope file findings down to changed lines derived from `git diff`
- report serialization supports plain text, structured JSON, SARIF, GitHub workflow annotations, a GitHub PR comment, and CycloneDX SBOMs
