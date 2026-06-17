# Competitive Roadmap

This roadmap turns the current `codeguard` feature set into a concrete next-build plan. It is ordered by product leverage, implementation fit with the existing codebase, and how much reusable infrastructure each step creates for the next one.

## Current position

The repo already has useful primitives to build on:

- diff-aware scan orchestration in [internal/codeguard/runner/runner.go](/Users/alex/Documents/GitHub/codeguard/internal/codeguard/runner/runner.go:1)
- patch materialization and patch validation in [internal/codeguard/runner/support/patch.go](/Users/alex/Documents/GitHub/codeguard/internal/codeguard/runner/support/patch.go:1)
- single-finding verified fix in [internal/codeguard/ai/fix/verify.go](/Users/alex/Documents/GitHub/codeguard/internal/codeguard/ai/fix/verify.go:1)
- prompt and agent-config governance in [internal/codeguard/checks/prompts/prompts.go](/Users/alex/Documents/GitHub/codeguard/internal/codeguard/checks/prompts/prompts.go:1)
- AI quality signals and `slop_score` artifacts in [internal/codeguard/checks/quality/quality_ai.go](/Users/alex/Documents/GitHub/codeguard/internal/codeguard/checks/quality/quality_ai.go:1)
- lightweight dependency graph support in [internal/codeguard/checks/support/dependency_graph.go](/Users/alex/Documents/GitHub/codeguard/internal/codeguard/checks/support/dependency_graph.go:1) and [internal/codeguard/checks/support/gomod.go](/Users/alex/Documents/GitHub/codeguard/internal/codeguard/checks/support/gomod.go:1)
- language-specific taint engines in `internal/codeguard/checks/security/`

The gap is not raw capability; it is packaging these primitives into broader product workflows.

## Execution principles

- Prefer adding new packages over overloading existing `quality` and `security` families with unrelated concerns.
- Land parser and artifact infrastructure before policy logic.
- Expose every major capability through CLI, SDK, and MCP-facing surfaces where practical.
- Keep diff-mode behavior first-class. Most competitive value comes from PR-time analysis, not only full-repo scans.
- Ship findings plus machine-readable artifacts so downstream agents can reason over rankings, fix queues, and provenance.

## 1. Real `supply_chain` check family

This should be the next major family, not an extension of `quality` or `security`. It has enough surface area to justify its own config, rule catalog entries, docs, and tests.

### Scope

- dependency graphing across manifest formats
- SBOM output
- license policy enforcement
- lockfile drift detection
- unpinned dependency detection
- manifest parsing for `go.mod`, `package.json`, `requirements*.txt`, `pyproject.toml`, and `Cargo.toml`

### Suggested shape

- Add `checks.supply_chain` and `checks.supply_chain_rules` to [internal/codeguard/core/config_types.go](/Users/alex/Documents/GitHub/codeguard/internal/codeguard/core/config_types.go:1) and [internal/codeguard/core/config_rule_types.go](/Users/alex/Documents/GitHub/codeguard/internal/codeguard/core/config_rule_types.go:1).
- Add `internal/codeguard/checks/supplychain/` for rule execution.
- Add manifest parsers under `internal/codeguard/checks/support/` so they can be reused by risk scoring and fix planning later.
- Add SBOM and dependency graph artifacts alongside the existing artifact model in `internal/codeguard/core/report_artifact_types.go`.

### Milestones

1. Manifest substrate
   - Parse the five target manifest families into one normalized dependency model.
   - Preserve package name, version/range, source file, lockfile linkage, dev/runtime scope, and whether the dependency is pinned.
2. Policy rules
   - Add rules for unpinned dependencies, missing lockfiles, lockfile drift, and denied licenses.
   - Start with deterministic local policy. Defer network-backed advisories unless a stable offline cache story exists.
3. Graph and SBOM artifacts
   - Emit a machine-readable dependency graph artifact.
   - Emit SPDX or CycloneDX JSON as a report artifact and optional CLI output target.
4. PR ergonomics
   - Diff-aware findings should only fail changed manifests and newly introduced violations.

### Acceptance criteria

- `codeguard scan` can detect at least one dependency policy issue in each supported ecosystem.
- report artifacts include a normalized dependency graph and SBOM payload.
- lockfile drift works for npm, pip, Cargo, and Go modules where a lock concept exists.
- docs and rule catalog explain exactly which ecosystems and lockfile semantics are supported.

### Agent task split

- Agent A: config and rule-catalog scaffolding
- Agent B: manifest parsers plus normalized dependency model
- Agent C: policy rules for pinning, lockfile drift, and license allow/deny lists
- Agent D: SBOM and dependency graph artifact emission
- Agent E: black-box tests in `tests/checks/` and CLI/report coverage

## 2. Preventive secret protection

Current prompt governance detects risky secret usage, but the competitive step is prevention before a bad patch lands.

### Scope

- patch-time secret rejection
- custom secret patterns
- secret-type classification
- optional validity-check adapters

### Suggested shape

- Keep prompt-oriented governance in `prompts`, but move general secret prevention into a shared secret engine under `internal/codeguard/checks/support/`.
- Run the same detector during full scans and `validate-patch` / `RunPatch`.
- Add a structured secret classification result so agents do not only receive a generic "hardcoded secret" finding.

### Milestones

1. Shared secret matcher
   - Replace the current regex-only secret checks with a reusable matcher library.
   - Support built-in detectors for API keys, bearer tokens, passwords, private keys, webhook URLs, and cloud credentials.
2. Patch-time blocking
   - Reject diffs that introduce classified secrets, not just files that already contain them.
   - Preserve changed-line scope in findings so patch validation stays precise.
3. Custom patterns and config
   - Add config-driven custom secret detectors with rule ID prefixing and severity selection.
4. Optional validity adapters
   - Add an adapter interface for external secret validation commands.
   - Keep adapters opt-in and fail-closed only when explicitly configured.

### Acceptance criteria

- `codeguard validate-patch` fails when a patch introduces a known secret pattern.
- findings include secret type metadata that distinguishes hardcoded token, private key, password, and interpolation exposure cases.
- custom patterns are configurable without code changes.

### Agent task split

- Agent A: shared matcher and classification types
- Agent B: patch-validation integration and diff-aware tests
- Agent C: config surface and validation
- Agent D: optional adapter execution model and report plumbing

## 3. Batch verified fix

The current verified-fix flow is a good primitive, but it is too narrow. The product should be able to remediate a safe subset of findings in one run.

### Scope

- select top safe findings in a PR or repo slice
- generate multiple candidate patches
- verify them in sequence or in isolated batches
- stop on conflicts or verification regressions

### Suggested shape

- Keep single-finding verification intact.
- Add a planner layer above [internal/codeguard/ai/fix/verify.go](/Users/alex/Documents/GitHub/codeguard/internal/codeguard/ai/fix/verify.go:1), rather than making `Verify` itself stateful.
- Represent remediation as a queue of finding-targeted operations plus merge/conflict metadata.

### Milestones

1. Fix planning
   - Rank findings by safety and fixability.
   - Exclude overlapping files, conflicting hunks, and rules that are not auto-fix-safe.
2. Batch execution
   - Materialize a workspace, apply one candidate at a time, rerun targeted verification, and keep only passing patches.
3. UX surfaces
   - Add SDK and CLI entrypoints for "fix top findings" and "fix findings in diff scope".
   - Extend MCP tools after the SDK contract is stable.
4. Reporting
   - Return applied, skipped, conflicted, and failed-verification buckets.

### Acceptance criteria

- one command can safely fix multiple independent findings in a repo slice
- verification remains fail-closed
- outputs clearly explain why a finding was skipped

### Agent task split

- Agent A: finding ranking and safe-fix eligibility
- Agent B: patch queue executor and conflict detection
- Agent C: CLI and SDK surface
- Agent D: tests for partial success, rollback, and conflicting edits

## 4. Risk scoring and hotspot ranking

`slop_score` is useful but too narrow. The next step is a broader risk model that helps reviewers and agents prioritize what matters first.

### Scope

- file-level risk ranking
- PR-level hotspot ranking
- inputs from churn, severity, taint reachability, coverage delta, and AI provenance

### Suggested shape

- Emit risk as a dedicated artifact rather than overloading finding severity.
- Keep scoring explainable: every score should list the weighted signals that produced it.
- Reuse dependency, semantic, and provenance artifacts instead of recomputing signals ad hoc.

### Milestones

1. Data model
   - Add artifact types for `file_risk` and `pr_hotspots`.
   - Define transparent factor weights in config with stable defaults.
2. Signal collection
   - Reuse changed files, coverage delta, taint findings, AI provenance, and future supply-chain signals.
3. Ranking and presentation
   - Sort files and PR slices by composite score.
   - Surface "why this ranked high" in text, JSON, and GitHub comment output.

### Acceptance criteria

- scans produce a deterministic file and PR ranking artifact
- each rank entry shows contributing signals and weights
- diff-mode reports identify the top risky changed files even when finding counts are low

### Agent task split

- Agent A: artifact schema and config
- Agent B: score computation pipeline
- Agent C: reporter integration for text, JSON, and GitHub comment modes
- Agent D: tests for explainability and stable ordering

## 5. Framework-aware semantic models

This is the deepest technical investment and should come after the supply-chain and secret-prevention foundations are in place.

### Scope

- framework-specific sources, sinks, and sanitizers for Express and Next.js
- framework-specific sources, sinks, and sanitizers for Django and FastAPI
- common Go HTTP and database stack models
- reusable model and query packs

### Suggested shape

- Keep language parser and taint engines where they are.
- Add framework model packs as data-driven overlays where possible, not hardcoded one-offs inside each scanner.
- Separate "semantic facts extraction" from "framework rule packs" so new frameworks do not require parser rewrites.

### Milestones

1. Model-pack format
   - Define framework source, sink, sanitizer, and router-binding descriptors.
2. Runtime integration
   - Load model packs into the Go, Python, and TypeScript taint engines.
3. Query packs
   - Add reusable security and quality queries that consume framework facts.
4. Regression suite
   - Add realistic framework fixtures in `tests/checks/` instead of only unit-scale snippets.

### Acceptance criteria

- framework-aware taint results catch issues that the current generic models miss
- false positives do not regress sharply on existing tests
- model packs can be extended without changing parser internals

### Agent task split

- Agent A: model-pack schema and loader
- Agent B: Express and Next.js bindings
- Agent C: Django and FastAPI bindings
- Agent D: Go HTTP and database sink coverage
- Agent E: fixture-heavy regression tests

## Recommended sequencing

1. `supply_chain` family
2. preventive secret protection
3. batch verified fix
4. risk scoring and hotspot ranking
5. framework-aware semantic models

This order matters:

- `supply_chain` creates a normalized dependency model that risk scoring can consume later.
- preventive secret protection strengthens patch-time governance before batch auto-remediation becomes broader.
- batch verified fix becomes more valuable once safer finding selection and richer risk metadata exist.
- framework-aware models are high leverage, but they are also the most open-ended and should build on the stronger artifact and ranking story.

## First implementation batch

If work starts immediately, the best first batch is:

1. add config, runner, and rule scaffolding for `supply_chain`
2. implement normalized manifest parsing for `go.mod`, `package.json`, `requirements*.txt`, `pyproject.toml`, and `Cargo.toml`
3. add patch-time secret rejection using a shared secret classifier
4. define the batch verified-fix planner interface without changing current single-finding verification behavior

That batch produces reusable infrastructure and avoids overcommitting to scoring formulas or framework model details too early.
