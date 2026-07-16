package cli_test

import (
	"strings"
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
		codeguard.RuleLanguageCPP,
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

func TestSDKRuleMetadataForSupplyChainRule(t *testing.T) {
	rule := requireRuleMetadata(t, "supply_chain.lockfile-drift")
	assertExecutionModel(t, rule, codeguard.RuleExecutionModelLanguageAgnostic)
	assertLanguageCoverage(t, rule, codeguard.RuleLanguageCoverageRepositoryWide)
}

func TestSDKRuleMetadataFixTemplateIncludesBeforeAfterSnippet(t *testing.T) {
	rule := requireRuleMetadata(t, "quality.gofmt")
	if !strings.Contains(rule.FixTemplate.Text, "Before:") || !strings.Contains(rule.FixTemplate.Text, "After:") {
		t.Fatalf("expected before/after snippet in gofmt fix template, got %q", rule.FixTemplate.Text)
	}
	if rule.FixTemplate.Kind != codeguard.FixTemplateKindDeterministic {
		t.Fatalf("expected deterministic gofmt fix template, got %q", rule.FixTemplate.Kind)
	}
}
