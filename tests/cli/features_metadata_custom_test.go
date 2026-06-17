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
	ruleIDs := []string{
		"quality.gofmt",
		"quality.ai.swallowed-error",
		"quality.ai.hallucinated-import",
		"quality.ai.narrative-comment",
		"quality.ai.dead-code",
		"quality.ai.over-mocked-test",
		"quality.ai.contract-drift",
		"quality.ai.semantic-test-adequacy",
		"quality.javascript.explicit-any",
		"quality.javascript.ts-ignore",
		"quality.javascript.debugger-statement",
		"quality.javascript.non-null-assertion",
		"prompts.secret-interpolation",
		"prompts.agent-standing-permissions",
		"prompts.mcp-config-risk",
		"ci.test-without-assertion",
		"quality.max-function-lines",
		"quality.cyclomatic-complexity",
	}
	for _, ruleID := range ruleIDs {
		rule := requireRuleMetadata(t, ruleID)
		if strings.TrimSpace(rule.FixTemplate) == "" {
			t.Fatalf("%s fix template is empty, want a populated template", ruleID)
		}
	}
}
