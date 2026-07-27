# Task board: feature/deep-code-smells-maintainability-precision

Status: staging
Branch: feature/deep-code-smells-maintainability-precision
Last updated: 2026-07-27
Not final product docs: this is implementation planning for the branch, not shipped user-facing documentation.

## Goal

Make CodeGuard materially stronger at local maintainability review: structural code smells, domain vocabulary consistency, richer function responsibility signals, error-contract quality, defensive boundary checks, and remaining reliability parity hardening across Python, TypeScript, JavaScript, and C++.

The product target is to answer:

> Did this PR make the code easier to understand, safer to change, and less likely to fail at boundaries?

## Branch baseline

This branch is cut from the rebased production-readiness stack after:

- `feature/production-reliability-data-readiness`
- `feature/change-safety-testability-refactors`
- `feature/operability-design-delivery-governance`
- dogfood coverage commits through `7199646`

Do not loosen `.codeguard` policy to make this branch pass. Dogfood warnings should be resolved by concrete refactors, narrower detectors, better fixtures, or explicit scoped waivers only when there is a real documented exception.

## Non-goals

- Do not duplicate already-shipped architecture-boundary, observability, data, delivery, or change-safety rules unless the new behavior is materially deeper.
- Do not add broad LLM-dependent checks. Keep this branch deterministic/static/history-backed.
- Do not claim full semantic equivalence or prove correctness. Use evidence, confidence, and clear remediation text.
- Do not broaden rule metadata to languages where behavior/tests do not exist.

## Workstreams

### Workstream A: Reliability parity hardening

Owner scope:

- `internal/codeguard/checks/reliability/*`
- `internal/codeguard/rules/catalog_reliability.go`
- `internal/codeguard/rules/catalog_fix_templates_reliability.go`
- `tests/checks/reliability_multilang_test.go`
- docs snippets only if rule behavior or coverage changes

Tasks:

| Status | Rule / capability | Expected behavior | Languages | Tests |
| --- | --- | --- | --- | --- |
| Done | `reliability.missing-cancellation` parity | Detect detached/background cancellation or missing abort/signal propagation in production calls. | Python, TS, JS, C++ | Added focused positive cases; existing bounded fixtures cover negatives. |
| Done | `reliability.missing-graceful-shutdown` parity | Detect server/listener/thread/service start paths without signal/shutdown/stop evidence. | Python, TS, JS, C++ | Added focused positive cases; safe non-server fixtures remain clean. |
| Done | `reliability.missing-concurrency-limit` parity | Detect unbounded task/thread/promise creation beyond existing loop heuristics. | Python, TS, JS, C++ | Added threshold-aware positive cases. |
| Done | `reliability.resource-leak` parity | Add TS/JS cleanup detection for response/body/stream/file handles. | TS, JS | Added stream/file positive cases plus existing safe resource cases. |
| Done | `reliability.missing-timeout` C++ | Detect common C++ HTTP/RPC calls without timeout/deadline evidence. | C++ | Added C++ positive case. |
| Done | `reliability.swallowed-error` C++ | Detect catch blocks that return/continue without surfacing failures. | C++ | Added C++ positive case. |
| Done | `reliability.lost-error-context` parity | Detect generic rethrows/returns that discard cause or operation context. | Python, TS, JS, C++ | Added Python/TS/JS/C++ positive cases. |
| Done | Metadata reconciliation | Narrow or expand language coverage only to tested behavior. | all | `go test ./tests/cli -run Metadata` passed locally. |

Verification:

```sh
env -u GOROOT GOCACHE=/private/tmp/codeguard-deep-smells-go-cache go test ./tests/checks -run 'TestReliability' -count=1
env -u GOROOT GOCACHE=/private/tmp/codeguard-deep-smells-go-cache go test ./tests/cli -run 'TestSDKRuleMetadata.*Reliability|TestSDKRuleMetadata' -count=1
make codeguard-ci
```

### Workstream B: Structural smell rules

Owner scope:

- `internal/codeguard/checks/quality/quality_smells*.go` or equivalent new quality files
- `internal/codeguard/rules/catalog_quality.go`
- `internal/codeguard/rules/catalog_fix_templates_quality.go`
- `tests/checks/quality_smells_test.go`
- shared support helpers only when needed

Candidate rules:

| Status | Rule ID | Signal | Notes |
| --- | --- | --- | --- |
| Todo | `smell.god-object` | One type/class owns too many methods, fields, responsibilities, or dependency clusters. | Avoid duplicating existing god-module; local type-level only. |
| Todo | `smell.feature-envy` | Function/method accesses more external object fields/methods than own receiver/context. | Confidence-based. |
| Todo | `smell.middle-man` | Type/class mostly delegates to one collaborator without policy/translation. | Coordinate with pass-through abstraction. |
| Todo | `smell.message-chain` | Long call chains across objects/modules. | Warn, medium confidence. |
| Todo | `smell.data-clump` | Same group of primitive parameters appears repeatedly. | Good bridge to primitive obsession. |
| Todo | `smell.switch-on-type` | Repeated type/kind branching that should move behind polymorphism/dispatch. | Go/Python/TS/JS/C++. |
| Todo | `smell.refused-bequest` | Subclass/derived type overrides many inherited methods with no-op/throw/unsupported behavior. | Only if reliable evidence exists. |

Required behavior:

- Catalog metadata and fix templates for each implemented rule.
- Multi-language tests for every claimed language.
- Confidence metadata with evidence counts where useful.
- No broad false positives on simple fixtures.

Verification:

```sh
env -u GOROOT GOCACHE=/private/tmp/codeguard-deep-smells-go-cache go test ./tests/checks -run 'TestQualitySmell|TestSmell' -count=1
make codeguard-ci
```

### Workstream C: Naming glossary and vocabulary precision

Owner scope:

- `internal/codeguard/checks/quality/quality_naming*.go`
- `internal/codeguard/core/config_rule_types.go`
- `internal/codeguard/config/defaults_rules.go`
- `internal/codeguard/config/validate_rules.go`
- `internal/codeguard/rules/catalog_quality.go`
- `tests/checks/quality_naming_test.go`
- config/docs only after behavior exists

Candidate rules:

| Status | Rule ID | Signal |
| --- | --- | --- |
| Todo | `naming.behavior-mismatch` | Name says query/format/build but body mutates/sends/writes, or name says save/delete but body only reads. |
| Todo | `naming.boolean-not-predicate` | Boolean variable/field/return names without `is/has/can/should/allow/enabled` style predicate. |
| Todo | `naming.domain-vocabulary-drift` | Configured glossary detects multiple terms for one domain concept. |
| Todo | `naming.unknown-abbreviation` | Identifier contains abbreviation not established in repo/config. |
| Todo | `naming.cardinality-mismatch` | Plural name used for scalar or singular name used for collection-like value. |
| Todo | `naming.implementation-leak` | Domain/API names encode infrastructure details like SQL, HTTP, Redis, Kafka, ORM. |
| Todo | `naming.missing-unit` | Numeric names for durations/sizes/money lack unit suffix. |
| Todo | `naming.role-suffix-overuse` | Excessive `Manager`, `Helper`, `Util`, `Service`, `Processor` suffixes. |
| Todo | `naming.cross-layer-inconsistency` | Same concept renamed across API/domain/persistence layers. |

Config shape proposal:

```yaml
checks:
  quality_rules:
    naming:
      glossary:
        restaurant:
          avoid: [venue, merchant, establishment]
      allowed_abbreviations: [id, url, api, http]
      role_suffix_warn_threshold: 4
```

Verification:

```sh
env -u GOROOT GOCACHE=/private/tmp/codeguard-deep-smells-go-cache go test ./internal/codeguard/config ./tests/checks -run 'TestQualityNaming|TestNaming' -count=1
make codeguard-ci
```

### Workstream D: Function responsibility and maintainability delta

Owner scope:

- `internal/codeguard/checks/quality/quality_functions*.go`
- `internal/codeguard/checks/quality/quality_precision*.go`
- `internal/codeguard/rules/catalog_quality.go`
- `tests/checks/quality_functions_test.go`
- PR-summary/maintainability artifacts only if existing seams make it safe

Candidate rules:

| Status | Rule ID | Signal |
| --- | --- | --- |
| Todo | `function.excessive-length` | Existing max-function-lines alias/metadata consistency if needed. |
| Todo | `function.excessive-branching` | Decision count above threshold. |
| Todo | `function.excessive-nesting` | Nesting depth above threshold. |
| Todo | `function.excessive-returns` | Too many return paths. |
| Todo | `function.hidden-mutation` | Function mutates input/global/collaborator without name making it explicit. |
| Todo | `function.inconsistent-return-contract` | Mixed nil/value/error/partial shapes or inconsistent success semantics. |
| Todo | `function.multiple-responsibilities` | Responsibility count from validation, load, write, send, emit, auth, transform, cache, etc. |
| Todo | `function.orchestration-domain-mix` | Handler/job orchestration mixed with domain decisions. |
| Todo | `function.control-flow-needs-explanation` | Complex control flow with no extracted helper or named decision. |
| Todo | `function.name-behavior-mismatch` | Function name conflicts with dominant behavior. |
| Todo | `function.partial-result` | Returns partially valid result without explicit partial/error contract. |

Maintainability delta extensions:

| Status | Rule ID | Signal |
| --- | --- | --- |
| Todo | `maintainability.regression` | Combined delta worsens complexity/deps/public/duplication/testability. |
| Todo | `maintainability.no-improvement-in-hotspot` | Hotspot touched but no local simplification/testability improvement. |
| Todo | `maintainability.duplication-growth` | Duplication increased in touched code. |
| Todo | `maintainability.nesting-growth` | Nesting increased in touched code. |
| Todo | `maintainability.testability-regression` | More hardwired/nondeterministic dependencies or fewer tests. |

Verification:

```sh
env -u GOROOT GOCACHE=/private/tmp/codeguard-deep-smells-go-cache go test ./tests/checks -run 'TestQualityFunction|TestMaintainability' -count=1
make codeguard-ci
```

### Workstream E: Error contracts and defensive boundaries

Owner scope:

- `internal/codeguard/checks/quality/quality_errors*.go`
- `internal/codeguard/checks/quality/quality_defensive*.go`
- `internal/codeguard/rules/catalog_quality.go`
- `internal/codeguard/rules/catalog_fix_templates_quality.go`
- `tests/checks/quality_errors_test.go`
- `tests/checks/quality_defensive_test.go`

Candidate error rules:

| Status | Rule ID | Signal |
| --- | --- | --- |
| Todo | `error.logged-and-returned` | Error is logged and returned, risking duplicate logs. |
| Todo | `error.generic-message` | Generic error string without operation/resource context. |
| Todo | `error.wrong-abstraction-level` | Infrastructure errors leak into domain/API/user boundary. |
| Todo | `error.inconsistent-wrapping` | Same function mixes wrapping styles or drops cause. |
| Todo | `error.retryable-not-distinguished` | Retry path cannot distinguish permanent/transient failure. |
| Todo | `error.user-message-leaks-internals` | User/API message exposes DB/SQL/stack/infrastructure details. |
| Todo | `error.cleanup-error-ignored` | Close/rollback/delete cleanup errors discarded. |
| Todo | `error.fallback-hides-corruption` | Fallback success after corruption/deserialization/validation failure. |
| Todo | `error.exception-used-for-control-flow` | Exception/panic/throw used for ordinary branch control. |

Candidate defensive rules:

| Status | Rule ID | Signal |
| --- | --- | --- |
| Todo | `defensive.unvalidated-boundary-input` | Handler/API/event/filesystem input consumed without validation. |
| Todo | `defensive.invalid-state-representable` | Boolean/string status combos allow impossible states. |
| Todo | `defensive.null-assumption` | Dereference/use without guard at nullable boundary. |
| Todo | `defensive.integer-overflow` | Arithmetic on bounded numeric/input sizes without guard. |
| Todo | `defensive.bounds-assumption` | Index/key access without length/existence guard. |
| Todo | `defensive.unsafe-default` | Missing config/env defaults fail open or disable safety. |
| Todo | `defensive.non-exhaustive-branch` | Switch/match over enum-like values lacks default/exhaustive evidence. |
| Todo | `defensive.unchecked-external-response` | External response consumed without status/schema/error check. |
| Todo | `defensive.missing-schema-validation` | JSON/event/request decoded but not validated at boundary. |
| Todo | `defensive.missing-resource-limit` | Boundary reads/uploads/queues without size/count/time bound. |
| Todo | `defensive.invalid-state-transition` | State transition accepts impossible backwards/skipped transitions. |
| Todo | `defensive.fail-open-authorization` | Authz failure path allows or defaults to success. |

Verification:

```sh
env -u GOROOT GOCACHE=/private/tmp/codeguard-deep-smells-go-cache go test ./tests/checks -run 'TestQualityError|TestQualityDefensive|TestDefensive|TestError' -count=1
make codeguard-ci
```

### Workstream F: Docs, metadata, profiles, and final dogfood

Owner scope:

- `docs/checks.md`
- `docs/features.md`
- `docs/production.md`
- `internal/codeguard/rules/catalog*.go`
- `internal/codeguard/rules/catalog_fix_templates*.go`
- `internal/codeguard/config/profile.go`
- `internal/codeguard/config/profile_test.go`
- `tests/cli/features_metadata_test.go`

Tasks:

| Status | Task | Notes |
| --- | --- | --- |
| Todo | Reconcile rule catalog | Every implemented rule has metadata, language coverage, default level, fix template. |
| Todo | Reconcile docs glossary | Docs updated only after behavior/tests land. |
| Todo | Reconcile profile behavior | AI-safe should enable stronger smell/naming/function/error/defensive checks; strict should focus regressions. |
| Todo | Add metadata tests | Include representative new smell, naming, function, error, defensive, reliability parity rules. |
| Todo | Run dogfood | `make codeguard-ci`; fix real warnings by refactor or detector precision. |
| Todo | Run full CI | `make ci` outside restricted sandbox for httptest. |

## Agent assignments

Initial parallel split:

1. Reliability parity worker: Workstream A.
2. Smells/design worker: Workstream B plus structural smell metadata/tests.
3. Naming/function/error/defensive worker: Workstreams C, D, E, starting with catalog/config skeleton and first detector set.
4. Integration/docs worker: Workstream F, plus task-board reconciliation after worker commits.

Workers must:

- Work on disjoint file scopes when possible.
- Not revert other workers' edits.
- Update this board when they complete a task.
- Run focused tests for their slice.
- Run `make codeguard-ci` before pushing or handing off if their changes can affect self-scan.

## Final branch gates

Before PR handoff:

```sh
env -u GOROOT GOCACHE=/private/tmp/codeguard-deep-smells-go-cache go test ./tests/checks ./tests/cli ./internal/codeguard/config ./internal/codeguard/rules -count=1
make codeguard-ci
make ci
git diff --check
```

Expected final result:

- No advertised non-emitting rule IDs.
- No language coverage claims without at least representative tests.
- No new CodeGuard dogfood failures resolved by broad threshold tuning.
- Docs and `codeguard rules` output agree.
