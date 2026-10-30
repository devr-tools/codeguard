package support_test

import (
	"testing"

	"github.com/devr-tools/codeguard/internal/codeguard/core"
	runnersupport "github.com/devr-tools/codeguard/internal/codeguard/runner/support"
)

const (
	confidenceRuleHigh = "security.demo-high"
	confidenceRuleLow  = "security.demo-low"
)

func confidenceContext(t testing.TB, policy core.ConfidencePolicyConfig) runnersupport.Context {
	t.Helper()
	cfg := core.Config{Name: "test"}
	cfg.Checks.MinConfidence = policy
	return runnersupport.Context{
		Cfg:       cfg,
		RuleStats: runnersupport.NewRuleStatsCollector(),
		RuleCatalog: map[string]core.RuleMetadata{
			confidenceRuleHigh: {ID: confidenceRuleHigh, Section: "Security", DefaultLevel: "fail"},
			confidenceRuleLow:  {ID: confidenceRuleLow, Section: "Security", DefaultLevel: "fail"},
		},
	}
}

func confidenceFinding(sc runnersupport.Context, ruleID string, confidence string) core.Finding {
	return runnersupport.NewFinding(sc, runnersupport.FindingInput{
		RuleID:     ruleID,
		Path:       "app/main.go",
		Line:       10,
		Column:     1,
		Message:    "demo finding",
		Confidence: confidence,
	})
}

func statsEntry(t testing.TB, sc runnersupport.Context, ruleID string) core.RuleStatsEntry {
	t.Helper()
	for _, entry := range sc.RuleStats.Snapshot() {
		if entry.RuleID == ruleID {
			return entry
		}
	}
	t.Fatalf("no rule stats entry for %q", ruleID)
	return core.RuleStatsEntry{}
}

// An unconfigured policy must admit every finding, which is what keeps the
// default scan byte-identical to the pre-policy behavior.
func TestConfidenceFilterDefaultAdmitsEverything(t *testing.T) {
	sc := confidenceContext(t, core.ConfidencePolicyConfig{})
	findings := []core.Finding{
		confidenceFinding(sc, confidenceRuleLow, core.ConfidenceLow),
		confidenceFinding(sc, confidenceRuleHigh, core.ConfidenceHigh),
	}
	section := runnersupport.FinalizeSection(sc, "security", "Security", findings)
	if len(section.Findings) != 2 {
		t.Fatalf("findings = %d, want 2", len(section.Findings))
	}
	if section.ConfidenceFilteredCount != 0 {
		t.Fatalf("ConfidenceFilteredCount = %d, want 0", section.ConfidenceFilteredCount)
	}
}

func TestConfidenceFilterDropsBelowThreshold(t *testing.T) {
	sc := confidenceContext(t, core.ConfidencePolicyConfig{Default: core.ConfidenceHigh})
	findings := []core.Finding{
		confidenceFinding(sc, confidenceRuleLow, core.ConfidenceLow),
		confidenceFinding(sc, confidenceRuleHigh, core.ConfidenceHigh),
	}
	section := runnersupport.FinalizeSection(sc, "security", "Security", findings)
	if len(section.Findings) != 1 || section.Findings[0].RuleID != confidenceRuleHigh {
		t.Fatalf("findings = %+v, want only %q", section.Findings, confidenceRuleHigh)
	}
	if section.ConfidenceFilteredCount != 1 {
		t.Fatalf("ConfidenceFilteredCount = %d, want 1", section.ConfidenceFilteredCount)
	}
	if section.SuppressedCount != 0 {
		t.Fatalf("SuppressedCount = %d, want 0: a confidence filter is not a suppression", section.SuppressedCount)
	}

	filtered := statsEntry(t, sc, confidenceRuleLow)
	if filtered.ConfidenceFiltered != 1 || filtered.Emitted != 0 {
		t.Fatalf("filtered rule stats = %+v, want ConfidenceFiltered 1 and Emitted 0", filtered)
	}
	if filtered.Suppressed() != 0 {
		t.Fatalf("Suppressed() = %d, want 0: confidence must not inflate the suppression ratio", filtered.Suppressed())
	}
	if emitted := statsEntry(t, sc, confidenceRuleHigh); emitted.Emitted != 1 {
		t.Fatalf("admitted rule stats = %+v, want Emitted 1", emitted)
	}
}

// Every finding must land in exactly one bucket, so a threshold can never make
// findings disappear without a trace.
func TestConfidenceFilterAccountsForEveryFinding(t *testing.T) {
	sc := confidenceContext(t, core.ConfidencePolicyConfig{Default: core.ConfidenceHigh})
	findings := []core.Finding{
		confidenceFinding(sc, confidenceRuleLow, core.ConfidenceLow),
		confidenceFinding(sc, confidenceRuleLow, core.ConfidenceMedium),
		confidenceFinding(sc, confidenceRuleHigh, core.ConfidenceHigh),
	}
	runnersupport.FinalizeSection(sc, "security", "Security", findings)

	total := 0
	for _, entry := range sc.RuleStats.Snapshot() {
		total += entry.Emitted + entry.ConfidenceFiltered + entry.Suppressed()
	}
	if total != len(findings) {
		t.Fatalf("accounted findings = %d, want %d", total, len(findings))
	}
}

func TestConfidenceFilterHonoursPerSectionOverride(t *testing.T) {
	policy := core.ConfidencePolicyConfig{Sections: map[string]string{"security": core.ConfidenceHigh}}
	sc := confidenceContext(t, policy)
	low := []core.Finding{confidenceFinding(sc, confidenceRuleLow, core.ConfidenceLow)}

	if section := runnersupport.FinalizeSection(sc, "security", "Security", low); len(section.Findings) != 0 {
		t.Fatalf("security findings = %d, want 0", len(section.Findings))
	}
	if section := runnersupport.FinalizeSection(sc, "quality", "Code Quality", low); len(section.Findings) != 1 {
		t.Fatalf("quality findings = %d, want 1: the override must not leak across sections", len(section.Findings))
	}
}

// A filtered finding must not fail or warn the gate it was filtered out of.
func TestConfidenceFilterLeavesSectionStatusClean(t *testing.T) {
	sc := confidenceContext(t, core.ConfidencePolicyConfig{Default: core.ConfidenceHigh})
	findings := []core.Finding{confidenceFinding(sc, confidenceRuleLow, core.ConfidenceLow)}
	section := runnersupport.FinalizeSection(sc, "security", "Security", findings)
	if section.Status != core.StatusPass {
		t.Fatalf("status = %q, want %q", section.Status, core.StatusPass)
	}
}

func TestConfidenceFilterSurfacesUnderIncludeSuppressed(t *testing.T) {
	sc := confidenceContext(t, core.ConfidencePolicyConfig{Default: core.ConfidenceHigh})
	sc.Opts = core.ScanOptions{IncludeSuppressed: true}
	sc.Suppressed = &runnersupport.SuppressedFindingCollector{}
	findings := []core.Finding{confidenceFinding(sc, confidenceRuleLow, core.ConfidenceLow)}
	runnersupport.FinalizeSection(sc, "security", "Security", findings)

	snapshot := sc.Suppressed.Snapshot()
	if len(snapshot) != 1 {
		t.Fatalf("suppressed snapshot = %d findings, want 1", len(snapshot))
	}
	if !snapshot[0].Suppressed || snapshot[0].SuppressionReason != runnersupport.SuppressionReasonConfidence {
		t.Fatalf("snapshot finding = %+v, want suppressed with reason %q", snapshot[0], runnersupport.SuppressionReasonConfidence)
	}
}

// The filter runs before suppression matching, so a finding removed by the
// threshold is never also attributed to a baseline entry.
func TestConfidenceFilterPrecedesSuppressionMatching(t *testing.T) {
	sc := confidenceContext(t, core.ConfidencePolicyConfig{Default: core.ConfidenceHigh})
	finding := confidenceFinding(sc, confidenceRuleLow, core.ConfidenceLow)
	sc.Baseline = map[string]core.BaselineEntry{
		finding.Fingerprint: {Fingerprint: finding.Fingerprint},
	}
	section := runnersupport.FinalizeSection(sc, "security", "Security", []core.Finding{finding})
	if section.SuppressedCount != 0 {
		t.Fatalf("SuppressedCount = %d, want 0", section.SuppressedCount)
	}
	if section.ConfidenceFilteredCount != 1 {
		t.Fatalf("ConfidenceFilteredCount = %d, want 1", section.ConfidenceFilteredCount)
	}
	entry := statsEntry(t, sc, confidenceRuleLow)
	if entry.BaselineSuppressed != 0 || entry.ConfidenceFiltered != 1 {
		t.Fatalf("rule stats = %+v, want only ConfidenceFiltered 1", entry)
	}
}
