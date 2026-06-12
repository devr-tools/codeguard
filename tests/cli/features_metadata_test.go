package cli_test

import (
	"reflect"
	"testing"

	"github.com/devr-tools/codeguard/pkg/codeguard"
)

func TestSDKRuleMetadataForBuiltInGoRule(t *testing.T) {
	rule := requireRuleMetadata(t, "quality.gofmt")
	assertExecutionModel(t, rule, codeguard.RuleExecutionModelGoNative)
	assertLanguageCoverage(t, rule, codeguard.RuleLanguageCoverageFixed, codeguard.RuleLanguageGo)
}

func TestSDKRuleMetadataForMultiLanguageRule(t *testing.T) {
	rule := requireRuleMetadata(t, "quality.max-function-lines")
	assertExecutionModel(t, rule, codeguard.RuleExecutionModelLanguageAgnostic)
	assertLanguageCoverage(
		t,
		rule,
		codeguard.RuleLanguageCoverageFixed,
		codeguard.RuleLanguageCSharp,
		codeguard.RuleLanguageGo,
		codeguard.RuleLanguageJava,
		codeguard.RuleLanguagePython,
		codeguard.RuleLanguageRuby,
		codeguard.RuleLanguageRust,
		codeguard.RuleLanguageTypeScript,
	)
}

func TestSDKRuleMetadataForTypeScriptRule(t *testing.T) {
	rule := requireRuleMetadata(t, "quality.typescript.explicit-any")
	assertLanguageCoverage(t, rule, codeguard.RuleLanguageCoverageFixed, codeguard.RuleLanguageTypeScript)
}

func TestSDKRuleMetadataForCommandDrivenRule(t *testing.T) {
	rule := requireRuleMetadata(t, "security.command-check")
	assertExecutionModel(t, rule, codeguard.RuleExecutionModelCommandDriven)
	assertLanguageCoverage(t, rule, codeguard.RuleLanguageCoverageConfigurable)
}

func TestSDKRuleMetadataForRepositoryWideRule(t *testing.T) {
	rule := requireRuleMetadata(t, "security.hardcoded-secret")
	assertLanguageCoverage(t, rule, codeguard.RuleLanguageCoverageRepositoryWide)
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
