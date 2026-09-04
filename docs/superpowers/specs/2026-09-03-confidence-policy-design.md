# Confidence As Policy Design

## Scope

Findings already carry a confidence level that nothing consumes. This change makes confidence a policy input: a configurable minimum threshold that filters findings with full accounting, an optional level demotion for low-confidence hits, and confidence as a deterministic ordering key in reports.

It does not change which confidence any rule assigns, does not add or remove rules, and does not alter finding identity. Defaults reproduce today's behavior exactly.

## Problem

`core.ConfidenceHigh`, `ConfidenceMedium`, and `ConfidenceLow` exist, `NormalizedConfidence` maps unspecified to medium, and 177 call sites set the field. `NewFinding` stores it, `report/sarif_builders.go` emits it as a SARIF result property, and `report/text_helpers.go` appends "(low confidence)" to the text renderer's `why` line. That is the whole of it: no filtering, no ordering, no effect on section status or exit code.

The gap matters most where accuracy work has already landed. Tree-sitter-derived findings are tagged `high` and their regex fallbacks are not, and structured taint analysis is tagged `high` while regex line scans are effectively medium — but a consumer cannot ask for only the trustworthy half. A team that wants coverage without gate noise has to disable rules wholesale, which loses the signal entirely, or baseline the noise, which hides it.

## Configuration

One struct on `CheckConfig`, holding the global default and per-section overrides:

```yaml
checks:
  min_confidence:
    default: low      # low | medium | high; low is today's behavior
    sections:
      security: high
  confidence_demotion: false
```

Section overrides sit under their own `sections` key rather than beside
`default`, so the block decodes as a plain struct with no reserved-key
handling in the codec.

`default: low` admits everything, which is why the default configuration is behavior-preserving. Section keys are the ids that appear in scan output and in `checks.disabled` — note that supply chain is `supply_chain` there, while the runner's internal registry id is `supply-chain`. Validation rejects unknown levels and unknown section keys with the same message style as `parsers.treesitter` validation in `internal/codeguard/config/validate.go`.

Both fields live directly on `CheckConfig` rather than inside any rule struct. `SectionConfigHashes` additionally strips them before fingerprinting, so they stay out of every family including the conservative all-checks fallback, which is required for the caching property below.

## Filter Placement

Filtering happens in `FinalizeSectionWithDiagnostics` (`internal/codeguard/runner/support/findings_section.go`), the single gate every section's findings already pass through, immediately before suppression matching and before section status is computed.

That placement gives two properties:

- Findings are filtered after the per-file cache (`cachedFileFindings`), so changing a threshold never invalidates cached findings. A threshold change re-renders a scan; it does not re-run one.
- Diff-mode scoping, waiver auditing, suppression precedence, and status computation keep their existing order and semantics.

## Accounting

A filtered finding is never silently dropped. It is counted through the existing `RuleStatsCollector` under a new reason alongside baseline, waiver, and inline suppression, and it is included in the section's suppressed accounting so the report can state how many findings the threshold removed and for which rules.

Confidence filtering is reported as its own mechanism, not folded into suppression counts, because the two answer different questions: a suppression says a team accepted a finding, a confidence filter says the tool was not sure enough to show it. `--include-suppressed` surfaces confidence-filtered findings with their reason, as it does for the other mechanisms.

## Demotion

With `confidence_demotion: true`, a `low`-confidence finding on a rule whose level is `fail` reports as `warn`. Demotion never promotes, never applies to `medium` or `high`, and applies before section status is computed so a demoted finding cannot fail a gate. It is off by default.

Demotion and thresholds compose in one direction: the threshold decides whether a finding is shown, then demotion decides how loudly. A finding removed by the threshold is not demoted, it is gone.

## Ordering

Findings today carry no explicit sort: they reach the report in file-scan order, and the text renderer groups them by rule title through `groupTextFindings`. Sorting at the section level would therefore reorder JSON and SARIF output as a side effect, which the byte-identical default rules out.

Confidence ordering is instead applied as a stable sort by confidence rank *within* each existing text finding group. Group identity and group order are unchanged, JSON and SARIF field order is untouched, and the only visible difference is that a rule group mixing confidences lists its high-confidence hits first. The sort is stable, so equal-confidence findings keep their scan order and output stays reproducible.

## Identity

Confidence must not affect the exact, context, or content fingerprint. Neither may demotion: a demoted finding keeps the identity it would have had at its catalog level, so an existing baseline entry keeps matching after `confidence_demotion` is switched on. This follows the identity rule established for structural findings — prose, metadata, and now confidence and reported level are excluded from identity.

## Non-Goals

Mapping confidence onto SARIF `rank` is not part of this change; confidence stays a result property so external consumers see no schema churn. Auditing the 177 emission sites to replace the empty-means-medium default with explicit values is also out of scope: the empty mapping is documented and the accuracy-relevant contrast (tree path `high` versus regex fallback default) already holds.

## Acceptance

- With no `min_confidence` configured, findings, counts, section statuses, exit codes, and JSON/SARIF bytes are identical to the current implementation across the corpus suite and the self-scan. Text output may differ only in within-group ordering where a rule group mixes confidences, and that difference is pinned by a test.
- `min_confidence.default: high` on the self-scan reduces the finding count, and every removed finding is accounted for in rule stats under the confidence reason, with the two totals reconciling exactly.
- A per-section override applies to that section only.
- `confidence_demotion: true` turns a low-confidence `fail` into a `warn`, changes section status accordingly, and leaves all three fingerprints unchanged.
- Changing a threshold or the demotion flag produces cache hits, not recomputation, proven by a cache test.
- Ordering tests pin the confidence tiebreak.
