package checks_test

import (
	"testing"

	"github.com/devr-tools/codeguard/pkg/codeguard"
)

func assertFindingRulePresent(t *testing.T, report codeguard.Report, section string, ruleID string) {
	t.Helper()
	for _, result := range report.Sections {
		if result.Name != section {
			continue
		}
		for _, finding := range result.Findings {
			if finding.RuleID == ruleID {
				return
			}
		}
		t.Fatalf("section %q missing rule %q", section, ruleID)
	}
	t.Fatalf("section %q not found", section)
}

func assertFindingRuleAbsent(t *testing.T, report codeguard.Report, section string, ruleID string) {
	t.Helper()
	for _, result := range report.Sections {
		if result.Name != section {
			continue
		}
		for _, finding := range result.Findings {
			if finding.RuleID == ruleID {
				t.Fatalf("section %q unexpectedly reported rule %q", section, ruleID)
			}
		}
		return
	}
	t.Fatalf("section %q not found", section)
}
