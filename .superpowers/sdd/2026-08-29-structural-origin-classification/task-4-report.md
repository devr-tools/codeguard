# Task 4 Report — Named Regressions, Invariants, and crumb-app Acceptance

- status: DONE_WITH_CONCERNS — repository verification passes; the frozen crumb-app count gates do not
- commit: this commit (`test: lock structural origin classification acceptance`)

## Coverage delivered

- Added reduced, source-shaped Go regressions for badge normalization, brand patching, place updates, `OvertureDatasetPath`, `NewStaticOvertureDivisionResolver`, and `duckDBDivisionHierarchy`.
- Added reduced C++ regressions for a local fluent builder and `DbRow::integer`.
- Asserted that every named local/value case emits no structural finding and that genuinely unresolved operations remain diagnostic-only (Go: 3; C++: 1 in the fixtures).
- Added proven Go and C++ receiver, argument/reference, global, and escaped mutations. Each invariant checks `function.hidden-mutation`, the exact mutation target, `effect_kind=shared_state`, and the expected `origin` (`caller_owned` or `shared`).
- Added baseline upgrade coverage proving that diagnostic prose, confidence, and evidence metadata do not change finding identity. A prior exact fingerprint plus current context/content identities remains active through real audit semantics, while a resolved false positive becomes the sole stale/prunable entry.
- Migrated three stale tests that expected guessed ownership for unknown React Native/Go collaborators. They now enforce the Task 1–3 contract: no structural finding without ownership proof and a language-specific unresolved diagnostic instead.
- Removed unused compatibility helpers and parameters and clarified two local names so the repository-pinned linter remains clean after the Task 2/3 integration.
- Added the requested changelog entry for declaration-based Go/C++ ownership, unresolved diagnostics, and wording-independent fingerprints.

## RED evidence

Before the new fixture files existed:

`GOCACHE=/private/tmp/codeguard-go-cache szr proxy go test ./tests/checks -run 'TestStructuralOrigin|TestBaselineAuditAndPruneKeep' -count=1 -v`

- Both structural-origin fixture tests failed: no expected unresolved diagnostic was present and none of the eight proven mutation cases could be found.
- The baseline upgrade invariant passed immediately because it locks the Task 1 identity/audit behavior already consumed by this task; it required no production change.

The first fixture run then exposed three useful expectation errors before becoming green:

- unresolved fixture counts were Go 3 and C++ 1, not the provisional values;
- a generic lookup selected a command/query finding before the required hidden-mutation finding;
- the C++ escaped-local fixture's first mutation was the global escape store, so the fixture was reduced to make the escaped local mutation deterministic.

## GREEN evidence

- `GOCACHE=/private/tmp/codeguard-go-cache szr go test ./tests/checks -run 'TestStructuralOrigin|TestBaselineAuditAndPruneKeep' -count=1`
  - PASS: 1 package.
- `GOCACHE=/private/tmp/codeguard-go-cache szr go test ./internal/codeguard/checks/quality ./internal/codeguard/checks/support ./tests/checks ./tests/support -count=1`
  - PASS: 4 packages.
- `GOCACHE=/private/tmp/codeguard-go-cache szr go test ./...`
  - PASS: 27 packages.
- `GOCACHE=/private/tmp/codeguard-go-cache szr go vet ./...`
  - PASS.
- `GOMODCACHE=/private/tmp/codeguard-gomodcache GOCACHE=/private/tmp/codeguard-lint-go-cache-final GOLANGCI_LINT_CACHE=/private/tmp/codeguard-lint-cache-final go run github.com/golangci/golangci-lint/v2/cmd/golangci-lint@v2.12.2 run`
  - PASS: `0 issues.`
- `szr git diff --check`
  - PASS.
- `GOCACHE=/private/tmp/codeguard-go-cache szr go build -o /private/tmp/codeguard-structural-origin ./cmd/codeguard`
  - PASS. Go emitted a non-fatal module stat-cache permission warning; the requested binary was produced and executed successfully.

## crumb-app acceptance

The command shape was confirmed read-only from `scan --help`, `baseline audit --help`, `baseline prune --help`, and the checked-in crumb-app configuration before execution. From `/Users/agentcarl/Documents/Github/crumb-app`:

`/private/tmp/codeguard-structural-origin scan -config .codeguard/codeguard.yaml -profile startup -mode full -format json -set cache.enabled=false -set checks.security_rules.govulncheck_mode=off`

`/private/tmp/codeguard-structural-origin baseline audit -config .codeguard/codeguard.yaml -profile startup -mode full -format json -set cache.enabled=false -set checks.security_rules.govulncheck_mode=off`

`/private/tmp/codeguard-structural-origin baseline prune -check -config .codeguard/codeguard.yaml -profile startup -mode full -format json -set cache.enabled=false -set checks.security_rules.govulncheck_mode=off`

The scan and audit exited 0. The prune check exited 1 because stale entries exist, as documented by its check-only semantics, and emitted the same count report without changing the baseline. crumb-app remained clean on `main` at `8ffc82b295b1bd6358a778d6dabacd1fdccc712b`.

Observed scan counts:

| Measure | Go | C++ | Total |
|---|---:|---:|---:|
| Unsuppressed findings | 130 | 203 (88 `.cpp`, 115 `.hpp`) | 333 |
| Unresolved mutation symbols | 1,965 | 3,367 | 5,332 |

- suppressed findings: 5,450
- scan sections: 5 pass, 2 warn, 0 fail
- baseline before: 6,560
- active: 5,316 (`active_context`: 5,316; exact/content: 0)
- stale: 1,244
- prune-check simulated removal: 1,244; simulated final: 5,316; invalid: 0
- genuine structural negatives in the CodeGuard fixtures: PASS (eight exact rule/metadata assertions)

## Concerns

- The required crumb-app gates are **not satisfied**: 333 unsuppressed findings exceeds the maximum of 19, and 1,244 stale entries differs from the required 887. These are the exact results from the requested branch binary, flags, clean external checkout, and mutation-free audit/prune-check workflow.
- Unresolved evidence exists for both analyzed language groups as required, but at substantial volume: Go 1,965 and C++ 3,367. Unsupported declarations/macros and intentionally conservative unknown ownership remain diagnostic-only rather than becoming guessed structural findings.
- The acceptance mismatch should be reconciled against the frozen crumb-app revision/baseline or investigated as a separate behavior change; this task does not alter crumb-app or rewrite its baseline to manufacture the expected totals.
