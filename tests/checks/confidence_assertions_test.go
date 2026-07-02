package checks_test

import (
	"testing"

	"github.com/devr-tools/codeguard/pkg/codeguard"
)

// findFinding returns the first finding for a rule inside a named section.
func findFinding(t *testing.T, report codeguard.Report, section string, ruleID string) codeguard.Finding {
	t.Helper()
	for _, result := range report.Sections {
		if result.Name != section {
			continue
		}
		for _, finding := range result.Findings {
			if finding.RuleID == ruleID {
				return finding
			}
		}
		t.Fatalf("section %q missing rule %q", section, ruleID)
	}
	t.Fatalf("section %q not found", section)
	return codeguard.Finding{}
}

func assertFindingConfidence(t *testing.T, report codeguard.Report, section string, ruleID string, confidence string) {
	t.Helper()
	finding := findFinding(t, report, section, ruleID)
	if finding.Confidence != confidence {
		t.Fatalf("section %q rule %q confidence = %q, want %q", section, ruleID, finding.Confidence, confidence)
	}
}
