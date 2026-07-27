# Task board: feature/operability-design-delivery-governance

Status: staging
Branch: feature/operability-design-delivery-governance
Last updated: 2026-07-27
Not final product docs: this is implementation planning for the branch, not shipped user-facing documentation.

## Goal

Make CodeGuard evaluate whether production code is operable, locally well-designed, and safe to roll out.

This branch owns:

- observability and operations readiness;
- abstraction-quality and local software-design checks;
- delivery-governance and rollout-safety checks;
- enterprise/profile behavior for ownership, runbooks, service compatibility, supply-chain provenance, and deployment verification.

The product target is to catch changes that are technically correct but hard to operate, hard to change, or unsafe to deploy.

## Workstream D audit and reconciliation checklist

Status: prep-audited. As of 2026-07-27, the branch task board lists the intended rule inventory, but the shipped rule catalogs, detector packages, config fields, profile defaults, and user-facing docs for this branch have not landed yet. Keep `docs/checks.md`, `docs/features.md`, `docs/production.md`, `README.md`, and `examples/codeguard.json` unchanged until matching behavior exists in code and tests.

Workstream D owns final reconciliation after implementation slices merge:

- Confirm new rule metadata exists for every implemented `observability.*`, `operations.*`, `delivery.*`, `ci.*`, `supply_chain.*`, `design.*`, and `quality.*` rule in scope.
- Confirm every new rule has language coverage, profile behavior, examples where useful, and a fix template or explicit guided remediation.
- Confirm SDK aliases/config API cover any new config structs or rule toggles.
- Confirm profile comparison output reflects startup, strict, enterprise, and AI-safe behavior for the landed rules.
- Update shipped docs only after detector behavior and tests exist.
- Keep this task board accurate as implementation workers land commits; mark a task Done only after code, tests, metadata, and docs/profile behavior are reconciled.
- Add the final PR-summary draft section once the branch has enough implementation to summarize accurately.

Workstream D verification commands:

```sh
env -u GOROOT GOCACHE=/private/tmp/codeguard-operability-go-cache go test ./internal/codeguard/config ./tests/cli ./tests/codeguard -run 'Test.*(Profile|Metadata|Config|Documentation|SDK)'
git diff --check
```

## Non-goals

- Do not implement reliability/data-correctness detectors owned by `feature/production-reliability-data-readiness`.
- Do not implement change/refactor/testability metrics owned by `feature/change-safety-testability-refactors`.
- Do not duplicate existing architecture-boundary checks unless the new rule is about local abstraction quality or operability.
- Do not block rollout-governance findings by default in startup/strict without profile-specific staging.

## Product split

This branch owns:

- Rule families: `observability.*`, `operations.*`, additional `design.*`, additional `quality.*`, additional `delivery.*`, additional `ci.*`, and `supply-chain.missing-provenance`.
- Enterprise behavior: ownership, observability, rollout safety, supply chain, runbooks, and service compatibility.
- Production-risk inputs: observability/delivery/operations findings can feed the `production_risk` metric once the shared `pr_summary` artifact exists.

Adjacent branch contracts:

- `feature/production-reliability-data-readiness` owns the initial `production_risk` artifact field and reliability/data signals.
- `feature/change-safety-testability-refactors` owns `maintainability_delta`; this branch may add design-governance findings that become inputs later.

## Existing repo seams to reuse

- Existing design rules/catalogs: `internal/codeguard/checks/design/*`, `internal/codeguard/rules/catalog_design.go`, `catalog_design_graph.go`, `catalog_design_policy.go`.
- Existing CI/release rules: `internal/codeguard/checks/ci/*`, `internal/codeguard/rules/catalog_test_quality.go`, `catalog_misc.go`.
- Existing supply-chain rules: `internal/codeguard/checks/supplychain/*`, `internal/codeguard/rules/catalog_supplychain.go`.
- Config surface: `internal/codeguard/core/config_types.go`, `internal/codeguard/core/config_rule_types.go`.
- Defaults/examples/validation: `internal/codeguard/config/defaults.go`, `defaults_rules.go`, `example.go`, `validate.go`.
- Runner section registry: `internal/codeguard/runner/checks/registry.go`.
- Rule metadata/fix templates: `internal/codeguard/rules/catalog*.go`, `internal/codeguard/rules/catalog_fix_templates*.go`.
- Report compatibility: `internal/codeguard/report/write.go`, `internal/codeguard/report/github_comment.go`, `internal/codeguard/report/sarif_builders.go`.
- Docs: `docs/checks.md`, `docs/features.md`, `docs/production.md`, `docs/integrations.md`, `README.md`.

## Rule inventory

### Observability and operations

- `observability.unstructured-log`
- `observability.error-without-context`
- `observability.sensitive-log-data`
- `observability.high-cardinality-label`
- `observability.critical-path-uninstrumented`
- `observability.log-and-ignore`
- `observability.shallow-health-check`
- `operations.missing-owner`
- `operations.missing-runbook`

### Abstraction quality and local design

- `design.shallow-module`
- `design.excessive-public-surface`
- `design.pass-through-abstraction`
- `design.configuration-leak`
- `design.temporal-coupling`
- `quality.duplicated-knowledge`
- `design.infrastructure-type-leak`
- `design.persistence-model-leak`
- `design.domain-logic-in-handler`
- `quality.ambiguous-name`
- `quality.boolean-argument`
- `quality.mixed-abstraction-levels`
- `quality.excessive-parameters`
- `quality.primitive-obsession`
- `quality.hidden-side-effect`
- `quality.mutable-global-state`
- `quality.redundant-comment`

### Delivery governance

- `ci.missing-required-gate`
- `ci.mutable-deployment-reference`
- `delivery.missing-rollback-strategy`
- `delivery.unsafe-migration-order`
- `delivery.high-risk-change-without-kill-switch`
- `delivery.missing-post-deploy-verification`
- `supply-chain.missing-provenance`
- `quality.environment-branching`

## Implementation phases

### Phase 0: Decide section and profile shape

| Status | Task | Files/area | Tests | Notes |
| --- | --- | --- | --- | --- |
| Todo | Decide section IDs | `runner/checks/registry.go` | section smoke tests | Suggested new sections: `observability`, `operations`, `delivery`; extend existing `design`, `quality`, `ci`, and `supply_chain` where rule families already exist. |
| Todo | Define enterprise defaults | `config/profile.go` | profile tests | Enterprise should enable ownership, observability, rollout safety, supply-chain provenance, runbooks, and service compatibility. |
| Todo | Define strict/startup behavior | profile/docs | profile tests | Startup warns only. Strict can warn for observability/design and block only existing required CI/security gates. |
| Todo | Define evidence model | rule packages | report confidence tests | Most rules need confidence/evidence rather than binary proof. Avoid shallow style-lint behavior. |

### Phase 1: Add config, catalogs, and section scaffolding

| Status | Task | Files/area | Tests | Notes |
| --- | --- | --- | --- | --- |
| Todo | Add `ObservabilityRulesConfig` | `core/config_rule_types.go`, `core/config_types.go` | config tests | Include structured logger patterns, sensitive-name patterns, metric label deny patterns, critical path patterns, healthcheck path patterns. |
| Todo | Add `OperationsRulesConfig` | config files | config tests | Include owner file patterns, runbook path patterns, critical service path patterns. |
| Todo | Add `DeliveryRulesConfig` | config files | config tests | Include required CI gates, allowed deployment refs, rollback docs patterns, migration ordering config, kill-switch patterns, post-deploy verification patterns. |
| Todo | Add defaults/examples/validation | `config/defaults*.go`, `config/example*.go`, `config/validate_*.go` | `go test ./internal/codeguard/config ./tests/codeguard` | Validate non-empty patterns, positive thresholds, and no conflicting allow/deny refs. |
| Todo | Add SDK aliases | `pkg/codeguard/sdk_types_config_checks.go` | SDK tests | Keep config API complete. |
| Todo | Add catalogs | `rules/catalog_observability.go`, `catalog_operations.go`, `catalog_delivery.go`, extend design/quality/ci/supplychain catalogs | metadata tests | Explicit `LanguageCoverage`. Keep `supply-chain.missing-provenance` spelling aligned with existing prefix convention; repo currently uses `supply_chain.*`, so decide whether to normalize to `supply_chain.missing-provenance` before implementation. |
| Todo | Add fix templates | `rules/catalog_fix_templates_observability.go`, `catalog_fix_templates_delivery.go`, design/quality template files | metadata tests | Mostly guided templates; deterministic only for pinning mutable refs or adding metadata files. |

### Phase 2: Implement observability checks

| Status | Task | Files/area | Tests | Notes |
| --- | --- | --- | --- | --- |
| Todo | Create observability package | `internal/codeguard/checks/observability/observability.go` | `tests/checks/observability_test.go` | Follow section pattern and finalize as `observability`, `Observability`. |
| Todo | Register section | `runner/checks/registry.go` | section smoke test | Run after reliability/data when those exist; otherwise after security/design. |
| Todo | Detect unstructured logs | Go/TS/Python detectors | `TestObservabilityUnstructuredLog` | Flag `fmt.Println`, `console.log`, raw string logs, logger calls without fields in production code. Allow tests/scripts. |
| Todo | Detect errors without context | detectors | `TestObservabilityErrorWithoutContext` | Error logs should include operation/request/customer-safe context. Avoid requiring request IDs in low-level pure functions. |
| Todo | Detect sensitive log data | detectors | `TestObservabilitySensitiveLogData` | Reuse security secret/sensitive-name patterns. Flag tokens, passwords, auth headers, PII-like names in log fields/messages. |
| Todo | Detect high-cardinality metric labels | detectors | `TestObservabilityHighCardinalityLabel` | Flag labels with user_id, email, request_id, path with raw params, UUID/order IDs. Allow configured sanitized labels. |
| Todo | Detect critical paths without instrumentation | path/config + parser helpers | `TestObservabilityCriticalPathUninstrumented` | Critical paths: handlers, jobs, consumers, migrations, payment/write flows. Require span/metric/log evidence based on config. |
| Todo | Detect log-and-ignore | error/log detectors | `TestObservabilityLogAndIgnore` | Distinguish from `error.logged-and-ignored` sibling branch by placing operability-focused log-only failure under observability unless it changes reliability semantics. |
| Todo | Detect shallow health checks | route/config scanner | `TestObservabilityShallowHealthCheck` | Flag health endpoints that only return static OK while critical dependencies exist. Confidence based on dependency evidence. |

### Phase 3: Implement operations ownership/runbook checks

| Status | Task | Files/area | Tests | Notes |
| --- | --- | --- | --- | --- |
| Todo | Create operations package | `internal/codeguard/checks/operations/operations.go` | `tests/checks/operations_test.go` | Repo-level and path-level findings. |
| Todo | Register section | `runner/checks/registry.go` | section smoke test | Enterprise profile should enable by default. |
| Todo | Detect missing service ownership | operations package | `TestOperationsMissingOwner` | Support CODEOWNERS, service catalog files, ownership metadata in config, package-level metadata. |
| Todo | Detect missing runbook metadata | operations package | `TestOperationsMissingRunbook` | Critical systems require runbook links or local runbook files. Allow configured critical path patterns. |
| Todo | Add ownership-gap cross-feed | operations + maintainability later | operations tests | Findings can feed maintainability/production-risk metrics in other branches. |

### Phase 4: Implement abstraction-quality design checks

| Status | Task | Files/area | Tests | Notes |
| --- | --- | --- | --- | --- |
| Todo | Extend design config | `DesignRulesConfig` or new local-design config | config tests | Thresholds: public symbol count, pass-through ratio, temporal coupling evidence, handler/domain path patterns, infrastructure/domain path patterns. |
| Todo | Extend design catalog | `rules/catalog_design.go` or `catalog_design_local.go` | metadata tests | Keep existing design family rather than a competing family. |
| Todo | Detect shallow modules | design package | `TestDesignShallowModule` | Public API surface high but implementation depth/behavior low. Confidence-based. |
| Todo | Detect excessive public surface | design package | `TestDesignExcessivePublicSurface` | Exported symbols/public members per package/module. Exempt SDK packages via config. |
| Todo | Detect pass-through abstractions | design package | `TestDesignPassThroughAbstraction` | Methods/functions that only delegate without policy, translation, validation, or isolation. |
| Todo | Detect configuration leak | design package | `TestDesignConfigurationLeak` | Config structs/options crossing module boundaries or leaking env/deployment concerns into domain code. |
| Todo | Detect temporal coupling | design/history package | `TestDesignTemporalCoupling` | Required call order encoded implicitly. Start with obvious init/use/close or set-before-call patterns. |
| Todo | Detect duplicated business knowledge | quality/design package | `TestQualityDuplicatedKnowledge` | Constants/rules/calculations duplicated across layers. Not the same as token-level duplicate code. |
| Todo | Detect infrastructure type leak | design package | `TestDesignInfrastructureTypeLeak` | DB/HTTP/framework/logger/cloud SDK types in domain packages or public APIs. |
| Todo | Detect persistence model leak | design package | `TestDesignPersistenceModelLeak` | ORM/db model structs returned through public API/handler contracts. |
| Todo | Detect domain logic in handlers/controllers | design package | `TestDesignDomainLogicInHandler` | Handlers should orchestrate/validate/translate, not own business rules. |

### Phase 5: Precision cleanup for quality rules in this branch

| Status | Task | Files/area | Tests | Notes |
| --- | --- | --- | --- | --- |
| Todo | Add/align local quality catalog entries | `rules/catalog_quality.go` or new local quality catalog | metadata tests | These are local design-quality rules that fit existing `quality.*` prefix. |
| Todo | Detect ambiguous names | quality parser helpers | `TestQualityAmbiguousName` | `data`, `manager`, `helper`, `process`, `thing`; avoid one-off test fixture false positives. |
| Todo | Detect boolean arguments | quality parser helpers | `TestQualityBooleanArgument` | Flag public/business functions with behavior-hiding booleans. Allow setters/options/builders. |
| Todo | Detect mixed abstraction levels | quality/design parser helpers | `TestQualityMixedAbstractionLevels` | Coordinate with `function.mixed-abstraction-level` branch later. |
| Todo | Detect primitive obsession | quality parser helpers | `TestQualityPrimitiveObsession` | Repeated raw strings/ints for domain concepts, especially IDs/units/currency. |
| Todo | Detect hidden side effects | quality parser helpers | `TestQualityHiddenSideEffect` | Function name implies query/format/build but mutates state, writes, logs, or performs I/O. |
| Todo | Detect mutable global state | quality parser helpers | `TestQualityMutableGlobalState` | Flag mutable package/module globals in production code; allow constants and guarded test hooks. |
| Todo | Detect redundant comments | quality text/parser helpers | `TestQualityRedundantComment` | Comments that only restate nearby code. Low confidence; warn only. |

### Phase 6: Implement delivery governance checks

| Status | Task | Files/area | Tests | Notes |
| --- | --- | --- | --- | --- |
| Todo | Extend CI package or create delivery package | `internal/codeguard/checks/ci/*`, `internal/codeguard/checks/delivery/*` | `tests/checks/delivery_test.go` | Use `ci.*` for CI gate config; use `delivery.*` for rollout strategy. |
| Todo | Detect missing required CI gates | CI package | `TestCIMissingRequiredGate` | Validate required workflows/jobs/check names in `.github/workflows`, config, or CI provider files. |
| Todo | Detect mutable deployment references | CI/delivery package | `TestCIMutableDeploymentReference` | Floating GitHub Actions refs, image tags like `latest`, branch deploy refs, unpinned external actions. |
| Todo | Detect missing rollback strategy | delivery package | `TestDeliveryMissingRollbackStrategy` | High-risk deployment/migration changes require rollback docs/config/runbook reference. |
| Todo | Detect unsafe migration ordering | delivery + data migration scanner | `TestDeliveryUnsafeMigrationOrder` | Coordinate with data branch migration rule; this branch focuses rollout sequencing evidence. |
| Todo | Detect high-risk feature without kill switch | delivery package | `TestDeliveryHighRiskNoKillSwitch` | New critical path, payment/auth/data migration behavior needs feature flag/kill switch evidence. |
| Todo | Detect missing post-deploy verification | delivery package | `TestDeliveryMissingPostDeployVerification` | Deploy workflows should verify health/SLO/smoke checks after production rollout. |
| Todo | Detect missing artifact provenance | supplychain package | `TestSupplyChainMissingProvenance` | Prefer `supply_chain.missing-provenance` to match existing prefix unless compatibility requires hyphen. Check SBOM/attestation/provenance files or workflow steps. |
| Todo | Detect environment branching in source | quality/delivery package | `TestQualityEnvironmentBranching` | Flag production/staging/dev branching embedded in domain/source code. Allow config/bootstrap boundaries. |

### Phase 7: Production-risk integration and docs

| Status | Task | Files/area | Tests | Notes |
| --- | --- | --- | --- | --- |
| Todo | Feed observability/operations/delivery findings into production risk | `runner/pr_summary.go` if present | PR-summary tests | Additive only; do not make this branch depend on unmerged artifact work unless rebased after branch 1. |
| Todo | Render optional GitHub-comment block | `report/github_comment.go` | report tests | Only if `pr_summary` exists. Keep annotations finding-only. |
| Todo | Update docs after behavior lands | `docs/checks.md`, `docs/features.md`, `docs/production.md`, `docs/integrations.md`, `README.md` | docs/self-scan | Explain enterprise vs startup/strict behavior and tuning. |
| Todo | Update examples | `examples/codeguard.json`, `.codeguard/codeguard.yaml` if appropriate | `make codeguard-ci` | Enterprise-only checks may be too noisy for default example. |

## Confidence policy

- High confidence: mutable deployment refs, missing required CI gate, sensitive log fields, high-cardinality metric labels, infrastructure type leaked through public/domain APIs, mutable global state, unpinned provenance requirement.
- Medium confidence: shallow health checks, critical path without instrumentation, missing owner/runbook, domain logic in handler, pass-through abstraction, configuration leak.
- Low confidence: shallow module, temporal coupling, redundant comments, duplicated business knowledge without direct matched constants/rules.

Findings should include safe metadata such as operation kind, logger/metric call kind, deployment reference type, owner source searched, runbook source searched, public surface count, pass-through ratio, and confidence evidence. Do not include secrets or raw source snippets.

## Profile behavior target

| Profile | Behavior |
| --- | --- |
| Startup | Warn only for clear mutable deployment refs, sensitive logs, and severe operability gaps. |
| Strict | Warn observability/design/delivery issues; block existing severe CI/security/supply-chain policy only. |
| Enterprise | Strict plus ownership, observability, rollout safety, provenance, runbooks, and service compatibility as hard or near-hard gates by threshold. |
| AI-safe | Strict plus unnecessary abstractions, duplicated knowledge, environment branching, weak operability metadata, and generated-code reviewability risks. |

## Acceptance criteria

- New config fields validate and round-trip in JSON/YAML.
- New rule metadata includes fix templates and explicit language coverage.
- Observability package can detect unstructured logs, error-without-context, sensitive log data, high-cardinality labels, critical path without instrumentation, log-and-ignore, and shallow health checks.
- Operations package can detect missing owners and runbooks for configured critical systems.
- Design extensions can detect at least infrastructure type leak, persistence model leak, domain logic in handler, pass-through abstraction, and excessive public surface.
- Delivery/CI extensions can detect mutable deployment refs, missing CI gates, missing rollback strategy, unsafe migration ordering, high-risk change without kill switch, missing post-deploy verification, missing provenance, and environment branching.
- Enterprise profile enables the intended checks without changing startup defaults aggressively.
- Existing JSON/SARIF/GitHub annotation/text summary compatibility is preserved.
- Targeted tests and `make test` pass before push/PR.

## Verification plan

Targeted during implementation:

```sh
go test ./internal/codeguard/config ./internal/codeguard/rules ./internal/codeguard/runner/checks
go test ./tests/codeguard ./tests/checks ./tests/cli -run 'Test.*(Observability|Operations|Delivery|Owner|Runbook|Provenance|PublicSurface|DomainLogic|InfrastructureLeak|Deployment|Profile|Metadata)'
go test ./tests/checks -run 'TestDesign|TestQuality|TestCI|TestSupplyChain|TestWriteReport'
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

- [ ] Rule IDs use existing prefix conventions where possible, especially `supply_chain.*`.
- [ ] Every built-in rule has a fix template.
- [ ] New config has defaults, validation, examples, and SDK aliases.
- [ ] Startup/strict defaults are not made unexpectedly noisy.
- [ ] Enterprise behavior is explicit and test-covered.
- [ ] Findings include actionable evidence and confidence.
- [ ] SARIF/GitHub annotations remain finding-only.
- [ ] Product docs describe implemented behavior, not planned behavior.
- [ ] `make test` passes.
- [ ] `make ci` passes or any skipped gate is explicitly documented.
