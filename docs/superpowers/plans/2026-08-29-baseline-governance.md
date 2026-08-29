# Baseline Governance Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add safe baseline auditing, pruning, suppression-level JSON reporting, governance policy enforcement, ownership/risk summaries, and deterministic false-positive review samples.

**Architecture:** The runner will optionally retain suppressed findings with structured suppression metadata while preserving existing report defaults. Focused baseline-governance files in the existing `internal/cli` package will compare an existing baseline with a suppression-free full scan, classify entries through exact, context, or content fingerprints, report collisions without changing v1.7.3 suppression semantics, prune only stale entries, and compare baselines for policy enforcement without adding a new dependency on the central core package. The CLI will retain the existing `codeguard baseline` creation command while adding `audit`, `prune`, and `policy` subcommands.

**Tech Stack:** Go 1.23+, standard-library JSON/file APIs, existing CodeGuard runner/config/report packages.

**Spec:** User-approved baseline-governance prompt in the 2026-08-29 conversation, corrected to preserve many-to-many context/content fingerprint semantics.

## Global Constraints

- Existing baseline files and scan behavior remain compatible; governance is opt-in.
- Audit and prune run full scans with the configured baseline disabled and never add current findings.
- An entry is active if any exact, context, or content fingerprint matches a current finding.
- Collisions are reported and preserved; matching is not forced one-to-one.
- `prune --check` never writes; `prune --write` atomically removes only stale entries and refuses invalid entries unless explicitly overridden.
- JSON, text, SARIF, GitHub, and CycloneDX defaults remain unchanged unless suppressed findings are explicitly requested.
- Governance output and deterministic samples use stable ordering.

---

### Task 1: Structured suppression reporting

**Files:**
- Modify: `internal/codeguard/core/report_types.go`
- Modify: `internal/codeguard/core/diff_types.go`
- Modify: `internal/codeguard/runner/support/context.go`
- Modify: `internal/codeguard/runner/support/suppressions.go`
- Modify: `internal/codeguard/runner/support/findings_section.go`
- Modify: `internal/codeguard/runner/runner.go`
- Modify: `internal/cli/scan_flags.go`
- Test: `tests/checks/suppressed_findings_test.go`
- Test: `tests/cli/scan_suppressed_test.go`

**Interfaces:**
- Produces: `Suppression{Kind, Match, BaselineFingerprint}`, `Report.SuppressedFindings`, and `ScanOptions.IncludeSuppressed`.
- Preserves: existing emitted `SectionResult.Findings`, summary counts, and default JSON shape.

- [ ] Write a runner test proving baseline exact/context/content, waiver, and inline suppressions are individually distinguishable and reconcile with summary/rule statistics.
- [ ] Run the focused test and verify it fails because suppressed records are absent.
- [ ] Add structured suppression matching and opt-in collection with no default-output change.
- [ ] Run the focused runner tests and verify they pass.
- [ ] Write and fail a CLI test for `scan -include-suppressed -format json`.
- [ ] Wire the flag through scan options and verify the CLI test passes.

### Task 2: Deterministic baseline audit and pruning engine

**Files:**
- Create: `internal/cli/baseline_audit.go`
- Create: `internal/cli/baseline_io.go`
- Test: `internal/cli/baseline_audit_test.go`
- Test: `internal/cli/baseline_io_test.go`

**Interfaces:**
- Consumes: `core.BaselineFile` and current `[]core.Finding`.
- Produces: `AuditResult` with active-exact/context/content, stale, invalid, duplicate, collision, rule, owner, risk, language, confidence, and deterministic sample data.
- Produces: `Prune(source, output string, audit AuditResult, allowInvalid bool) error` using atomic replacement.

- [ ] Write table-driven failing tests for exact/context/content activation, moved findings, duplicate snippets, collisions, invalid entries, stable ordering, ownership/risk grouping, and deterministic samples.
- [ ] Implement the smallest audit engine that passes those tests while preserving all collision-matched entries.
- [ ] Write failing filesystem tests for check-mode immutability, stale-only pruning, output candidates, malformed JSON, and failed atomic replacement preserving the source.
- [ ] Implement strict loading and atomic deterministic writing, then run all baseline package tests.

### Task 3: Governance configuration and policy comparison

**Files:**
- Modify: `internal/codeguard/core/config_types.go`
- Modify: `internal/codeguard/config/validate.go`
- Create: `internal/cli/baseline_policy.go`
- Test: `internal/cli/baseline_policy_test.go`
- Test: `internal/codeguard/config/baseline_governance_test.go`

**Interfaces:**
- Produces: opt-in `baseline.governance` fields for limits, stale checks, prohibited prefixes, ownership mappings, and sample limits.
- Produces: deterministic `ComparePolicy(current, comparison, governance)` with exact additions/removals and violations.

- [ ] Write failing tests for invalid limits/mappings/prefixes and valid omitted governance.
- [ ] Add config types and validation, then verify config tests pass.
- [ ] Write failing policy tests for growth, prohibited high-risk additions, allowed existing debt, and deterministic diffs.
- [ ] Implement policy comparison and verify baseline package tests pass.

### Task 4: Baseline audit, prune, and policy CLI

**Files:**
- Modify: `internal/cli/commands.go`
- Create: `internal/cli/baseline.go`
- Test: `tests/cli/baseline_governance_test.go`

**Interfaces:**
- Produces: `codeguard baseline audit`, `baseline prune --check|--write`, and `baseline policy -compare-baseline`.
- Preserves: legacy `codeguard baseline -config ... -output ...` creation.

- [ ] Write failing end-to-end CLI tests for audit text/JSON, prune check/write/output, invalid refusal/override, policy failures, and legacy creation.
- [ ] Add subcommand parsing and a shared full-scan-with-baseline-disabled path.
- [ ] Render stable text/JSON, enforce exit codes, and verify all CLI tests pass.

### Task 5: Documentation, compatibility, and version evidence

**Files:**
- Modify: `README.md`
- Modify: `docs/production.md`
- Modify: `docs/features.md`
- Test: `tests/cli/version_test.go`

**Interfaces:**
- Documents command distinctions, matching semantics, collision behavior, CI usage, review workflow, configuration, and exit codes.

- [ ] Add a failing version test covering linker/build-info precedence and JSON report agreement with `codeguard version`.
- [ ] Fix version plumbing only if the test demonstrates a defect.
- [ ] Document create/audit/check/write/policy workflows and compatible suppression semantics.
- [ ] Run formatting, focused tests, full tests, vet/lint targets, and review the complete diff against every specification requirement.
