package checks_test

import (
	"strings"
	"testing"
)

func TestSmellHistoryRulesUseCoChangeEvidence(t *testing.T) {
	dir := initMaintainabilityHistoryRepo(t)

	report := runMaintainabilityDeltaScan(t, qualityPrecisionConfig(dir))

	assertFindingRulePresent(t, report, "Code Quality", "smell.shotgun-surgery-history")
	assertFindingRulePresent(t, report, "Code Quality", "smell.divergent-change-history")
	assertFindingRulePresent(t, report, "Code Quality", "maintainability.change-amplification")

	shotgun := findFinding(t, report, "Code Quality", "smell.shotgun-surgery-history")
	if partners := shotgun.Metadata["top_partners"]; !strings.Contains(partners, "partner_a.go") || !strings.Contains(partners, "partner_b.go") {
		t.Fatalf("top_partners metadata = %q, want recurring partner evidence", partners)
	}

	amplifier := findFinding(t, report, "Code Quality", "maintainability.change-amplification")
	if !strings.Contains(amplifier.Message, "co-change partner") {
		t.Fatalf("change amplification message should include co-change evidence: %q", amplifier.Message)
	}
}

func TestChangeAmplificationDeterministicMetadataOrdering(t *testing.T) {
	dir := initMaintainabilityHistoryRepo(t)

	report := runMaintainabilityDeltaScan(t, qualityPrecisionConfig(dir))

	finding := findFinding(t, report, "Code Quality", "maintainability.change-amplification")
	if got := finding.Metadata["top_partners"]; !strings.HasPrefix(got, "partner_a.go:") {
		t.Fatalf("top_partners ordering = %q, want lexical tie-break after count ordering", got)
	}
}
