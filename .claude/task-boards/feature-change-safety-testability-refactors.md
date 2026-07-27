# Task board: feature/change-safety-testability-refactors

Status: final gate prep
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
- Worker handoffs have landed and this board has been reconciled against the implemented rule IDs, tests, and docs in the final parity audit below.

## Final parity audit

Audited on 2026-07-27 against `internal/codeguard/rules/catalog_change_safety.go`, `internal/codeguard/rules/catalog_fix_templates_change_safety.go`, `internal/codeguard/checks/change/*`, `internal/codeguard/checks/quality/quality_precision.go`, `internal/codeguard/runner/pr_summary.go`, `tests/checks/*change*`, `tests/checks/*testability*`, `tests/checks/*precision*`, `tests/checks/*maintainability*`, `internal/codeguard/runner/pr_summary_test.go`, and the SDK metadata tests.

Implemented detector subset in the current worktree:

- `change.oversized-diff`
- `change.mixed-concerns`
- `change.too-many-concerns`
- `change.mixed-refactor-and-behavior`
- `change.unnecessary-surface-area`
- `change.one-use-abstraction`
- `change.duplicate-helper`
- `change.cleanup-regression`
- `change.complexity-increased`
- `change.move-without-verification`
- `testing.behavior-change-without-test`
- `testing.failure-path-missing`
- `testing.hardwired-dependency`
- `testing.nondeterministic-domain-logic`
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
- `maintainability.hotspot`
- `maintainability.high-churn-hotspot`
- `maintainability.repeat-defect-area`
- `maintainability.unstable-interface`
- `maintainability.change-amplification`
- `smell.shotgun-surgery-history`
- `smell.divergent-change-history`
- `pr_summary.change_safety`
- `pr_summary.maintainability_delta`
- `pr_summary.refactor_confidence`

Catalog/config/deferred IDs for this branch:

- `testing.legacy-hotspot-uncovered`: cataloged/configured, intentionally non-emitting without reliable history/hotspot inputs.
- `refactor.*`: direct detector code and `tests/checks/refactor_test.go` are present and green in the current implementation. `pr_summary.refactor_confidence` rolls up `refactor.*` findings plus implemented mixed-refactor/move-without-verification signals.

Metadata/doc parity:

- Every built-in rule in the branch catalog has explicit language coverage through the rule metadata helpers.
- Every branch catalog rule has a populated guided fix template.
- `docs/checks.md` and `docs/features.md` distinguish implemented detectors from catalog/planned IDs so planned-only behavior is not described as shipped.
- `examples/codeguard.json` was updated for the final `change_rules` config surface after concurrent config changes added direct refactor left-behind toggles.

Current gate blocker:

- None known after the safe-refactor worker landed. Final branch gates still need to run on the quiescent branch before PR handoff.
- Full-parity worker assignments after MVP landed:
  - Einstein (`019fa485-4e06-73e1-9cd8-67592526456d`): Phase 3 safe-refactor detectors.
  - Nietzsche (`019fa485-8175-7540-9df5-dc0ca89ac3bd`): remaining Phase 2 change-smell detectors.
  - McClintock (`019fa485-b88b-71c0-a426-53fc5cee95a4`): Phase 6 history-aware maintainability and smell signals.
  - Hume (`019fa485-f437-74b1-b617-62805c7ab20a`): docs/task-board parity audit and final checklist.

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

Status: complete for the Phase 1/2 detector subset and cleanup-style change-smell detectors.

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

Status: complete for behavior-change, failure-path, hardwired-dependency, and nondeterministic-domain detectors; legacy-hotspot emission deferred until reliable history inputs are available.

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

Status: complete for additive artifact fields and deterministic finding-family rollups.

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

Status: complete for the small high-value subset plus history-aware maintainability/smell detectors that degrade gracefully when git history is unavailable.

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

This inventory is the branch catalog and planning map. It is not a shipped-detector list. The final parity audit above is the source of truth for which IDs currently emit findings.

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
| Done | Decide whether to extend existing sections or add new sections | `core/config_types.go`, `runner/checks/registry.go` | config + section tests | Added `checks.change` for diff/testability and kept local naming/function/error/defensive/maintainability precision in `Code Quality`. |
| Done | Define confidence policy | rule metadata + docs | report confidence tests | Implemented findings carry explicit confidence; docs tell users to treat medium-confidence heuristics as review cues. |
| Done | Define profile behavior | `internal/codeguard/config/profile.go` | profile tests | Startup leaves change off; strict/enterprise enable it; AI-safe enables it with tighter diff/test-ratio budgets. |
| Done | Define shared PR-summary artifact contract | `core/report_artifact_types.go` | report serialization tests | Additive `pr_summary` fields landed; metrics remain artifact-only and do not create GitHub annotations. |

### Phase 1: Add change-analysis infrastructure

| Status | Task | Files/area | Tests | Notes |
| --- | --- | --- | --- | --- |
| Done | Add `ChangeRulesConfig` | `core/config_rule_types.go`, `core/config_types.go` | config tests | Thresholds landed: max files, dirs, public interfaces, changed lines, concern families, and min test/prod ratio. |
| Done | Add defaults/examples/validation | `config/defaults*.go`, `config/example*.go`, config validation | `go test ./internal/codeguard/config ./tests/codeguard` | Defaults and validation landed; example config still reflects the full config surface. |
| Done | Add change section package | `internal/codeguard/checks/change/change.go` | `tests/checks/change_test.go` | Diff-mode section landed; full scans no-op. |
| Done | Register section | `runner/checks/registry.go` | section smoke test | Registered as a first-class check family. |
| Done | Add rule catalog/fix templates | `rules/catalog_change_safety.go`, `catalog_fix_templates_change_safety.go` | metadata tests | Branch catalog has explicit language coverage and populated fix templates. Some IDs are catalog/planned only. |
| Done | Add SDK aliases | `pkg/codeguard/sdk_types_config_checks.go`, runtime report aliases | SDK tests | Config and PR-summary SDK aliases landed. |

### Phase 2: Implement change concentration and mixed-concern detection

| Status | Task | Files/area | Tests | Notes |
| --- | --- | --- | --- | --- |
| Done | Compute change concentration evidence | `checks/change/*`, `runner/support/diff_scope.go` | `tests/checks/change_test.go` | Inputs landed: directories, layers, concern families, public-surface files, changed lines, moved files, and prod/test ratio metadata. |
| Done | Detect oversized diffs | change check package | `TestChangeOversizedDiffUsesConfiguredThresholds` | Uses configurable thresholds and evidence metadata. |
| Done | Detect mixed concerns | change check package | `TestChangeDetectsMixedAndTooManyConcerns` | Path/layer/concern classification landed. |
| Done | Detect mixed refactor and behavior | change check package | `TestChangeDetectsMoveMixedWithBehaviorAndNoVerification` | Evidence is file movement plus behavior-bearing production edits. |
| Done | Detect unnecessary surface area | change check package | `TestChangeDetectsUnnecessarySurfaceArea` | Uses public-surface file budget evidence. |
| Done | Detect one-use abstraction | quality/change packages | `TestChangeOneUseAbstractionDetectsGoInterface`, TS and negative tests | New interfaces/abstract boundaries with only one repository reference. |
| Done | Detect duplicate helper | quality/change packages | `TestChangeDuplicateHelperDetectsGoDuplicate`, TS and negative tests | Finds changed helper bodies that duplicate existing production helper logic. |
| Done | Detect cleanup regression and complexity increase | quality metrics + change package | `TestChangeComplexityIncreasedDetectsPythonBranchGrowth`, `TestChangeCleanupRegressionDetectsClaimedCleanupComplexityGrowth`, negative tests | Complexity increase is general diff evidence; cleanup regression requires cleanup/refactor/chore wording evidence. |
| Done | Detect move without verification | change package | `TestChangeDetectsMoveMixedWithBehaviorAndNoVerification`, `TestChangeMoveWithVerificationDoesNotWarnAboutMissingVerification` | File moves/renames without tests or verification files. |

### Phase 3: Implement safe-refactor analysis

| Status | Task | Files/area | Tests | Notes |
| --- | --- | --- | --- | --- |
| Done | Add before/after signature extraction | `internal/codeguard/checks/change/refactor.go` | `tests/checks/refactor_test.go` | Compares conservative public signatures and source evidence in diff scans. |
| Done | Add behavior-preservation evidence model | `internal/codeguard/checks/change/refactor.go` | `tests/checks/refactor_test.go` | Evidence categories include behavior, public contracts, errors, side effects, visibility, dependency direction, duplicate implementations, and dead paths. |
| Done | Detect error-path changes | `internal/codeguard/checks/change/refactor.go` | `tests/checks/refactor_test.go` | Flags changed error/fallback/panic/throw behavior in refactor-labeled diffs. |
| Done | Detect side-effect-order changes | `internal/codeguard/checks/change/refactor.go` | `tests/checks/refactor_test.go` | Tracks ordered side-effect call evidence conservatively. |
| Done | Detect visibility expansion | `internal/codeguard/checks/change/refactor.go` | `tests/checks/refactor_test.go` | Flags widened public/exported API evidence. |
| Done | Detect dependency direction worsened | `internal/codeguard/checks/change/refactor.go` | `tests/checks/refactor_test.go` | Flags new inward infrastructure/framework dependencies in refactor-labeled diffs. |
| Done | Detect duplicate/dead implementation left behind | `internal/codeguard/checks/change/refactor.go` | `TestRefactorDetectsDuplicateImplementationAndDeadPathLeftBehind` | Flags duplicate implementations and obsolete branch/path leftovers. |
| Done | Compute `refactor_confidence` | PR-summary postprocessor | `TestAddPRSummaryArtifactAddsChangeSafetyMetrics` | Artifact rollup landed. It consumes `refactor.*` findings and implemented mixed-refactor/move-without-verification findings. |

### Phase 4: Expand testability checks

| Status | Task | Files/area | Tests | Notes |
| --- | --- | --- | --- | --- |
| Done | Detect behavior changes without tests | change + CI/test package | `TestTestingBehaviorChangeWithoutTestAcrossLanguages`, suppression test | Compares changed production files to changed test files across Go, Python, TypeScript, JavaScript, and C++. |
| Done | Detect failure-path tests missing | test-quality package | `TestTestingFailurePathMissingRequiresFailureTestEvidence` | Flags changed error/retry/fallback/auth/external paths without failure-test evidence. |
| Done | Detect hardwired dependencies | quality/design package | `TestTestingHardwiredDependencyFindsChangedProductionLine` | Flags direct construction/use of external dependencies in changed production lines. |
| Done | Detect nondeterministic domain logic | quality/change package | `TestTestingNondeterministicDomainLogicFindsDomainClock` | Flags direct clock/random/env/process access in domain paths. |
| Deferred | Detect legacy hotspot uncovered | history + change package | `TestTestingLegacyHotspotUncoveredDoesNotEmitWithoutHistory` | Catalog/config/fix-template exists; intentionally non-emitting without reliable history/hotspot inputs. |

### Phase 5: Implement local quality precision

| Status | Task | Files/area | Tests | Notes |
| --- | --- | --- | --- | --- |
| Done | Add naming/function/error/defensive/maintainability subset catalog | `rules/catalog_change_safety.go` | metadata/config tests | Implemented subset is cataloged with fix templates and explicit language coverage. Domain glossary config deferred. |
| Done | Detect generic names | quality parsers | `TestNamingGenericIdentifierWarnsForPlaceholderNames`, fixture negative test | Contextual fixture/test suppression landed. Broader misleading-name rules deferred. |
| Deferred | Detect vocabulary drift | glossary/config + parser indexes | planned `TestNamingDomainVocabularyDrift` | Deferred. Existing AI naming drift is separate from this local precision subset. |
| Deferred | Add function semantic-responsibility count | quality metrics | planned `TestFunctionSemanticResponsibilityCount` | Deferred. |
| Done | Detect function subset | quality parsers | `TestFunctionExcessiveParametersWarnsWithSpecificRule`, `TestFunctionMixedAbstractionLevelWarnsForInfrastructureInsideOrchestration`, `TestFunctionCommandQueryMixWarnsWhenQueryMutatesState` | Landed excessive parameters, mixed abstraction level, and command/query mix. Other function contract/responsibility rules deferred. |
| Done | Expand error handling subset | Go/TS/Python quality parsers | `TestErrorLoggedAndIgnoredWarnsWhenErrorBecomesSuccess`, `TestErrorContextLostWarnsForBareErrorReturn` | Landed logged-and-ignored and context-lost. Other error IDs remain outside this branch subset. |
| Deferred | Add defensive boundary classification | config + parser helpers | defensive rule tests | Deferred. |
| Done | Implement defensive subset | parser helpers | `TestDefensiveUncheckedTypeAssertionWarnsForSingleValueAssertion`, safe assertion negative test, `TestDefensiveUnsafeNumericConversionWarnsForNarrowingConversion` | Landed unchecked type assertion and unsafe numeric conversion. Broader boundary/overflow/schema/fail-open rules deferred. |

### Phase 6: Maintainability delta and history-aware smells

| Status | Task | Files/area | Tests | Notes |
| --- | --- | --- | --- | --- |
| Done | Add maintainability delta/history subset | quality metrics + history support | `TestMaintainabilityPublicSurfaceGrowthWarnsInDiffScan`, `TestMaintainabilityDependencyGrowthWarnsInDiffScan`, history tests pending final gate | Before/after public-surface and direct-dependency counts landed; bounded git-history maintainability/smell signals landed and skip when history is unavailable. Complexity/nesting deltas are also represented through `change.complexity-increased` and `change.cleanup-regression`. |
| Done | Compute `maintainability_delta` | PR-summary postprocessor | `TestAddPRSummaryArtifactAddsChangeSafetyMetrics` | Artifact rollup landed over maintainability, quality, error, and defensive findings. |
| Done | Detect public surface/dependency growth | quality/design/history packages | maintainability rule tests | Public-surface and dependency growth landed. Duplication growth remains deferred outside duplicate-helper detection. |
| Done | Detect high-churn hotspots | `internal/codeguard/history/*` | history tests pending final gate | Bounded local git-history collection landed; unavailable history produces no findings. |
| Done | Detect shotgun surgery/divergent change history | history support | history tests pending final gate | Co-change and commit-subject concern-family signals landed. |
| Done/Deferred | Detect repeat defect/unstable interface/ownership gaps | history + ownership config | history tests pending final gate | Repeat-defect and unstable-interface signals landed. Ownership-gap detection remains deferred. |
| Done | Compute `change_safety` | PR-summary postprocessor | `TestAddPRSummaryArtifactAddsChangeSafetyMetrics`, `TestAddPRSummaryArtifactPublishesChangeMetricsWithoutProductionRisk` | Artifact rollup landed over implemented `change.*` and `testing.*` findings. |

### Phase 7: Reporting, docs, and rollout

| Status | Task | Files/area | Tests | Notes |
| --- | --- | --- | --- | --- |
| Done | Add/extend `pr_summary` artifact | `core/report_artifact_types.go`, runner postprocessor | serialization/report tests | Existing artifact extended additively with `change_safety`, `maintainability_delta`, and `refactor_confidence`. |
| Done | Preserve compact text/GitHub-comment behavior | `report/write.go`, `report/github_comment.go` | `TestPRSummaryMetricsAreArtifactOnlyForGitHubAnnotations` | Existing `Summary:` sentence unchanged; metrics do not emit as annotations. |
| Done | Update docs after behavior lands | `docs/checks.md`, `docs/features.md` | docs/metadata tests | Docs now mark implemented detector subset vs catalog/planned IDs. README did not need a user-facing summary update. |
| Done | Add examples | `examples/codeguard.json` | `python3 -m json.tool examples/codeguard.json` | Updated for the final `change_rules` config surface after direct refactor left-behind toggles were added. |

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

- Done: new config fields validate and round-trip in JSON/YAML.
- Done: new rule metadata includes fix templates and explicit language coverage.
- Done: implemented diff-only change/testability checks do not produce noise in full scans.
- Done: `pr_summary` includes deterministic `change_safety`, `maintainability_delta`, and `refactor_confidence` metrics.
- Done: vertical slices exist for:
  - behavior change without tests;
  - mixed refactor and behavior;
  - maintainability regression via public-surface/dependency growth and change complexity/cleanup regression signals.
- In rollout/blocked: direct `refactor.*` detector code and tests exist, but `TestRefactorDetectsDuplicateImplementationAndDeadPathLeftBehind` is failing.
- Done: existing JSON/SARIF/GitHub annotations/text summary compatibility is preserved for PR-summary metrics.
- Done/Deferred: history-aware checks degrade gracefully by skipping `testing.legacy-hotspot-uncovered` without reliable hotspot inputs; richer maintainability/smell history detectors landed and skip when git history is unavailable.
- Pending final gate: targeted docs/metadata tests should pass before PR; full `make ci` should wait until no implementation workers are actively changing the branch.

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

## Final PR checklist

- [x] Task board reconciled against implemented rule IDs and tests.
- [x] Stale Todo rows converted to Done/Deferred states.
- [x] Docs distinguish implemented detectors from catalog/planned IDs.
- [x] Built-in branch rule metadata checked for explicit language coverage and populated fix templates.
- [x] `examples/codeguard.json` updated for the final `change_rules` config shape.
- [ ] Fix direct `refactor.*` test failure: `TestRefactorDetectsDuplicateImplementationAndDeadPathLeftBehind` is missing `refactor.duplicate-implementation-left-behind`.
- [x] Run targeted docs/metadata tests:
  `env -u GOROOT GOCACHE=/private/tmp/codeguard-go-cache go test ./internal/codeguard/config ./tests/cli -run 'TestPolicyProfileDocumentationMatchesGeneratedComparison|TestSDKRuleMetadata|TestSDKRuleMetadataFixTemplatesPopulated'`
- [x] Validate sample JSON:
  `python3 -m json.tool examples/codeguard.json`
- [x] Run narrow change/maintainability detector checks:
  `env -u GOROOT GOCACHE=/private/tmp/codeguard-go-cache go test ./tests/checks -run 'Test(Change|Maintainability)'`
- [x] Run direct refactor detector check and record blocker:
  `env -u GOROOT GOCACHE=/private/tmp/codeguard-go-cache go test ./tests/checks -run 'TestRefactor'` currently fails in `TestRefactorDetectsDuplicateImplementationAndDeadPathLeftBehind`.
- [ ] Run broader final gates after active implementation work is finished:
  `make fmt-check`, `make test`, `make codeguard-ci`, and `make ci` when practical.

## PR summary draft

This branch adds a final-tested change-safety rollout focused on PR reviewability and testability. It introduces the `checks.change` config family, diff-mode concentration detectors, testability detectors for changed behavior/failure paths/hardwired dependencies/nondeterministic domain logic, a local-quality precision subset for naming/function/error/defensive findings, and maintainability-delta findings for public-surface/dependency growth. The PR-summary artifact is extended additively with `change_safety`, `maintainability_delta`, and `refactor_confidence` rollups without changing GitHub annotations or per-rule severities.

Catalog/config IDs for direct `refactor.*` checks are included with metadata, explicit language coverage, and fix templates for rollout compatibility, but they are documented as in-rollout until the `TestRefactor` target passes.

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
