# Task board: feature/production-reliability-data-readiness

Status: active
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
| Todo | Decide section IDs and display names | `internal/codeguard/runner/checks/registry.go` | `go test ./tests/checks ./tests/cli` | Prefer stable snake_case final IDs: `reliability`, `data`. Avoid the existing supply-chain hyphen/underscore mismatch. |
| Todo | Decide default enablement | `internal/codeguard/config/defaults.go`, `internal/codeguard/config/profile.go` | `go test ./internal/codeguard/config ./tests/codeguard` | Suggested: startup warns only for severe reliability; strict enables reliability; enterprise enables reliability + data; ai-safe enables reliability + data diff signals. |
| Todo | Define severity policy | rule catalogs + profile docs | metadata tests | Suggested: block only high-confidence outage/data-loss patterns; warn confidence-based heuristics. |
| Todo | Define language priority | check packages | targeted check tests | Start Go first, then TypeScript/JavaScript, then Python. C++/Rust/Java can begin as catalog/config placeholders only when detectors are not ready. |

### Phase 1: Add family scaffolding

| Status | Task | Files/area | Tests | Notes |
| --- | --- | --- | --- | --- |
| Todo | Add `ReliabilityRulesConfig` and `DataRulesConfig` | `internal/codeguard/core/config_rule_types.go` | config tests | Use `*bool` per rule toggle so omitted values can get defaults. Add thresholds for max retry count, max queue/buffer size, unbounded-read row limit, and trusted boundary patterns. |
| Todo | Add top-level toggles | `internal/codeguard/core/config_types.go` | config IO tests | `Reliability *bool` if omitted should support profile defaults; `Data *bool` if data rules should start opt-in outside enterprise. |
| Todo | Add defaults/examples | `internal/codeguard/config/defaults.go`, `defaults_rules.go`, `example.go`, `example_rules.go` | `go test ./internal/codeguard/config ./tests/codeguard` | Mirror supply-chain/performance patterns. |
| Todo | Add validation | `internal/codeguard/config/validate_reliability.go`, `validate_data.go`, `validate.go` | config validation tests | Validate thresholds are positive, pattern entries are non-empty, and rule dependencies are coherent. |
| Todo | Add SDK aliases | `pkg/codeguard/sdk_types_config_checks.go` | `go test ./pkg/codeguard` | Keep public SDK config usable. |
| Todo | Add catalogs | `internal/codeguard/rules/catalog_reliability.go`, `catalog_data.go`, `catalog.go` | `go test ./tests/cli` | Explicit `LanguageCoverage` for all non-language-prefixed IDs. |
| Todo | Add fix templates | `internal/codeguard/rules/catalog_fix_templates_reliability.go`, `catalog_fix_templates_data.go` | metadata tests | Use guided templates for concurrency/data risks; deterministic templates only for mechanical timeout/context cases. |

### Phase 2: Implement reliability detectors

| Status | Task | Files/area | Tests | Notes |
| --- | --- | --- | --- | --- |
| Todo | Create check package | `internal/codeguard/checks/reliability/reliability.go` | `tests/checks/reliability_test.go` | Follow `supplychain.Run` shape and use `env.FinalizeSection("reliability", "Reliability", findings)`. |
| Todo | Register section | `internal/codeguard/runner/checks/registry.go` | section smoke test | Place after performance and before design/security so production-readiness issues appear early. |
| Todo | Detect Go outbound calls without timeout/context | `internal/codeguard/checks/reliability/*go*.go` | `TestReliabilityGoMissingTimeout` | Flag `http.Get`, `http.Post`, `http.DefaultClient.Do`, `exec.Command`, raw network calls, and DB calls lacking context where applicable. Avoid false positives for explicit `http.Client{Timeout: ...}` and context-bound requests. |
| Todo | Detect retry loops without limits/backoff/jitter | Go detector files | `TestReliabilityGoRetryPolicy` | Identify loops around calls with `retry`, `attempt`, transient errors, or status-code checks. Evidence should include limit/backoff/jitter absence separately. |
| Todo | Detect non-idempotent retries | Go detector files | `TestReliabilityGoNonIdempotentRetry` | Flag retried `POST`, writes, DB mutations, event publishes, or side-effect calls unless idempotency key/dedup marker is present. Confidence-based. |
| Todo | Detect missing cancellation propagation | Go detector files | `TestReliabilityGoMissingCancellation` | Flag background goroutines or downstream calls using `context.Background()`/`TODO()` inside request/job flows. |
| Todo | Detect unbounded work/concurrency | Go detector files | `TestReliabilityGoUnboundedWork` | Flag unbounded goroutine spawn in loops, unbounded channel buffers, unbounded worker queues, and `errgroup` without limits where supported. |
| Todo | Detect resource leaks | Go detector files | `TestReliabilityGoResourceLeak` | Track opened files, response bodies, rows, tickers, and timers. Reuse existing parser/support helpers if available. |
| Todo | Detect missing graceful shutdown | Go detector files | `TestReliabilityGoMissingGracefulShutdown` | Flag servers/workers started without signal handling, shutdown context, drain/close path, or wait group. Keep confidence low/medium unless evidence is strong. |
| Todo | Detect swallowed/lost errors and recoverable panic | Go detector files | `TestReliabilityGoErrorHandling` | Coordinate with existing `quality.ai.*` error signals to avoid duplicate rule spam. Prefer reliability IDs for production failure semantics. |

### Phase 3: Implement data-correctness detectors

| Status | Task | Files/area | Tests | Notes |
| --- | --- | --- | --- | --- |
| Todo | Create check package | `internal/codeguard/checks/data/data.go` | `tests/checks/data_test.go` | Use a dedicated `Data Correctness` section. |
| Todo | Register section | `internal/codeguard/runner/checks/registry.go` | section smoke test | Run after reliability; many data findings will be repo/path-level and diff-filtered by line when possible. |
| Todo | Detect read-modify-write race | Go detector files | `TestDataGoReadModifyWriteRace` | Look for select/read followed by update/write outside transaction or conditional update. Evidence: same key/entity, mutation after read, no transaction/lock/compare-and-swap. |
| Todo | Detect missing transaction boundary | Go detector files | `TestDataGoMissingTransactionBoundary` | Flag multiple related DB writes without transaction wrapper. Keep configurable DB API patterns. |
| Todo | Detect external side effects inside retried transaction | Go detector files | `TestDataGoSideEffectInTransaction` | Flag HTTP/email/event calls inside transaction/retry closures. High production risk. |
| Todo | Detect consumer idempotency gaps | Go/TS/Python detectors | `TestDataConsumerIdempotency` | Flag message handlers without dedup/idempotency key checks around side effects. Start with naming/framework heuristics and confidence evidence. |
| Todo | Detect unsafe dual writes and missing outbox | Go detector files | `TestDataGoOutbox` | Flag DB write plus event publish without outbox, transactional event table, or equivalent configured strategy. |
| Todo | Detect unstable pagination | Go/TS/Python detectors | `TestDataUnstablePagination` | Flag limit/offset without deterministic order or cursor stability. |
| Todo | Detect unbounded DB reads | Go/TS/Python detectors | `TestDataUnboundedRead` | Flag `Find/Select/Query` without limit, streaming, pagination, or bounded filters. |
| Todo | Detect unsafe schema migrations | migration file scanner | `TestDataUnsafeMigration` | Coordinate with `contracts.non-expand-contract-migration`; detect destructive/contracting migrations without expand/contract staging metadata. |
| Todo | Detect exactly-once assumptions | text/code scanner | `TestDataExactlyOnceAssumption` | Flag comments/config/code that assert exactly-once without idempotency/dedup. Low/medium confidence unless tied to consumer code. |
| Todo | Detect cache without policy | Go/TS/Python detectors | `TestDataCacheWithoutPolicy` | Require TTL/invalidation/ownership policy for production caches. Allow configured cache wrappers. |

### Phase 4: Add production-risk artifact

| Status | Task | Files/area | Tests | Notes |
| --- | --- | --- | --- | --- |
| Todo | Add artifact schema | `internal/codeguard/core/report_artifact_types.go`, maybe new `report_artifact_pr_summary_types.go` | serialization tests | Add `ReportArtifactKindPRSummary = "pr_summary"` and `PRSummaryArtifact` with `production_risk`. Keep fields additive and `omitempty`. |
| Todo | Add artifact helper | `internal/codeguard/checks/support/artifacts.go` or new support file | artifact tests | Follow `NewSlopScoreArtifact`/`NewChangeRiskArtifact`; defensively copy evidence slices. |
| Todo | Add runner postprocessor | `internal/codeguard/runner/pr_summary.go` | `internal/codeguard/runner/*test.go` | Publish once with `sc.Artifacts.Put(...)`, sorted evidence, deterministic scoring. |
| Todo | Wire production risk inputs | `internal/codeguard/runner/pr_summary.go` | risk tests | Inputs: reliability/data fail/warn findings, non-idempotent retry, missing transaction/outbox, resource leak, unbounded work/read, suppressed findings excluded by current pipeline. |
| Todo | Preserve outputs | `internal/codeguard/report/write.go`, `github_comment.go` if rendered | report tests | JSON should include artifact automatically. Do not emit PR metrics as GitHub annotations. Do not mutate the existing text `Summary:` line. |
| Todo | SDK aliases | `pkg/codeguard/sdk_types_runtime_report.go` | SDK tests | Export new runtime types if public consumers need them. |

### Phase 5: Documentation and examples

| Status | Task | Files/area | Tests | Notes |
| --- | --- | --- | --- | --- |
| Todo | Update product docs when behavior exists | `docs/checks.md`, `docs/features.md`, `docs/production.md`, `README.md` | docs checks/self-scan | Do not advertise catalog-only rules as fully implemented. Mark staged/confidence-based behavior clearly. |
| Todo | Update examples | `examples/codeguard.json`, `.codeguard/codeguard.yaml` if appropriate | `make codeguard-ci` | Consider keeping new families opt-in until false-positive rate is measured. |
| Todo | Add migration notes | `docs/production.md` or release notes | n/a | Explain profile behavior and how to tune/waive noisy reliability/data checks. |

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

- [ ] Rule IDs are stable and documented.
- [ ] Every built-in rule has a fix template.
- [ ] New config fields have defaults, validation, examples, and SDK aliases.
- [ ] New sections use stable section IDs and deterministic output.
- [ ] Findings are diff-filtered correctly where line-level evidence exists.
- [ ] Production-risk scoring has deterministic evidence ordering.
- [ ] SARIF/GitHub annotations remain finding-only.
- [ ] Product docs describe implemented behavior, not planned behavior.
- [ ] `make test` passes.
- [ ] `make ci` passes or any skipped gate is explicitly documented.
