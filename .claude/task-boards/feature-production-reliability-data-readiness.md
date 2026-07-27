# Task board: feature/production-reliability-data-readiness

Status: complete
Branch: feature/production-reliability-data-readiness
Last updated: 2026-07-27
Not final product docs: this is implementation planning for the branch, not shipped user-facing documentation.

## Progress update: first implementation slice

Completed in the first implementation pass:

- Added Reliability and Data Correctness config surfaces, defaults, validation, profile enablement, SDK aliases, rule catalogs, and fix templates.
- Added `Reliability` and `Data Correctness` runner sections.
- Added Go reliability detectors for missing HTTP timeouts, missing cancellation propagation, unbounded goroutine work, retry policy gaps, HTTP response body leaks, swallowed errors, lost error context, recoverable panic, and missing graceful shutdown evidence.
- Added Go data-correctness detectors for read-modify-write/multi-write transaction gaps, side effects inside transaction callbacks, unsafe dual writes, missing outbox evidence, consumer idempotency/dedupe gaps, unstable pagination, unbounded SQL reads, exactly-once assumptions, and cache policy gaps.
- Added additive `pr_summary.production_risk` report artifact and SDK aliases. The artifact is diff-only and does not change SARIF/GitHub annotation/text summary compatibility.
- Added focused tests in `tests/checks/reliability_test.go`, `tests/checks/data_test.go`, `internal/codeguard/runner/pr_summary_test.go`, and representative metadata tests.

Verification completed:

- `go test ./...` with localhost test escalation.
- `make codeguard-ci`.

## Progress update: multi-language production-readiness slice

Completed in the second implementation pass:

- Expanded Reliability and Data Correctness rule language coverage to include C++ in addition to Go, Python, TypeScript, and JavaScript.
- Added Python reliability detectors for outbound HTTP calls without timeouts, retry/backoff gaps, non-idempotent retry evidence, unbounded asyncio work, swallowed exceptions, generic recoverable raises, and nearby resource-leak evidence.
- Added TypeScript/JavaScript reliability detectors for HTTP calls without timeout/abort evidence, promise/HTTP work in loops without concurrency limits, retry/backoff gaps, non-idempotent retry evidence, swallowed catch blocks, and generic recoverable throws.
- Added C++ reliability detectors for retry/backoff gaps, non-idempotent retry evidence, thread/task launches in loops without concurrency bounds, generic runtime throws, and raw allocation without nearby ownership cleanup.
- Added Python data-correctness detectors for unbounded reads, unstable pagination, multi-write transaction gaps, write+publish/outbox gaps, consumer idempotency/dedupe gaps, exactly-once assumptions, and cache writes without TTL evidence.
- Added TypeScript/JavaScript data-correctness detectors for unbounded reads, unstable pagination, multi-write transaction gaps, write+publish/outbox gaps, consumer idempotency/dedupe gaps, exactly-once assumptions, and cache writes without TTL evidence.
- Added C++ data-correctness detectors for unbounded reads, unstable pagination, multi-write transaction gaps, write+publish/outbox gaps, consumer idempotency/dedupe gaps, exactly-once assumptions, and cache writes without TTL evidence.
- Added focused multi-language tests for Python, TypeScript, JavaScript, and C++ reliability/data behavior.

Verification completed:

- `go test ./internal/codeguard/config ./internal/codeguard/rules ./internal/codeguard/runner ./internal/codeguard/runner/checks ./internal/codeguard/checks/reliability ./internal/codeguard/checks/data ./pkg/codeguard ./tests/checks ./tests/cli`.
- `go test ./...` with localhost test escalation.
- `make codeguard-ci`.

## Progress update: completion audit

Completed in the final audit pass:

- Verified the task-board inventory against implemented Reliability, Data Correctness, API Contracts, and `pr_summary.production_risk` code paths.
- Added targeted positive and negative tests for Go, Python, TypeScript, JavaScript, and C++ reliability/data detectors.
- Wired `contracts.non-expand-contract-migration` into actual migration scan output while preserving the legacy `contracts.migration-destructive` finding for existing waivers and baselines.
- Updated shipped docs and examples for `reliability`, `data`, `production_risk`, and non-expand/contract migration behavior.

Verification completed:

- `go test ./tests/checks -run 'Test(Reliability|Data)'`
- `go test ./internal/codeguard/checks/reliability ./internal/codeguard/checks/data ./tests/cli -run 'TestSDKRuleMetadataFor(Reliability|Data)'`
- `go test ./tests/checks -run 'TestContracts(Migration|FullScan)'`
- `go test ./tests/cli -run 'TestSDKRuleMetadataFor(Reliability|Data|NonExpand)'`
- `go test ./...` with localhost test escalation.
- `make codeguard-ci`.
- `make ci` with localhost test escalation.

## Goal

Make CodeGuard detect production-readiness failures that commonly cause outages or data loss:

- reliability failures in outbound calls, retry loops, cancellation, concurrency, cleanup, shutdown, and partial-failure handling;
- distributed-system and data-correctness failures around transactions, idempotency, dual writes, pagination, unbounded reads, migrations, caches, and delivery semantics;
- a first production-risk rollup that turns these findings into PR-level risk evidence.

This branch should move CodeGuard beyond general code quality and into “will this change behave safely in production?”

## Non-goals

- Do not implement broad local design smells, naming checks, or reviewability metrics here. Those belong to `feature/change-safety-testability-refactors`.
- Do not implement observability, ownership, runbook, or rollout-governance checks here except where needed as production-risk inputs. Those belong to `feature/operability-design-delivery-governance`.
- Do not rename existing rule IDs. Waivers and baselines depend on stable IDs.
- Do not make new rules blocking in every profile without a staged rollout path.

## Product split

This branch owns:

- Rule families: `reliability.*`, `data.*`, and `contracts.non-expand-contract-migration`.
- New sections/config: `reliability`, `data`, and production-risk artifact/config.
- Product metric: `production_risk` as part of a new `pr_summary` artifact.

Adjacent branch contracts:

- `feature/change-safety-testability-refactors` will add `change_safety`, `maintainability_delta`, and `refactor_confidence` to the same `pr_summary` artifact.
- `feature/operability-design-delivery-governance` will add observability/delivery signals that can feed `production_risk` after the artifact shape exists.

## Existing repo seams to reuse

- Rule catalog merge point: `internal/codeguard/rules/catalog.go`.
- Rule metadata schema: `internal/codeguard/core/rule_metadata_types.go`.
- Fix-template requirement: `internal/codeguard/rules/catalog_fix_templates*.go`; every built-in rule needs fix guidance.
- Config surface: `internal/codeguard/core/config_types.go`, `internal/codeguard/core/config_rule_types.go`.
- Defaults/examples/validation: `internal/codeguard/config/defaults.go`, `internal/codeguard/config/defaults_rules.go`, `internal/codeguard/config/example.go`, `internal/codeguard/config/validate.go`.
- Profile behavior: `internal/codeguard/config/profile.go`.
- Check family runner pattern: `internal/codeguard/checks/supplychain/supplychain.go`.
- Runner section registration: `internal/codeguard/runner/checks/registry.go`.
- Finding construction/finalization: `internal/codeguard/runner/support/findings.go`, `internal/codeguard/runner/support/findings_section.go`.
- Diff-aware inputs: `internal/codeguard/runner/support/diff_scope.go`, `internal/codeguard/runner/support/changed_files.go`, `internal/codeguard/core/diff_types.go`.
- Existing risk artifacts: `internal/codeguard/runner/risk_scoring.go`, `internal/codeguard/core/report_artifact_types.go`.

## Rule inventory

### Reliability

Initial rule IDs:

- `reliability.missing-timeout`
- `reliability.unbounded-retry`
- `reliability.retry-without-backoff`
- `reliability.non-idempotent-retry`
- `reliability.missing-cancellation`
- `reliability.unbounded-work`
- `reliability.missing-concurrency-limit`
- `reliability.resource-leak`
- `reliability.partial-failure-hidden`
- `reliability.missing-graceful-shutdown`
- `reliability.swallowed-error`
- `reliability.lost-error-context`
- `reliability.recoverable-panic`

### Data correctness

Initial rule IDs:

- `data.read-modify-write-race`
- `data.missing-transaction-boundary`
- `data.side-effect-in-transaction`
- `data.non-idempotent-consumer`
- `data.missing-deduplication`
- `data.unsafe-dual-write`
- `data.missing-outbox-strategy`
- `data.unstable-pagination`
- `data.unbounded-read`
- `data.exactly-once-assumption`
- `data.cache-without-policy`
- `contracts.non-expand-contract-migration`

## Implementation phases

### Phase 0: Design the rollout contract

| Status | Task | Files/area | Tests | Notes |
| --- | --- | --- | --- | --- |
| Done | Decide section IDs and display names | `internal/codeguard/runner/checks/registry.go` | `go test ./tests/checks ./tests/cli` | Stable section IDs: `reliability`, `data`. |
| Done | Decide default enablement | `internal/codeguard/config/defaults.go`, `internal/codeguard/config/profile.go` | `go test ./internal/codeguard/config ./tests/codeguard` | Profile-gated rollout implemented. |
| Done | Define severity policy | rule catalogs + profile docs | metadata tests | High-confidence outage/data-loss patterns fail; confidence-based heuristics warn. |
| Done | Define language priority | check packages | targeted check tests | Implemented for Go, Python, TypeScript, JavaScript, and C++. |

### Phase 1: Add family scaffolding

| Status | Task | Files/area | Tests | Notes |
| --- | --- | --- | --- | --- |
| Done | Add `ReliabilityRulesConfig` and `DataRulesConfig` | `internal/codeguard/core/config_rule_types.go` | config tests | Implemented with `*bool` toggles and thresholds. |
| Done | Add top-level toggles | `internal/codeguard/core/config_types.go` | config IO tests | `Reliability *bool` and `Data *bool` implemented. |
| Done | Add defaults/examples | `internal/codeguard/config/defaults.go`, `defaults_rules.go`, `example.go`, `example_rules.go`, `examples/codeguard.json` | `go test ./internal/codeguard/config ./tests/codeguard` | Defaults and example config updated. |
| Done | Add validation | `internal/codeguard/config/validate_reliability_data.go`, `validate.go` | config validation tests | Threshold validation implemented. |
| Done | Add SDK aliases | `pkg/codeguard/sdk_types_config_checks.go` | `go test ./pkg/codeguard` | Public SDK aliases implemented. |
| Done | Add catalogs | `internal/codeguard/rules/catalog_reliability.go`, `catalog_data.go`, `catalog_contracts.go`, `catalog.go` | `go test ./tests/cli` | Explicit language coverage implemented. |
| Done | Add fix templates | `internal/codeguard/rules/catalog_fix_templates_reliability.go`, `catalog_fix_templates_data.go`, `catalog_fix_templates_misc.go` | metadata tests | Fix templates implemented. |

### Phase 2: Implement reliability detectors

| Status | Task | Files/area | Tests | Notes |
| --- | --- | --- | --- | --- |
| Done | Create check package | `internal/codeguard/checks/reliability/reliability.go` | `tests/checks/reliability_test.go` | Implemented with `Reliability` section. |
| Done | Register section | `internal/codeguard/runner/checks/registry.go` | section smoke test | Registered in runner. |
| Done | Detect Go outbound calls without timeout/context | `internal/codeguard/checks/reliability/*go*.go` | `TestReliabilityGoMissingTimeout` | Includes safe-pattern coverage for `http.Client{Timeout: ...}` and context-bound requests. |
| Done | Detect retry loops without limits/backoff/jitter | Go/Python/TS/JS/C++ detector files | `TestReliabilityGoRetryRisk`, multi-language reliability tests | Implemented with cross-language coverage. |
| Done | Detect non-idempotent retries | Go/Python/TS/JS/C++ detector files | retry-risk tests | Implemented with idempotency/dedupe evidence checks. |
| Done | Detect missing cancellation propagation | Go detector files | `TestReliabilityGoDetectsCancellationAndUnboundedWork` | Implemented for Go context propagation gaps. |
| Done | Detect unbounded work/concurrency | Go/Python/TS/JS/C++ detector files | unbounded-work tests | Implemented with safe-pattern negative coverage. |
| Done | Detect resource leaks | Go/Python/C++ detector files | resource-leak tests | Implemented with safe cleanup negative coverage. |
| Done | Detect missing graceful shutdown | Go detector files | reliability tests/catalog coverage | Implemented for Go server start without shutdown evidence. |
| Done | Detect swallowed/lost errors and recoverable panic | Go/Python/TS/JS/C++ detector files | error-handling tests | Implemented with reliability IDs for production failure semantics. |

### Phase 3: Implement data-correctness detectors

| Status | Task | Files/area | Tests | Notes |
| --- | --- | --- | --- | --- |
| Done | Create check package | `internal/codeguard/checks/data/data.go` | `tests/checks/data_test.go` | Implemented with `Data Correctness` section. |
| Done | Register section | `internal/codeguard/runner/checks/registry.go` | section smoke test | Registered in runner after reliability. |
| Done | Detect read-modify-write race | Go detector files | `TestDataGoDetectsReadModifyWriteTransactionSideEffectCacheAndExactlyOnce` | Implemented for Go. |
| Done | Detect missing transaction boundary | Go/Python/TS/JS/C++ detector files | transaction tests | Implemented with cross-language coverage. |
| Done | Detect external side effects inside retried transaction | Go detector files | side-effect transaction tests | Implemented for Go transaction callbacks. |
| Done | Detect consumer idempotency gaps | Go/Python/TS/JS/C++ detectors | consumer idempotency tests | Implemented with dedupe/idempotency evidence checks. |
| Done | Detect unsafe dual writes and missing outbox | Go/Python/TS/JS/C++ detector files | outbox tests | Implemented with outbox negative coverage. |
| Done | Detect unstable pagination | Go/Python/TS/JS/C++ detectors | pagination tests | Implemented with order/bound negative coverage. |
| Done | Detect unbounded DB reads | Go/Python/TS/JS/C++ detectors | unbounded-read tests | Implemented with bound/filter negative coverage. |
| Done | Detect unsafe schema migrations | `internal/codeguard/checks/contracts/migrations.go` | `TestContractsMigrationDestructiveFlagsNewMigrationsOnly` | Emits `contracts.non-expand-contract-migration` while preserving legacy migration rule. |
| Done | Detect exactly-once assumptions | Go/Python/TS/JS/C++ scanners | exactly-once tests | Implemented with idempotency/dedupe evidence exceptions. |
| Done | Detect cache without policy | Go/Python/TS/JS/C++ detectors | cache-policy tests | Implemented with TTL/policy negative coverage. |

### Phase 4: Add production-risk artifact

| Status | Task | Files/area | Tests | Notes |
| --- | --- | --- | --- | --- |
| Done | Add artifact schema | `internal/codeguard/core/report_artifact_types.go` | serialization tests | `ReportArtifactKindPRSummary` and `PRSummaryArtifact` implemented. |
| Done | Add artifact helper | `internal/codeguard/checks/support/artifacts.go` | artifact tests | PR summary artifact helper implemented. |
| Done | Add runner postprocessor | `internal/codeguard/runner/pr_summary.go` | `internal/codeguard/runner/pr_summary_test.go` | Deterministic scoring and artifact publication implemented. |
| Done | Wire production risk inputs | `internal/codeguard/runner/pr_summary.go` | risk tests | Reliability/data/non-expand migration inputs wired. |
| Done | Preserve outputs | report/SARIF/GitHub paths | report tests | Artifact remains additive; annotations remain finding-only. |
| Done | SDK aliases | `pkg/codeguard/sdk_types_runtime_report.go` | SDK tests | Runtime report aliases implemented. |

### Phase 5: Documentation and examples

| Status | Task | Files/area | Tests | Notes |
| --- | --- | --- | --- | --- |
| Done | Update product docs when behavior exists | `docs/checks.md`, `docs/features.md`, `docs/production.md` | docs checks/self-scan | Docs now describe implemented behavior and confidence-based heuristics. |
| Done | Update examples | `examples/codeguard.json` | `make codeguard-ci` | Example config includes opt-in reliability/data/production-risk knobs. |
| Done | Add migration notes | `docs/checks.md`, `docs/production.md` | n/a | Non-expand/contract migration and rollout notes added. |

## Detector confidence policy

- High confidence: syntactic evidence directly proves missing timeout, ignored cleanup error, unbounded goroutine in loop, multiple DB writes without transaction, or DB write plus event publish without outbox.
- Medium confidence: naming/API heuristics imply retry, idempotency, consumer, cache, transaction, or side effect but repository-specific wrapper may exist.
- Low confidence: comment/text/history-derived signals such as exactly-once assumptions or cascading synchronous dependency risk.

Findings should include evidence metadata that is safe for reports: operation kind, call kind, retry loop evidence, transaction wrapper evidence, idempotency evidence, and configured framework match. Do not include source snippets or secrets in metadata.

## Profile behavior target

| Profile | Reliability | Data correctness | Production risk |
| --- | --- | --- | --- |
| Startup | Warn severe/high-confidence reliability only | Off by default | Warn only |
| Strict | Block new high-confidence reliability regressions; warn medium confidence | Warn high-confidence data risks in diff mode | Warn elevated risk |
| Enterprise | Block severe reliability and data-loss risks | Block unsafe dual writes, missing transaction boundaries, unsafe migrations | Warn/block by threshold |
| AI-safe | Strict plus weak error handling, oversized risk, and missing tests from sibling branch | Strict data diff signals | Warn/block by threshold |

## Acceptance criteria

- New family config validates and round-trips in JSON/YAML.
- `codeguard rules` exposes reliability/data metadata with language coverage and fix templates.
- Enabling the new sections runs without panics on empty repos and on this repo.
- Go reliability detectors cover at least missing timeout, unbounded retry, missing cancellation, unbounded work, resource leak, swallowed/lost errors, and recoverable panic.
- Go data detectors cover at least read-modify-write race, missing transaction, side-effect-in-transaction, unsafe dual write/missing outbox, unstable pagination, unbounded read, and cache-without-policy.
- The `pr_summary` artifact includes deterministic `production_risk` score/evidence in diff scans.
- Existing JSON/SARIF/GitHub annotation/text summary compatibility is preserved.
- Targeted tests and `make test` pass before push/PR.

## Verification plan

Targeted during implementation:

```sh
go test ./internal/codeguard/config ./internal/codeguard/rules ./internal/codeguard/runner
go test ./tests/codeguard ./tests/checks ./tests/cli -run 'Test.*(Reliability|Data|ProductionRisk|Rules|Profiles|Metadata)'
go test ./tests/security ./tests/checks -run 'TestWriteReport|TestSARIF|TestGitHub'
```

Branch gate:

```sh
make fmt-check
make test
make codeguard-ci
```

Pre-push/PR gate when practical:

```sh
make ci
```

## Merge checklist

- [x] Rule IDs are stable and documented.
- [x] Every built-in rule has a fix template.
- [x] New config fields have defaults, validation, examples, and SDK aliases.
- [x] New sections use stable section IDs and deterministic output.
- [x] Findings are diff-filtered correctly where line-level evidence exists.
- [x] Production-risk scoring has deterministic evidence ordering.
- [x] SARIF/GitHub annotations remain finding-only.
- [x] Product docs describe implemented behavior, not planned behavior.
- [x] `make test` passes.
- [x] `make ci` passes or any skipped gate is explicitly documented.
