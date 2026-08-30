# Task 4 Report — Named Regressions, Invariants, and crumb-app Acceptance

- status: BLOCKED_BY_CONTRADICTORY_EXTERNAL_ACCEPTANCE_GATES — implementation and repository verification pass; the required crumb totals are impossible for the supplied unmodified checkout/configuration
- commits: `04398d3` (`test: lock structural origin classification acceptance`) plus this diagnosis/fix commit

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

## Resumed acceptance diagnosis and fix round

The initial count failure was not accepted as a revision assumption. A detached control worktree at the exact pre-repair base, `c077933`, was built and run against the same clean crumb-app checkout and flags. All artifacts are retained outside both repositories:

- control: `/private/tmp/crumb-structural-control-scan.json`, `/private/tmp/crumb-structural-control-baseline-audit.json`, `/private/tmp/crumb-structural-control-baseline-prune-check.json`
- initial branch: `/private/tmp/crumb-structural-origin-scan.json`, `/private/tmp/crumb-structural-origin-baseline-audit.json`, `/private/tmp/crumb-structural-origin-baseline-prune-check.json`
- final fix round: `/private/tmp/crumb-structural-origin-scan-final.json`, `/private/tmp/crumb-structural-origin-baseline-audit-final.json`, `/private/tmp/crumb-structural-origin-baseline-prune-check-final.json`

### Initial 333-finding classification

This table exhaustively partitions the initial unsuppressed result by rule and analyzed language:

| Rule | Go | C++ | Total | Recurring source pattern |
|---|---:|---:|---:|---|
| `design.persistence-model-leak` | 7 | 82 | 89 | ORM/storage records, tasks, and DB concepts exposed at boundaries |
| `function.hidden-mutation` | 25 | 60 | 85 | proven receiver/reference/global writes and calls; false lexical families investigated below |
| `function.command-query-mix` | 16 | 25 | 41 | value-returning functions with the same proven mutation evidence |
| `design.temporal-coupling` | 25 | 4 | 29 | call/state ordering dependencies |
| `design.pass-through-abstraction` | 26 | 0 | 26 | forwarding wrappers without added policy |
| `smell.message-chain` | 1 | 18 | 19 | long member/call chains |
| `design.configuration-leak` | 1 | 10 | 11 | configuration concepts crossing boundaries |
| `naming.behavior-mismatch` | 7 | 0 | 7 | query-shaped Go names with structural effects |
| `quality.hidden-side-effect` | 7 | 0 | 7 | same Go structural evidence surfaced by the quality rule |
| `design.infrastructure-type-leak` | 0 | 4 | 4 | infrastructure types exposed by APIs |
| `function.orchestration-domain-mix` | 3 | 0 | 3 | domain decisions mixed with orchestration |
| `defensive.unvalidated-boundary-input` | 2 | 0 | 2 | boundary inputs used without validation |
| `error.generic-message` | 2 | 0 | 2 | generic error prose |
| `naming.cardinality-mismatch` | 2 | 0 | 2 | scalar/collection naming mismatch |
| `defensive.integer-overflow` | 1 | 0 | 1 | unchecked integer operation |
| `error.cleanup-error-ignored` | 1 | 0 | 1 | ignored cleanup failure |
| `naming.role-suffix-overuse` | 1 | 0 | 1 | generic role suffix |
| `quality.duplicated-knowledge` | 1 | 0 | 1 | repeated encoded knowledge |
| `smell.data-clump` | 1 | 0 | 1 | repeated parameter group |
| `smell.god-object` | 1 | 0 | 1 | oversized responsibility surface |
| **Total** | **130** | **203** | **333** | |

Structural metadata was complete and non-guessed:

| Rule/language | Argument / caller-owned | Receiver / caller-owned | Global / shared | Total |
|---|---:|---:|---:|---:|
| C++ hidden mutation | 26 | 32 | 2 | 60 |
| C++ command/query | 18 | 6 | 1 | 25 |
| Go hidden mutation | 3 | 22 | 0 | 25 |
| Go command/query | 8 | 8 | 0 | 16 |

The recurring hidden-mutation evidence consisted of Go `r.pool.Exec` (14 receiver cases plus one argument `pool.Exec`), stateful service/loader receivers, and explicit map/pointer arguments. C++ was dominated by repository `context_` receiver binding (9), coroutine/member `state` writes (5), `error` and `out` references (4 each), event dispatchers and shutdown flags (3 each), and explicit reference operations such as `clear`, `assign`, and `execPrepared`. Those proven operations remain covered rather than being discarded to meet a count.

The named acceptance sources (`badge_helpers.go`, `brand_helpers.go`, `place_helpers.go`, `OvertureDatasetPath`, `NewStaticOvertureDivisionResolver`, `duckDBDivisionHierarchy`, the local fluent builders, and `DbRow::integer`) have zero findings in the initial and final branch JSON. Their reduced fixtures remain green alongside the genuine receiver/reference/global/escaped negatives.

### Concrete gaps and RED evidence

Four source-independent defects accounted for the remaining false structural family:

1. `(?i)` applied to the entire mutation/read regex, including its `[A-Z...]` camel-case boundary. Consequently `addedCellIds` matched `add` and `savedPlateIds` matched `save`.
2. Observable-effect classification inspected every token in a callee. Receiver/member words such as `savedPlateIds`, `upload`, and `prefetch` could therefore manufacture persistence/network effects even when the terminal method was read-only.
3. The C++ function-head parser interpreted `if constexpr (requires(...))` as a nested function named `constexpr`.
4. Constructor syntax and qualified method names were not honored by hidden-mutation naming: constructors appeared hidden, and `TraceSpan::setStatus` did not apply command semantics to leaf method `setStatus`.

The RED command was:

`GOCACHE=/private/tmp/codeguard-go-cache-red2 szr proxy go test ./internal/codeguard/checks/quality ./internal/codeguard/checks/support -run 'TestCppOriginReadOnlyMemberCalls|TestCppParserDoesNotTreatIfConstexpr' -count=1 -v`

- all four source-shaped read calls (`savedPlateIds.size`, `addedCellIds.size`, `savedPlateIds.contains`, and `savedPlateIds.empty`) produced caller-owned mutation evidence;
- the parser produced a nested `ParsedFunction{Name:"constexpr"}`;
- separate RED runs proved constructor initialization and `TraceSpan::setStatus` still produced hidden-mutation evidence.

The repair scopes case-insensitivity to verb tokens while keeping the camel boundary case-sensitive, classifies observable effects from terminal method identifier words, rejects the C++ keyword as a function head, recognizes constructor/destructor syntax structurally, and applies naming semantics to a qualified method's leaf name. It adds no crumb path/function/method allowlist, suppression, waiver, or baseline change.

### Final GREEN and acceptance results

- focused read/parser/constructor/qualified-name regressions: PASS (2 packages)
- required focused suite: PASS (4 packages)
- full `go test ./...`: PASS (27 packages)
- `go vet ./...`: PASS
- pinned golangci-lint v2.12.2: PASS (`0 issues.`)
- branch binary build and `git diff --check`: PASS

The final scan removed 12 initial unsuppressed false findings and added none. The removed family comprises four read-only C++ calls, two phantom `constexpr` functions, two constructors, one qualified explicit command, and three whole-callee substring effects in Go. Final exact results:

| Measure | Go | C++ | Total |
|---|---:|---:|---:|
| Unsuppressed findings | 127 | 194 | 321 |
| Unresolved mutation symbols | 1,921 | 3,113 | 5,034 |

- suppressed findings: 5,409
- baseline before: 6,560
- active: 5,275 (`active_context`: 5,275; exact/content: 0)
- stale: 1,285
- prune-check simulated removal: 1,285; simulated final: 5,275; invalid: 0
- unresolved diagnostics remain present independently for Go and C++
- crumb-app remained clean at `8ffc82b295b1bd6358a778d6dabacd1fdccc712b`

### Mathematical acceptance contradiction

The pre-repair control on the same source/configuration produced 645 unsuppressed findings, 5,807 suppressed findings, and exactly 887 stale entries. Its unsuppressed findings consist of 452 structural findings plus an unchanged **193-finding non-structural floor**:

- 159 Design Patterns findings (89 persistence-model, 29 temporal-coupling, 26 pass-through, 11 configuration-leak, 4 infrastructure-type-leak);
- 34 other quality findings, including the 19 message-chain findings.

Therefore a structural-origin repair cannot produce a total of at most 19 under the required full configuration: even eliminating every structural and structurally-derived finding leaves 193. The observed `19` is the count of `smell.message-chain` alone, not the control's total unsuppressed count.

The stale gate is independently incompatible with removing existing false positives from an unmodified baseline. With 6,560 entries, `stale = 6,560 - active`. The control has 5,673 active entries and 887 stale. The repaired result has 5,275 active and 1,285 stale. Restoring 887 requires 398 additional current findings matching distinct obsolete baseline entries. That can only be achieved by restoring removed findings or modifying/pruning crumb's baseline; both violate the requested repair/no-crumb-mutation constraints.

Accordingly the code work is verified, but the two external numeric gates cannot both be satisfied for the supplied checkout. The evidence is deterministic and preserved in the control/final artifacts above.
