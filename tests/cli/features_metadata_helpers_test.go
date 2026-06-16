package cli_test

import (
	"reflect"
	"testing"

	"github.com/devr-tools/codeguard/pkg/codeguard"
)

func requireRuleMetadata(t *testing.T, ruleID string) codeguard.RuleMetadata {
	t.Helper()
	rule, ok := codeguard.ExplainRule(ruleID)
	if !ok {
		t.Fatalf("expected %s metadata", ruleID)
	}
	return rule
}

func assertExecutionModel(t *testing.T, rule codeguard.RuleMetadata, want codeguard.RuleExecutionModel) {
	t.Helper()
	if rule.ExecutionModel != want {
		t.Fatalf("%s execution model = %q, want %q", rule.ID, rule.ExecutionModel, want)
	}
}

func assertLanguageCoverage(t *testing.T, rule codeguard.RuleMetadata, mode codeguard.RuleLanguageCoverageMode, languages ...codeguard.RuleLanguage) {
	t.Helper()
	if rule.LanguageCoverage.Mode != mode {
		t.Fatalf("%s language coverage mode = %q, want %q", rule.ID, rule.LanguageCoverage.Mode, mode)
	}
	if !reflect.DeepEqual(rule.LanguageCoverage.Languages, languages) {
		t.Fatalf("%s language coverage languages = %#v, want %#v", rule.ID, rule.LanguageCoverage.Languages, languages)
	}
}
