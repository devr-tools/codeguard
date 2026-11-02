package codeguard_test

import (
	"context"
	"strings"
	"testing"

	"github.com/devr-tools/codeguard/pkg/codeguard"
)

// disabledRulesFixture returns the triage fixture config with AI off and the
// rule id of its first finding.
func disabledRulesFixture(t *testing.T) (codeguard.Config, string) {
	t.Helper()
	t.Setenv("CODEGUARD_AI_TRIAGE_PROVIDER", "")
	cfg := triageFixtureConfig(t, t.TempDir())
	report, err := codeguard.Run(context.Background(), cfg)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	for _, section := range report.Sections {
		for _, finding := range section.Findings {
			return cfg, finding.RuleID
		}
	}
	t.Fatal("fixture produced no findings")
	return cfg, ""
}

func countRuleFindings(report codeguard.Report, ruleID string) int {
	count := 0
	for _, section := range report.Sections {
		for _, finding := range section.Findings {
			if finding.RuleID == ruleID {
				count++
			}
		}
	}
	return count
}

func ruleStatsFor(report codeguard.Report, ruleID string) (codeguard.RuleStatsEntry, bool) {
	for _, artifact := range report.Artifacts {
		if artifact.RuleStats == nil {
			continue
		}
		for _, entry := range artifact.RuleStats.Rules {
			if entry.RuleID == ruleID {
				return entry, true
			}
		}
	}
	return codeguard.RuleStatsEntry{}, false
}

func TestDisabledRulesDropFindingsWithoutCountingSuppression(t *testing.T) {
	cfg, ruleID := disabledRulesFixture(t)
	cfg.Checks.DisabledRules = []string{ruleID}
	report, err := codeguard.Run(context.Background(), cfg)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if got := countRuleFindings(report, ruleID); got != 0 {
		t.Fatalf("%s still reported %d findings", ruleID, got)
	}
	if entry, ok := ruleStatsFor(report, ruleID); ok {
		t.Fatalf("disabled rule must not appear in rule stats, got %#v", entry)
	}
	for _, section := range report.Sections {
		if section.SuppressedCount != 0 {
			t.Fatalf("section %s counted %d suppressed findings", section.Name, section.SuppressedCount)
		}
	}
}

func TestWaiverForSameRuleCountsAsSuppressed(t *testing.T) {
	cfg, ruleID := disabledRulesFixture(t)
	cfg.Waivers = []codeguard.WaiverConfig{{Rule: ruleID, Path: "**", Reason: "contrast"}}
	report, err := codeguard.Run(context.Background(), cfg)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	entry, ok := ruleStatsFor(report, ruleID)
	if !ok || entry.WaiverSuppressed == 0 || entry.SuppressionRatio == 0 {
		t.Fatalf("waiver should count as suppression, got %#v (found=%v)", entry, ok)
	}
}

func TestDisabledRulesPrefixDisablesFamily(t *testing.T) {
	cfg, ruleID := disabledRulesFixture(t)
	family := ruleID[:strings.LastIndex(ruleID, ".")]
	cfg.Checks.DisabledRules = []string{family + ".*"}
	report, err := codeguard.Run(context.Background(), cfg)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if got := countRuleFindings(report, ruleID); got != 0 {
		t.Fatalf("%s.* left %d %s findings", family, got, ruleID)
	}
}

func TestDisabledRulesValidation(t *testing.T) {
	tests := []struct {
		name    string
		entries []string
		wantErr string
	}{
		{name: "exact id", entries: []string{"observability.unstructured-log"}},
		{name: "family prefix", entries: []string{"security.taint.*"}},
		{name: "retired alias", entries: []string{"quality.mixed-abstraction-levels"}},
		{name: "typo", entries: []string{"observability.unstructured-logs"}, wantErr: `unknown rule "observability.unstructured-logs"; did you mean "observability.unstructured-log"`},
		{name: "prefix without rules", entries: []string{"nosuchfamily.*"}, wantErr: `entry "nosuchfamily.*" matches no rule`},
		{name: "bare wildcard", entries: []string{".*"}, wantErr: "matches no rule"},
		{name: "retired performance id", entries: []string{"quality.n-plus-one-query"}, wantErr: `it is now "performance.n-plus-one-query"`},
		{name: "blank", entries: []string{" "}, wantErr: "checks.disabled_rules[0] must not be blank"},
		{name: "duplicate", entries: []string{"observability.unstructured-log", "observability.unstructured-log"}, wantErr: "duplicate entry"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := codeguard.ExampleConfig()
			cfg.Checks.DisabledRules = tt.entries
			err := codeguard.ValidateConfig(cfg)
			if tt.wantErr == "" {
				if err != nil {
					t.Fatalf("ValidateConfig: %v", err)
				}
				return
			}
			if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
				t.Fatalf("ValidateConfig error = %v, want %q", err, tt.wantErr)
			}
		})
	}
}

func TestDisabledRulesAcceptRulePackIDs(t *testing.T) {
	cfg := codeguard.ExampleConfig()
	cfg.RulePacks = []codeguard.RulePackConfig{{Name: "pack", Rules: []codeguard.CustomRuleConfig{{ID: "custom.todo", Title: "TODO", Message: "todo", ContentRegex: "TODO"}}}}
	cfg.Checks.DisabledRules = []string{"custom.todo"}
	if err := codeguard.ValidateConfig(cfg); err != nil {
		t.Fatalf("ValidateConfig: %v", err)
	}
}

func TestRuleEnabledHonorsDisabledRules(t *testing.T) {
	cfg := codeguard.ExampleConfig()
	cfg.Checks.DisabledRules = []string{"quality.mixed-abstraction-levels"}
	rule, ok := codeguard.ExplainRule("function.mixed-abstraction-level")
	if !ok {
		t.Fatal("rule missing from catalog")
	}
	if codeguard.RuleEnabled(cfg, rule) {
		t.Fatal("rule disabled through its retired alias still reports enabled")
	}
	if entry, disabled := codeguard.RuleDisabledBy(cfg, rule.ID); !disabled || entry != "quality.mixed-abstraction-levels" {
		t.Fatalf("RuleDisabledBy = %q, %v", entry, disabled)
	}
}
