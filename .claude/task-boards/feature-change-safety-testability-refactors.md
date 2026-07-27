# Task board: feature/change-safety-testability-refactors

Status: active
Branch: feature/change-safety-testability-refactors
Last updated: 2026-07-27
Not final product docs: this is implementation planning for the branch, not shipped user-facing documentation.

## Branch setup update

Completed before implementation work:

- Rebased `feature/change-safety-testability-refactors` onto latest `main` after `feature/production-reliability-data-readiness` was merged.
- Force-updated the remote branch with `--force-with-lease` after the rebase.
- Verified the working tree is clean before assigning implementation workstreams.

Agent spawning note:

- New worker spawns were attempted after the rebase, but the workspace was at the agent thread limit.
- Existing completed-agent summaries were collected and used to refine the workstream split below.
- The branch can proceed with these workstreams as soon as agent capacity is available, or the main thread can take the first workstream locally.
- Completed old agent threads were closed after Workstream A landed, freeing slots for the next implementation round.
- Active worker assignments:
  - Hegel (`019fa46e-8955-78a2-aa95-4fcd667d1b59`): Workstream B.
  - Rawls (`019fa46e-b46b-7ca2-a106-83bd1b194f4f`): Workstream C.
  - Raman (`019fa46e-de48-7b73-b3a5-4a48152d9954`): Workstream D.
  - Meitner (`019fa46f-0ef2-7253-bd4a-86ebaae7c6ec`): Workstream E.
  - Gauss (`019fa471-3497-7403-99f9-3d173e9b4da3`): finish-out docs and check glossary.
  - James (`019fa471-61de-74d2-9be5-daab986602da`): integration/QA checklist and consistency review.

## Agent workstreams

These workstreams are intentionally disjoint. Workers must not revert unrelated edits and should list changed files in their handoff.

### Workstream A: scaffolding, config, catalogs, and profiles

Status: complete in main thread; implementation committed/pushed separately from detector work.

Ownership:

- `internal/codeguard/core/config_types.go`
- `internal/codeguard/core/config_rule_types.go`
- `internal/codeguard/config/defaults*.go`
- `internal/codeguard/config/example*.go`
- `internal/codeguard/config/profile.go`
- `internal/codeguard/config/validate*.go`
- `internal/codeguard/rules/catalog_change_safety.go`
- `internal/codeguard/rules/catalog_fix_templates_change_safety.go`
- `pkg/codeguard/sdk_types_config_checks.go`
- config/profile/metadata tests

Tasks:

- Add a minimal top-level `change` section toggle and `ChangeRulesConfig`.
- Add thresholds for changed files, changed directories, changed lines, changed public interfaces, concern-family count, and production/test ratio.
- Add defaults, examples, validation, and SDK aliases.
- Add initial metadata/fix templates for the Phase 1/2/4 rules that will have detector support in this branch.
- Wire profile behavior:
  - `startup`: keep change-safety off unless explicitly enabled.
  - `strict`: enable high-confidence change/testability gates.
  - `enterprise`: inherit strict.
  - `ai-safe`: enable stronger oversized-diff, missing-test, weak-refactor-confidence, duplicated-helper, and unnecessary-abstraction signals.

Targeted verification:

```sh
go test ./internal/codeguard/config ./tests/cli ./pkg/codeguard
go test ./internal/codeguard/... ./pkg/codeguard ./tests/cli
```

### Workstream B: change section and diff concentration detectors

Ownership:

- `internal/codeguard/checks/change/**`
- `internal/codeguard/runner/checks/registry.go`
- `tests/checks/change*_test.go`
- helper additions under `internal/codeguard/checks/support/**` only if needed

Tasks:

- Add the `Change Safety` section runner.
- Run primarily in diff mode; full scans should no-op or emit only explicitly safe repo-level diagnostics.
- Compute change concentration evidence:
  - files touched
  - directories touched
  - architectural layer/path categories touched
  - production/test file ratio
  - public-surface file hints
- Detect:
  - `change.oversized-diff`
  - `change.mixed-concerns`
  - `change.too-many-concerns`
  - `change.mixed-refactor-and-behavior`
  - `change.unnecessary-surface-area`
  - `change.move-without-verification`
- Keep findings deterministic and confidence-based.

Targeted verification:

```sh
go test ./tests/checks -run 'TestChange'
go test ./internal/codeguard/runner/checks
```

### Workstream C: testability detectors

Ownership:

- `internal/codeguard/checks/change/testability*.go` or a clearly named sibling under the change package
- `tests/checks/testing*_test.go`
- no catalog/config edits except small integration adjustments coordinated with Workstream A

Tasks:

- Detect:
  - `testing.behavior-change-without-test`
  - `testing.failure-path-missing`
  - `testing.hardwired-dependency`
  - `testing.nondeterministic-domain-logic`
  - `testing.legacy-hotspot-uncovered` as warn-only if history inputs are available; otherwise leave a documented TODO and do not emit a misleading finding.
- Start with Go, Python, TypeScript, JavaScript, and C++ path/text heuristics where safe.
- Add positive and negative tests for changed production files with/without changed tests.
- Avoid duplicating CI test-quality findings unless the evidence is about change safety, not test style.

Targeted verification:

```sh
go test ./tests/checks -run 'TestTesting'
```

### Workstream D: PR-summary metrics

Ownership:

- `internal/codeguard/core/report_artifact_types.go`
- `internal/codeguard/checks/support/artifacts.go`
- `internal/codeguard/runner/pr_summary.go`
- `internal/codeguard/runner/pr_summary_test.go`
- `pkg/codeguard/sdk_types_runtime_report.go`
- report serialization tests only if artifact shape requires them

Tasks:

- Extend existing `pr_summary` additively with:
  - `change_safety`
  - `maintainability_delta`
  - `refactor_confidence`
- Preserve existing `production_risk` behavior from the merged production-readiness branch.
- Keep metrics artifact-only; do not emit GitHub annotations for metrics.
- Keep the existing text `Summary:` sentence unchanged.
- Sort evidence deterministically.

Targeted verification:

```sh
go test ./internal/codeguard/runner ./tests/codeguard ./tests/checks -run 'Test.*PRSummary|TestWriteReport'
```

### Workstream E: local quality precision and maintainability delta

Ownership:

- `internal/codeguard/checks/quality/**`
- `internal/codeguard/checks/design/**` only for graph/delta helpers
- `internal/codeguard/history/**` only for read-only history metrics
- `tests/checks/naming*_test.go`
- `tests/checks/function*_test.go`
- `tests/checks/error*_test.go`
- `tests/checks/defensive*_test.go`
- `tests/checks/maintainability*_test.go`

Tasks:

- Start with a small, high-value subset instead of every planned smell:
  - `naming.generic-identifier`
  - `function.excessive-parameters`
  - `function.mixed-abstraction-level`
  - `function.command-query-mix`
  - `error.logged-and-ignored`
  - `error.context-lost`
  - `defensive.unchecked-type-assertion`
  - `defensive.unsafe-numeric-conversion`
  - `maintainability.public-surface-growth`
  - `maintainability.dependency-growth`
- Reuse existing quality/design metrics where possible.
- Prefer warnings unless evidence is direct and high-confidence.

Targeted verification:

```sh
go test ./tests/checks -run 'Test(Naming|Function|Error|Defensive|Maintainability)'
```

## Goal

Make CodeGuard evaluate whether a PR is safe, incremental, understandable, testable, and actually improves the code it touches.

This branch owns change-quality, testability, safe-refactor, code-smell, naming/function/error/defensive-programming, and maintainability-delta work. The product target is to answer:

> Did this PR make the system safer, simpler, easier to change, and less likely to fail?

## Non-goals

- Do not implement reliability/data-outage rules owned by `feature/production-reliability-data-readiness`.
- Do not implement observability, ownership, runbook, or deployment-governance rules owned by `feature/operability-design-delivery-governance`.
- Do not overfit one language/framework. Start with the languages where CodeGuard already has parser coverage and tests.
- Do not claim semantic equivalence for refactors. The goal is confidence and evidence, not proof.

## Product split

This branch owns:

- Rule families: `testing.*`, `change.*`, `refactor.*`, `smell.*`, `maintainability.*`, `naming.*`, `function.*`, `error.*`, and `defensive.*`.
- Product metrics in the shared `pr_summary` artifact:
  - `change_safety`
  - `maintainability_delta`
  - `refactor_confidence`
- Diff/history-aware analysis inputs:
  - change concentration score;
  - behavior-preservation evidence;
  - hotspot/change-history signals;
  - ratio of production changes to test changes.

Adjacent branch contracts:

- `feature/production-reliability-data-readiness` owns `production_risk` and may consume `error.*` or `defensive.*` signals later if they indicate outage risk.
- `feature/operability-design-delivery-governance` owns design abstraction and delivery governance signals but can feed maintainability/risk deltas later.

## Existing repo seams to reuse

- Quality and complexity rules: `internal/codeguard/checks/quality/*`, `internal/codeguard/rules/catalog_quality.go`, `catalog_quality_ai.go`.
- CI/test-quality rules: `internal/codeguard/checks/ci/*`, `internal/codeguard/rules/catalog_test_quality.go`.
- Design change-impact helpers: `internal/codeguard/checks/design/design_change_impact.go`.
- Diff support: `internal/codeguard/runner/support/diff_scope.go`, `internal/codeguard/runner/support/changed_files.go`, `internal/codeguard/core/diff_types.go`.
- Risk scoring/postprocessors: `internal/codeguard/runner/risk_scoring.go`; add new PR-summary logic near it.
- History support: `internal/codeguard/history/*`, `internal/codeguard/runner/runner_history.go`, `internal/codeguard/runner/support/legibility_history.go`.
- Rule metadata and fix templates: `internal/codeguard/rules/catalog*.go`, `internal/codeguard/rules/catalog_fix_templates*.go`.
- Report/artifact schema: `internal/codeguard/core/report_artifact_types.go`, `pkg/codeguard/sdk_types_runtime_report.go`.

## Rule inventory

### Testability and change safety

- `testing.behavior-change-without-test`
- `testing.failure-path-missing`
- `testing.hardwired-dependency`
- `testing.nondeterministic-domain-logic`
- `testing.legacy-hotspot-uncovered`
- `change.mixed-concerns`
- `change.oversized-diff`
- `change.mixed-refactor-and-behavior`
- `change.too-many-concerns`
- `change.unnecessary-surface-area`
- `change.one-use-abstraction`
- `change.duplicate-helper`
- `change.cleanup-regression`
- `change.complexity-increased`
- `change.move-without-verification`

### Safe refactors

- `refactor.behavior-change-detected`
- `refactor.public-contract-changed`
- `refactor.test-coverage-reduced`
- `refactor.error-path-changed`
- `refactor.side-effect-order-changed`
- `refactor.visibility-expanded`
- `refactor.dependency-direction-worsened`
- `refactor.duplicate-implementation-left-behind`
- `refactor.dead-path-left-behind`

### Code smells and maintainability

- `smell.god-object`
- `smell.feature-envy`
- `smell.shotgun-surgery`
- `smell.divergent-change`
- `smell.middle-man`
- `smell.message-chain`
- `smell.inappropriate-intimacy`
- `smell.parallel-inheritance`
- `smell.data-clump`
- `smell.primitive-obsession`
- `smell.switch-on-type`
- `smell.refused-bequest`
- `smell.shotgun-surgery-history`
- `smell.divergent-change-history`
- `maintainability.high-churn-hotspot`
- `maintainability.repeat-defect-area`
- `maintainability.unstable-interface`
- `maintainability.ownership-gap`
- `maintainability.regression`
- `maintainability.no-improvement-in-hotspot`
- `maintainability.public-surface-growth`
- `maintainability.dependency-growth`
- `maintainability.duplication-growth`
- `maintainability.nesting-growth`
- `maintainability.testability-regression`
- `maintainability.hotspot`
- `maintainability.change-amplification`
- `maintainability.unstable-dependency`
- `maintainability.low-test-isolation`
- `maintainability.excessive-public-surface`
- `maintainability.architecture-drift`
- `maintainability.missing-owner`
- `maintainability.missing-design-context`
- `maintainability.repeat-regression`
- `maintainability.operational-opacity`

### Naming, functions, errors, and defensive programming

- `naming.generic-identifier`
- `naming.behavior-mismatch`
- `naming.boolean-not-predicate`
- `naming.domain-vocabulary-drift`
- `naming.unknown-abbreviation`
- `naming.cardinality-mismatch`
- `naming.implementation-leak`
- `naming.missing-unit`
- `naming.role-suffix-overuse`
- `naming.cross-layer-inconsistency`
- `function.excessive-length`
- `function.excessive-branching`
- `function.excessive-nesting`
- `function.excessive-parameters`
- `function.excessive-returns`
- `function.hidden-mutation`
- `function.mixed-abstraction-level`
- `function.command-query-mix`
- `function.inconsistent-return-contract`
- `function.multiple-responsibilities`
- `function.orchestration-domain-mix`
- `function.control-flow-needs-explanation`
- `function.name-behavior-mismatch`
- `function.partial-result`
- `error.swallowed`
- `error.logged-and-returned`
- `error.logged-and-ignored`
- `error.context-lost`
- `error.generic-message`
- `error.wrong-abstraction-level`
- `error.inconsistent-wrapping`
- `error.sentinel-comparison-fragile`
- `error.retryable-not-distinguished`
- `error.user-message-leaks-internals`
- `error.partial-failure-hidden`
- `error.cleanup-error-ignored`
- `error.fallback-hides-corruption`
- `error.panic-on-recoverable-path`
- `error.exception-used-for-control-flow`
- `defensive.unvalidated-boundary-input`
- `defensive.invalid-state-representable`
- `defensive.null-assumption`
- `defensive.unchecked-type-assertion`
- `defensive.unsafe-numeric-conversion`
- `defensive.integer-overflow`
- `defensive.bounds-assumption`
- `defensive.unsafe-default`
- `defensive.non-exhaustive-branch`
- `defensive.unchecked-external-response`
- `defensive.missing-schema-validation`
- `defensive.missing-resource-limit`
- `defensive.invalid-state-transition`
- `defensive.fail-open-authorization`

## Implementation phases

### Phase 0: Choose rollout shape

| Status | Task | Files/area | Tests | Notes |
| --- | --- | --- | --- | --- |
| Todo | Decide whether to extend existing sections or add new sections | `core/config_types.go`, `runner/checks/registry.go` | config + section tests | Suggested: add a `change` section for diff/refactor/testability and extend `quality`/`ci` catalogs for local naming/function/error checks only where behavior already exists. |
| Todo | Define confidence policy | rule metadata + docs | report confidence tests | Many smells are heuristic. Emit confidence and evidence; block only high-confidence diff regressions. |
| Todo | Define profile behavior | `internal/codeguard/config/profile.go` | profile tests | Strict blocks new complexity/error/testing regressions; AI-safe enables oversized diff, weak tests, fabricated API/local idiom, duplicated code, unnecessary abstraction. |
| Todo | Define shared PR-summary artifact contract | `core/report_artifact_types.go` | report serialization tests | Coordinate with production-risk branch. Artifact-first, additive JSON, no GitHub annotations for metrics. |

### Phase 1: Add change-analysis infrastructure

| Status | Task | Files/area | Tests | Notes |
| --- | --- | --- | --- | --- |
| Todo | Add `ChangeRulesConfig` | `core/config_rule_types.go`, `core/config_types.go` | config tests | Thresholds: max files, max dirs, max public interfaces, max changed lines, max concern families, min test/prod ratio. |
| Todo | Add defaults/examples/validation | `config/defaults*.go`, `config/example*.go`, `config/validate_change.go` | `go test ./internal/codeguard/config ./tests/codeguard` | Validate positive thresholds and non-empty concern classifiers. |
| Todo | Add change section package | `internal/codeguard/checks/change/change.go` | `tests/checks/change_test.go` | Run only in diff mode unless explicitly enabled for full scans. |
| Todo | Register section | `runner/checks/registry.go` | section smoke test | Suggested placement: after quality/performance, before design/security. |
| Todo | Add rule catalog/fix templates | `rules/catalog_change.go`, `catalog_refactor.go`, fix-template files | metadata tests | Use explicit language coverage. Diff/history rules are repository-wide or configurable. |
| Todo | Add SDK aliases | `pkg/codeguard/sdk_types_config_checks.go`, runtime report aliases | SDK tests | Export config and PR-summary types. |

### Phase 2: Implement change concentration and mixed-concern detection

| Status | Task | Files/area | Tests | Notes |
| --- | --- | --- | --- | --- |
| Todo | Compute change concentration score | `checks/change/*`, `runner/support/diff_scope.go` | `TestChangeConcentrationScore` | Inputs: directories touched, layers touched, public interfaces changed, unrelated rule families triggered, prod/test file ratio. |
| Todo | Detect oversized diffs | change check package | `TestChangeOversizedDiff` | Use thresholds configurable by profile. Avoid duplicate warnings when change concentration already fails unless messages are materially different. |
| Todo | Detect mixed concerns | change check package | `TestChangeMixedConcerns` | Classify files by path/layer/domain/test/docs/config/migration/build/deploy; flag unrelated clusters. |
| Todo | Detect mixed refactor and behavior | change check package | `TestChangeMixedRefactorAndBehavior` | Evidence: renames/moves plus changed conditionals/error paths/side effects/public signatures. |
| Todo | Detect unnecessary surface area | change check package | `TestChangeUnnecessarySurfaceArea` | Flag broad edits when touched functionality is narrow. Keep medium confidence unless diff proves interface expansion. |
| Todo | Detect one-use abstraction | quality/change packages | `TestChangeOneUseAbstraction` | Search newly introduced types/functions/interfaces used once. Exempt tests/adapters/generated code. |
| Todo | Detect duplicate helper | quality/change packages | `TestChangeDuplicateHelper` | Reuse duplicate-code token machinery and local helper signatures. |
| Todo | Detect cleanup regression and complexity increase | quality metrics + change package | `TestChangeCleanupRegression` | If PR claims cleanup/refactor via title/branch/commit metadata, flag increased complexity/public surface/dependencies. |
| Todo | Detect move without verification | change package | `TestChangeMoveWithoutVerification` | File moves/renames without tests or behavior-preservation evidence. |

### Phase 3: Implement safe-refactor analysis

| Status | Task | Files/area | Tests | Notes |
| --- | --- | --- | --- | --- |
| Todo | Add before/after signature extraction | language parser helpers | `TestRefactorPublicContractChanged` | Compare exported signatures, public methods, route/API schemas, and package exports. |
| Todo | Add behavior-preservation evidence model | `checks/change/refactor*.go` | `TestRefactorBehaviorChangeDetected` | Evidence categories: return paths, error paths, side-effect calls, DB writes, event emissions, network calls, auth checks, mutation order. |
| Todo | Detect error-path changes | Go/TS/Python helpers | `TestRefactorErrorPathChanged` | Flag removed/changed error checks, swallowed errors, changed wrapping, or changed fallback behavior. |
| Todo | Detect side-effect-order changes | parser helpers | `TestRefactorSideEffectOrderChanged` | Track ordered calls to writes, publishes, network, filesystem, auth, logging only when order matters. |
| Todo | Detect visibility expansion | parser helpers | `TestRefactorVisibilityExpanded` | Exported identifiers, public class members, widened package/module exports. |
| Todo | Detect dependency direction worsened | design graph + change package | `TestRefactorDependencyDirectionWorsened` | Reuse design graph edges where possible. |
| Todo | Detect duplicate/dead implementation left behind | quality clone/dead-code helpers | `TestRefactorLeftBehindCode` | Flag old and new implementation both present after extraction/move. |
| Todo | Compute `refactor_confidence` | PR-summary postprocessor | `TestPRSummaryRefactorConfidence` | High when mostly moves/renames/extractions and tests preserved; low when error/side-effect/public-contract changes appear. |

### Phase 4: Expand testability checks

| Status | Task | Files/area | Tests | Notes |
| --- | --- | --- | --- | --- |
| Todo | Detect behavior changes without tests | change + CI/test package | `TestTestingBehaviorChangeWithoutTest` | Compare changed production files to changed test files. Use route/API/domain behavior markers. |
| Todo | Detect failure-path tests missing | test-quality package | `TestTestingFailurePathMissing` | High-risk changes need tests covering errors, retries, auth denial, partial failure, invalid inputs, cancellation. |
| Todo | Detect hardwired dependencies | quality/design package | `TestTestingHardwiredDependency` | Direct construction of clients/clocks/random/filesystem/network/env in business/domain code. |
| Todo | Detect nondeterministic domain logic | quality/change package | `TestTestingNondeterministicDomainLogic` | Direct clock/random/filesystem/network/env access in domain code; allow injected interfaces/wrappers. |
| Todo | Detect legacy hotspot uncovered | history + change package | `TestTestingLegacyHotspotUncovered` | Combine churn/complexity/hotspot with missing characterization tests. |

### Phase 5: Implement local quality precision

| Status | Task | Files/area | Tests | Notes |
| --- | --- | --- | --- | --- |
| Todo | Add naming rule catalog and glossary config | `core/config_rule_types.go`, `rules/catalog_naming.go` | metadata/config tests | Support generic identifiers and optional domain glossary. |
| Todo | Detect generic/misleading names | quality parsers | `TestNamingGenericIdentifier`, `TestNamingBehaviorMismatch` | Contextual: avoid flagging conventional loop variables or test data where acceptable. |
| Todo | Detect vocabulary drift | glossary/config + parser indexes | `TestNamingDomainVocabularyDrift` | Preferred/avoid terms across API, DB, service, UI. |
| Todo | Add function semantic-responsibility count | quality metrics | `TestFunctionSemanticResponsibilityCount` | Count validation, load, auth, charge, write, send, emit, transform responsibilities. |
| Todo | Detect function responsibility and contract issues | quality parsers | function rule tests | Command/query mix, hidden mutation, partial-result returns, inconsistent return semantics, orchestration/domain mix. |
| Todo | Expand error handling rules | Go/TS/Python quality parsers | error rule tests | Logged-and-returned, context lost, wrong abstraction level, cleanup ignored, fallback hides corruption, exception control flow. |
| Todo | Add defensive boundary classification | config + parser helpers | defensive rule tests | Distinguish internal trusted code from public API, persistence, event/message, filesystem/network, and user-input boundaries. |
| Todo | Implement defensive boundary checks | parser helpers | defensive rule tests | Validation, impossible state, unchecked assertions/conversions, overflow, bounds, exhaustive cases, schema validation, fail-open auth. |

### Phase 6: Maintainability delta and history-aware smells

| Status | Task | Files/area | Tests | Notes |
| --- | --- | --- | --- | --- |
| Todo | Add maintainability metrics snapshot | quality metrics + history support | `TestMaintainabilityDelta` | Before/after: complexity, cognitive/nesting, duplication, dependency edges, public surface, testability, size. |
| Todo | Compute `maintainability_delta` | PR-summary postprocessor | `TestPRSummaryMaintainabilityDelta` | Positive means safer/simpler; negative means regression. Include evidence list. |
| Todo | Detect public surface/dependency/duplication growth | quality/design/history packages | maintainability rule tests | Use existing design graph and clone detector where possible. |
| Todo | Detect high-churn hotspots | `internal/codeguard/history/*` | history smell tests | Combine churn and complexity. Handle missing git history gracefully. |
| Todo | Detect shotgun surgery/divergent change history | history support | history smell tests | Files that change together; file changed for many unrelated reasons. |
| Todo | Detect repeat defect/unstable interface/ownership gaps | history + ownership config | history smell tests | Start warn-only; high false-positive potential. |
| Todo | Compute `change_safety` | PR-summary postprocessor | `TestPRSummaryChangeSafety` | Inputs: mixed concerns, oversized diff, missing tests, failure-path gaps, refactor confidence, maintainability delta, high-risk findings. |

### Phase 7: Reporting, docs, and rollout

| Status | Task | Files/area | Tests | Notes |
| --- | --- | --- | --- | --- |
| Todo | Add/extend `pr_summary` artifact | `core/report_artifact_types.go`, runner postprocessor | serialization/report tests | Coordinate with production branch. If artifact already exists, extend additively. |
| Todo | Render compact text/GitHub-comment block | `report/write.go`, `report/github_comment.go` | report tests | Do not change existing `Summary:` sentence; do not emit metrics as annotations. |
| Todo | Update docs after behavior lands | `docs/checks.md`, `docs/features.md`, `docs/ai-quality.md`, `README.md` | docs/self-scan | Clearly mark confidence-based/history-aware checks and tuning. |
| Todo | Add examples | `examples/codeguard.json`, `.codeguard/codeguard.yaml` if appropriate | `make codeguard-ci` | Keep aggressive rules opt-in or profile-gated. |

## Confidence policy

- High confidence: direct AST/diff evidence of public contract changes, missing tests for changed exported behavior, visibility expansion, swallowed errors, unchecked boundary input, fail-open auth, or complexity/duplication/public-surface regression.
- Medium confidence: one-use abstractions, mixed concerns, duplicated helpers, hardwired dependencies, semantic responsibility count, vocabulary drift.
- Low confidence: history-only smells and inferred misleading names without direct behavior evidence.

Every finding should include enough evidence for a reviewer to decide quickly:

- what changed;
- why it affects review/change safety;
- what test or refactor evidence is missing;
- whether confidence is high/medium/low.

## Profile behavior target

| Profile | Behavior |
| --- | --- |
| Startup | Warn on oversized/mixed diffs and severe local quality regressions. Do not block most heuristics. |
| Strict | Block new complexity, error-handling, testing, contract, and reliability-adjacent regressions. Warn on smells. |
| Enterprise | Strict plus hotspot/history, ownership gaps, change amplification, and public-surface governance. |
| AI-safe | Strict plus stronger oversized diff, duplicated code, fabricated/unknown APIs, weak error handling, missing tests, inconsistent local idioms, and unnecessary abstractions. |

## Acceptance criteria

- New config fields validate and round-trip in JSON/YAML.
- New rule metadata includes fix templates and explicit language coverage.
- Diff-only change/refactor checks do not produce noise in full scans unless explicitly enabled.
- `pr_summary` includes deterministic `change_safety`, `maintainability_delta`, and `refactor_confidence` metrics.
- At least one vertical slice exists for Go:
  - behavior change without tests;
  - mixed refactor and behavior;
  - public contract changed;
  - error path changed;
  - maintainability regression.
- Existing JSON/SARIF/GitHub annotations/text summary compatibility is preserved.
- History-aware checks degrade gracefully when git history is unavailable or shallow.
- Targeted tests and `make test` pass before push/PR.

## Verification plan

Targeted during implementation:

```sh
go test ./internal/codeguard/config ./internal/codeguard/rules ./internal/codeguard/runner
go test ./tests/codeguard ./tests/checks ./tests/cli -run 'Test.*(Change|Refactor|Maintainability|Testing|Naming|Function|Error|Defensive|PRSummary)'
go test ./tests/checks -run 'TestWriteReport|TestReport|TestSARIF|TestGitHub'
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

## Integration/QA finish-out checklist

Branch completion criteria:

- [ ] Workstream B/C/D/E commits are all integrated on `feature/change-safety-testability-refactors` with no untracked or unstaged worker leftovers.
- [ ] Rule metadata and fix-template coverage match the implemented rule IDs; metadata tests pass for every new `change.*`, `testing.*`, `naming.*`, `function.*`, `error.*`, `defensive.*`, and `maintainability.*` rule.
- [ ] Detector tests pass for implemented change/testability/refactor/local-quality behavior across Go, Python, TypeScript, JavaScript, and C++ fixtures where support landed.
- [ ] `pr_summary` keeps `production_risk` compatible and adds deterministic artifact-only `change_safety`, `maintainability_delta`, and `refactor_confidence` metrics.
- [ ] Final generated/profile docs and glossary describe only implemented, profile-gated support; no planned-only rules are presented as shipped.

Likely integration conflict points:

- Workstream B and C both touch `internal/codeguard/checks/change/**`; keep testability helpers isolated and verify the change section registry wires both detector groups once.
  - Observed 2026-07-27: current workspace has `internal/codeguard/checks/change/testability.go` redeclaring `sectionID`, `sectionName`, `Run`, and `enabled` from `change.go`; B/C need a single package entrypoint before Go tests can compile.
- Workstream D extends shared `pr_summary` artifact types and clone/report behavior; re-check SDK/runtime aliases and report serialization after all metric-producing findings land.
- Workstream E findings feed Workstream D metric grouping; verify `naming.*`, `function.*`, `error.*`, `defensive.*`, and `maintainability.*` rule IDs are grouped intentionally.
- Gauss docs/check glossary must be reconciled after detector support is final so docs do not outrun implementation.

Required pre-merge gates:

```sh
go test ./internal/codeguard/... ./pkg/codeguard ./tests/cli
go test ./tests/checks -run 'Test(Change|Testing|Naming|Function|Error|Defensive|Maintainability)'
go test ./internal/codeguard/runner ./tests/codeguard ./tests/checks -run 'Test.*PRSummary|TestWriteReport'
```

Broader final gates when the branch is quiescent:

```sh
make fmt-check
make test
make codeguard-ci
make ci
```

## Merge checklist

- [ ] Rule IDs are stable and grouped by owning family.
- [ ] Every built-in rule has a fix template.
- [ ] New config has defaults, validation, examples, and SDK aliases.
- [ ] Diff/history checks are deterministic and handle shallow history.
- [ ] PR-summary metrics have deterministic evidence ordering.
- [ ] SARIF/GitHub annotations remain finding-only.
- [ ] Product docs distinguish implemented, profile-gated, and confidence-based behavior.
- [ ] `make test` passes.
- [ ] `make ci` passes or any skipped gate is explicitly documented.
