# codeguard

Static-analysis CLI that scans repositories (full or PR-diff mode) across check
sections — quality, performance, design, security, prompts, CI, supply chain,
agent context — and reports pass/warn/fail findings per rule.

## Build, test, verify

- `make build` → `dist/codeguard`; `make test`; `make fmt` (gofmt the tree); `make lint` (vet) and `make lint-strict` (golangci-lint, CI-blocking, must be 0 issues)
- `make codeguard-ci` — validate + self-scan with `.codeguard/codeguard.yaml`
- `make check` — the full CI gate (fmt-check, lint, test, codeguard-ci)
- Scope gofmt to `gofmt -l cmd internal pkg tests changelog.go` (repo root picks up worktrees/module caches)
- Tests live under `tests/` as external `_test` packages only — never next to the code (enforced by the repo's own `ci.test-file-location` rule)

## Layout

- `internal/codeguard/checks/<section>/` — check implementations (one package per section)
- `internal/codeguard/runner/` — orchestration, registry, caching; `internal/codeguard/rules/` — rule metadata catalog
- `internal/codeguard/config/` — config load/defaults/validation; `internal/cli/` — command handlers; `pkg/codeguard/` — public SDK facade
- `docs/checks.md` — authoritative user-facing rule/config reference; update it when adding or changing rules

## Available Context

Additional context is available in the files below. Consult the relevant file when working in a related area — see each description for scope.

- `.claude/knowledge/local-dev-setup.md` — Local Development Setup: toolchain quirks, self-scan cache location, common setup issues.
- `.claude/knowledge/testing-patterns.md` — Testing Patterns: trust-policy TestMains, external-test-package rule, secret-fixture assembly, MCP test harnesses, catalog rule test requirements.
- `.claude/knowledge/architecture-boundaries.md` — Architecture & System Boundaries: trust gating for config-supplied commands, safehttp, scanner hardening invariants, section wiring checklist.
- `.claude/knowledge/deployment-release.md` — Deployment & Release: release-please/GoReleaser flow, cosign pinning, npm/PyPI trusted publishing.
