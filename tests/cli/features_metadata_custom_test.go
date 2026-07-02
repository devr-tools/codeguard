package cli_test

import (
	"strings"
	"testing"

	"github.com/devr-tools/codeguard/pkg/codeguard"
)

func TestSDKRuleMetadataForSemanticTestAdequacyRule(t *testing.T) {
	rule := requireRuleMetadata(t, "quality.ai.semantic-test-adequacy")
	assertExecutionModel(t, rule, codeguard.RuleExecutionModelCommandDriven)
	assertLanguageCoverage(
		t,
		rule,
		codeguard.RuleLanguageCoverageFixed,
		codeguard.RuleLanguageGo,
		codeguard.RuleLanguageJavaScript,
		codeguard.RuleLanguagePython,
		codeguard.RuleLanguageTypeScript,
	)
}

func TestSDKRuleMetadataForContractDriftRule(t *testing.T) {
	rule := requireRuleMetadata(t, "quality.ai.contract-drift")
	assertExecutionModel(t, rule, codeguard.RuleExecutionModelCommandDriven)
	assertLanguageCoverage(
		t,
		rule,
		codeguard.RuleLanguageCoverageFixed,
		codeguard.RuleLanguageGo,
		codeguard.RuleLanguageJavaScript,
		codeguard.RuleLanguagePython,
		codeguard.RuleLanguageTypeScript,
	)
}

func TestSDKRuleMetadataForCustomRulePack(t *testing.T) {
	cfg := codeguard.ExampleConfig()
	cfg.RulePacks = []codeguard.RulePackConfig{{
		Name: "repo-policy",
		Rules: []codeguard.CustomRuleConfig{{
			ID:       "custom.disallow-env",
			Title:    "Disallow env files",
			Severity: "fail",
			Message:  "env files must not be committed",
			Paths:    []string{".env"},
		}},
	}}

	var customRule codeguard.RuleMetadata
	for _, meta := range codeguard.RulesForConfig(cfg) {
		if meta.ID == "custom.disallow-env" {
			customRule = meta
			break
		}
	}
	if customRule.ID == "" {
		t.Fatal("expected custom.disallow-env metadata")
	}
	assertExecutionModel(t, customRule, codeguard.RuleExecutionModelLanguageAgnostic)
	assertLanguageCoverage(t, customRule, codeguard.RuleLanguageCoverageConfigurable)
}

func TestSDKRuleMetadataForNaturalLanguageCustomRulePack(t *testing.T) {
	cfg := codeguard.ExampleConfig()
	cfg.RulePacks = []codeguard.RulePackConfig{{
		Name: "repo-policy",
		Rules: []codeguard.CustomRuleConfig{{
			ID:              "custom.no-request-body-logs",
			Title:           "Never log request bodies",
			Severity:        "fail",
			Message:         "request bodies must not be logged in handlers",
			NaturalLanguage: "never log request bodies in handlers",
			Paths:           []string{"handlers/**"},
		}},
	}}

	var customRule codeguard.RuleMetadata
	for _, meta := range codeguard.RulesForConfig(cfg) {
		if meta.ID == "custom.no-request-body-logs" {
			customRule = meta
			break
		}
	}
	if customRule.ID == "" {
		t.Fatal("expected custom.no-request-body-logs metadata")
	}
	assertExecutionModel(t, customRule, codeguard.RuleExecutionModelCommandDriven)
	assertLanguageCoverage(t, customRule, codeguard.RuleLanguageCoverageConfigurable)
}

func TestSDKRuleMetadataFixTemplatesPopulated(t *testing.T) {
	rules := codeguard.Rules()
	if len(rules) == 0 {
		t.Fatal("expected a non-empty rule catalog")
	}
	for _, rule := range rules {
		if strings.TrimSpace(rule.FixTemplate.Text) == "" {
			t.Errorf("%s fix template text is empty, want a populated template", rule.ID)
		}
		switch rule.FixTemplate.Kind {
		case codeguard.FixTemplateKindDeterministic, codeguard.FixTemplateKindGuided:
		default:
			t.Errorf("%s fix template kind = %q, want deterministic or guided", rule.ID, rule.FixTemplate.Kind)
		}
	}
}
