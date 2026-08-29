package cli

import (
	"testing"

	"github.com/devr-tools/codeguard/internal/codeguard/core"
)

func TestComparePolicyRejectsGrowthAndNewHighRiskEntries(t *testing.T) {
	comparison := core.BaselineFile{Entries: []core.BaselineEntry{
		{Fingerprint: "existing-security", RuleID: "security.secret"},
		{Fingerprint: "old-quality", RuleID: "quality.old"},
	}}
	current := core.BaselineFile{Entries: []core.BaselineEntry{
		{Fingerprint: "existing-security", RuleID: "security.secret"},
		{Fingerprint: "new-security", RuleID: "security.injection"},
		{Fingerprint: "new-quality", RuleID: "quality.new"},
	}}
	policy := core.BaselineGovernanceConfig{ForbidGrowth: true, ProhibitedNewRulePrefixes: []string{"security.", "defensive.", "error."}}

	result := ComparePolicy(current, comparison, policy)
	if len(result.Added) != 2 || len(result.Removed) != 1 {
		t.Fatalf("diff = added %#v removed %#v", result.Added, result.Removed)
	}
	if len(result.Violations) != 2 {
		t.Fatalf("violations = %#v, want growth and prohibited addition", result.Violations)
	}
	for _, violation := range result.Violations {
		if violation.Entry != nil && violation.Entry.Fingerprint == "existing-security" {
			t.Fatal("existing approved high-risk entry was rejected")
		}
	}
}

func TestComparePolicyHonorsMaximumWithoutComparison(t *testing.T) {
	current := core.BaselineFile{Entries: []core.BaselineEntry{{Fingerprint: "a"}, {Fingerprint: "b"}}}
	result := ComparePolicy(current, core.BaselineFile{}, core.BaselineGovernanceConfig{MaxEntries: 1})
	if len(result.Violations) != 1 || result.Violations[0].Kind != "max_entries" {
		t.Fatalf("violations = %#v", result.Violations)
	}
}
