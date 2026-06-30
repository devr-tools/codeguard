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

//nolint:unparam // general-purpose test helper; section is part of its API shape
func assertFindingLevel(t *testing.T, report codeguard.Report, section string, ruleID string, level string) {
	t.Helper()
	for _, result := range report.Sections {
		if result.Name != section {
			continue
		}
		for _, finding := range result.Findings {
			if finding.RuleID == ruleID {
				if finding.Level != level {
					t.Fatalf("section %q rule %q level = %q, want %q", section, ruleID, finding.Level, level)
				}
				return
			}
		}
		t.Fatalf("section %q missing rule %q", section, ruleID)
	}
	t.Fatalf("section %q not found", section)
}
