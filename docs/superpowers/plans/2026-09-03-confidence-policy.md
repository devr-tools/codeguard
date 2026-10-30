# Confidence As Policy Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Turn the existing but unused finding confidence into a policy input — a configurable minimum threshold with full accounting, optional low-confidence demotion, and confidence-aware ordering — without changing any default behavior.

**Architecture:** Two new fields on `core.CheckConfig`, validated in `config/validate.go`. Filtering and demotion happen in `FinalizeSectionWithDiagnostics`, before suppression matching and before section status, which keeps the change out of the per-file cache path entirely. Removed findings are tallied by `RuleStatsCollector` under a new reason.

**Tech Stack:** Go, existing config load/validate, `runner/support` finalize gate, `RuleStatsCollector`, report writers.

**Spec:** `docs/superpowers/specs/2026-09-03-confidence-policy-design.md`

**Branch:** `feat/confidence-policy`

## Global Constraints

- Default configuration must produce identical findings, counts, statuses, exit codes, and JSON/SARIF bytes; the only permitted difference is within-group text ordering (Task 5).
- Confidence and reported level must not affect exact, context, or content fingerprints.
- Demotion never promotes and never touches medium or high.
- Threshold and demotion changes must not invalidate the per-file findings cache, so neither field may enter the named per-family fingerprints in `SectionConfigHashes`.
- Filtered findings must be counted, never silently dropped.
- No rule's assigned confidence changes in this branch.

**Local toolchain:** `export GOROOT=/opt/homebrew/opt/go/libexec && export PATH=$GOROOT/bin:$PATH`, or use `make` targets. Use `GOCACHE=/private/tmp/codeguard-go-cache`.

---

### Task 1: Config surface and validation

**Files:**
- Modify: `internal/codeguard/core/config_types.go`
- Modify: `internal/codeguard/config/validate.go`
- Modify: `internal/codeguard/config/defaults.go`
- Test: `tests/config/confidence_policy_test.go`

**Interfaces:**
- Produces: `core.ConfidencePolicyConfig{Default string, Sections map[string]string}` on `CheckConfig` as `min_confidence`, plus `ConfidenceDemotion bool` as `confidence_demotion`.
- Produces: `func (c ConfidencePolicyConfig) Threshold(sectionID string) string`.

- [ ] **Step 1: Write failing config tests**

Assert an absent block yields the permissive default; assert YAML and JSON both load global and per-section values; assert `Threshold` prefers the section entry then the default; assert validation rejects an unknown level and an unknown section key with a message shaped like the existing `parsers.treesitter` error.

```go
if err := config.Validate(cfgWithConfidence("sideways")); err == nil {
	t.Fatal("expected an error for an unknown confidence level")
}
```

- [ ] **Step 2: Run and verify red**

Run: `GOCACHE=/private/tmp/codeguard-go-cache go test ./tests/config -run 'Confidence' -count=1`

- [ ] **Step 3: Implement the config fields, defaults, and validation**

Place both fields directly on `CheckConfig`. Reuse `core.NormalizedConfidence` for level parsing and the existing section identifier list for key validation.

- [ ] **Step 4: Verify green**

Run: `GOCACHE=/private/tmp/codeguard-go-cache go test ./tests/config -count=1`

---

### Task 2: Confidence ranking helper

**Files:**
- Modify: `internal/codeguard/core/finding_confidence.go`
- Test: `tests/core/finding_confidence_test.go`

**Interfaces:**
- Produces: `core.ConfidenceRank(level string) int` and `core.MeetsConfidence(finding, threshold string) bool`, with empty confidence treated as medium per the documented mapping.

- [ ] **Step 1: Write failing ranking tests**

Cover high > medium > low, empty treated as medium, unknown input treated as medium, and threshold comparison at each of the three thresholds including the permissive one admitting everything.

- [ ] **Step 2: Run and verify red**

Run: `GOCACHE=/private/tmp/codeguard-go-cache go test ./tests/core -run 'Confidence' -count=1`

- [ ] **Step 3: Implement the helpers**

- [ ] **Step 4: Verify green**

Run: `GOCACHE=/private/tmp/codeguard-go-cache go test ./tests/core -count=1`

---

### Task 3: Filter gate with accounting

**Files:**
- Modify: `internal/codeguard/runner/support/findings_section.go`
- Modify: `internal/codeguard/runner/support/rule_stats.go`
- Modify: `internal/codeguard/runner/support/suppressions.go` (reason constant)
- Modify: `internal/codeguard/core/report_artifact_rule_stats_types.go`
- Test: `tests/support/confidence_filter_test.go`
- Test: `tests/support/rule_stats_test.go`

**Interfaces:**
- Adds: `SuppressionReasonConfidence`, a `confidence` tally on `ruleTally`, and the filter stage in `FinalizeSectionWithDiagnostics` before `MatchSuppression`.

- [ ] **Step 1: Write failing filter and accounting tests**

Assert a below-threshold finding is absent from `section.Findings`, counted in the section's suppressed accounting, and tallied under the confidence reason; assert a per-section threshold applies only to that section; assert the removed count plus the emitted count equals the pre-filter count; assert `IncludeSuppressed` surfaces the finding with its confidence reason; assert section status ignores filtered findings.

```go
if emitted+filtered != total {
	t.Fatalf("confidence filter lost findings: %d emitted + %d filtered != %d total", emitted, filtered, total)
}
```

- [ ] **Step 2: Run and verify red**

Run: `GOCACHE=/private/tmp/codeguard-go-cache go test ./tests/support -run 'Confidence|RuleStats' -count=1`

- [ ] **Step 3: Implement the filter stage**

Insert it after diff scoping and before waiver audit and suppression matching. Keep the reason distinct from the three suppression mechanisms in both the collector and the report artifact.

- [ ] **Step 4: Verify green**

Run: `GOCACHE=/private/tmp/codeguard-go-cache go test ./tests/support -count=1 -race`

---

### Task 4: Demotion

**Files:**
- Modify: `internal/codeguard/runner/support/findings_section.go`
- Test: `tests/support/confidence_demotion_test.go`
- Test: `tests/support/context_fingerprint_test.go`

- [ ] **Step 1: Write failing demotion and identity tests**

Assert a low-confidence `fail` reports as `warn` with demotion on and drives section status to warn; assert medium and high are untouched; assert `warn` is never promoted; assert `Level` and `Severity` stay consistent; assert all three fingerprints are identical with demotion on and off.

```go
if demoted.Fingerprint != undemoted.Fingerprint || demoted.ContextFingerprint != undemoted.ContextFingerprint || demoted.ContentFingerprint != undemoted.ContentFingerprint {
	t.Fatal("confidence demotion changed finding identity")
}
```

- [ ] **Step 2: Run and verify red**

Run: `GOCACHE=/private/tmp/codeguard-go-cache go test ./tests/support -run 'Demotion|Fingerprint' -count=1`

- [ ] **Step 3: Implement demotion before the status switch**

- [ ] **Step 4: Verify green**

Run: `GOCACHE=/private/tmp/codeguard-go-cache go test ./tests/support -count=1`

---

### Task 5: Ordering and report surfaces

**Files:**
- Modify: `internal/codeguard/report/text_helpers.go` (`groupTextFindings` only)
- Test: `tests/report/ordering_test.go`
- Test: `tests/report/confidence_report_test.go`

**Interfaces:**
- Changes: within-group ordering in the text renderer only. Section finding order is untouched, so JSON and SARIF serialization is unaffected.

- [ ] **Step 1: Write failing ordering and report tests**

Assert a rule group mixing confidences lists high before medium before low; assert the sort is stable so equal-confidence findings keep scan order; assert group identity and group order are unchanged; assert JSON and SARIF bytes are unchanged for the default config; assert the text report states how many findings the confidence threshold removed when any were.

- [ ] **Step 2: Run and verify red**

Run: `GOCACHE=/private/tmp/codeguard-go-cache go test ./tests/report -run 'Ordering|Confidence' -count=1`

- [ ] **Step 3: Implement the within-group stable sort and the threshold summary line**

Sort inside `groupTextFindings` with `sort.SliceStable` on `core.ConfidenceRank`. Do not sort `section.Findings`.

- [ ] **Step 4: Verify green**

Run: `GOCACHE=/private/tmp/codeguard-go-cache go test ./tests/report -count=1`

---

### Task 6: Cache invariance and default-behavior regression

**Files:**
- Test: `tests/support/cache_confidence_test.go`
- Test: `tests/corpus/` (default-config regression)

- [ ] **Step 1: Write the cache invariance test**

Scan, then rescan with a different `min_confidence` and with demotion toggled: assert per-file cache hits (no recomputation) and that the rendered findings differ as configured. Assert neither field appears in any named per-family fingerprint from `SectionConfigHashes`.

- [ ] **Step 2: Write the default-behavior regression**

With no confidence configuration, assert the corpus suite's findings, counts, statuses, and exit code are identical to the pre-change baseline.

- [ ] **Step 3: Run both and verify**

Run: `env -u GOROOT GOCACHE=/private/tmp/codeguard-go-cache go test ./tests/support ./tests/corpus -count=1`

---

### Task 7: Docs and full gate

**Files:**
- Modify: `docs/checks.md` (config reference for both fields)
- Modify: `docs/features.md` (baseline governance / policy surface)
- Modify: `README.md` (one line in the capability paragraph)
- Modify: `.claude/knowledge/architecture-boundaries.md`

- [ ] **Step 1: Document the config with a worked YAML example**

Follow the existing `docs/features.md` config-example style, including a per-section override and a note that thresholds re-render rather than re-run a scan.

- [ ] **Step 2: Record the self-scan deltas**

Run the self-scan at each threshold and record the finding counts and the confidence-filtered tallies, so the accounting reconciliation is documented, not just tested.

- [ ] **Step 3: Full gate**

Run: `make fmt && make lint && make test && make codeguard-ci`, then strict lint: `env -u GOROOT GOCACHE=/private/tmp/codeguard-go-cache GOLANGCI_LINT_CACHE=/private/tmp/golangci-lint-cache /Users/alex/.go/bin/golangci-lint run` (0 issues), then `go test ./... -race`.

- [ ] **Step 4: Capture the identity rule in knowledge**

Note that confidence and reported level are excluded from finding identity, alongside prose and metadata, so future policy knobs inherit the constraint.
