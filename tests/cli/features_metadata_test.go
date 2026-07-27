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

func TestSDKRuleMetadataForTypeScriptNamedJavaScriptDesignRules(t *testing.T) {
	for _, ruleID := range []string{
		"design.typescript.generic-module-name",
		"design.typescript.max-methods-per-type",
	} {
		rule := requireRuleMetadata(t, ruleID)
		assertLanguageCoverage(
			t,
			rule,
			codeguard.RuleLanguageCoverageFixed,
			codeguard.RuleLanguageJavaScript,
			codeguard.RuleLanguageTypeScript,
		)
	}
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

func TestSDKRuleMetadataForReliabilityRule(t *testing.T) {
	rule := requireRuleMetadata(t, "reliability.missing-timeout")
	assertExecutionModel(t, rule, codeguard.RuleExecutionModelLanguageAgnostic)
	assertLanguageCoverage(
		t,
		rule,
		codeguard.RuleLanguageCoverageFixed,
		codeguard.RuleLanguageCPP,
		codeguard.RuleLanguageGo,
		codeguard.RuleLanguageJavaScript,
		codeguard.RuleLanguagePython,
		codeguard.RuleLanguageTypeScript,
	)
	if rule.FixTemplate.Kind != codeguard.FixTemplateKindDeterministic {
		t.Fatalf("expected deterministic reliability fix template, got %q", rule.FixTemplate.Kind)
	}
}

func TestSDKRuleMetadataForDataRule(t *testing.T) {
	rule := requireRuleMetadata(t, "data.missing-outbox-strategy")
	assertExecutionModel(t, rule, codeguard.RuleExecutionModelLanguageAgnostic)
	assertLanguageCoverage(
		t,
		rule,
		codeguard.RuleLanguageCoverageFixed,
		codeguard.RuleLanguageCPP,
		codeguard.RuleLanguageGo,
		codeguard.RuleLanguageJavaScript,
		codeguard.RuleLanguagePython,
		codeguard.RuleLanguageTypeScript,
	)
	if rule.FixTemplate.Kind != codeguard.FixTemplateKindGuided {
		t.Fatalf("expected guided data fix template, got %q", rule.FixTemplate.Kind)
	}
}

func TestSDKRuleMetadataForChangeSafetyRule(t *testing.T) {
	rule := requireRuleMetadata(t, "change.oversized-diff")
	assertExecutionModel(t, rule, codeguard.RuleExecutionModelLanguageAgnostic)
	assertLanguageCoverage(t, rule, codeguard.RuleLanguageCoverageRepositoryWide)
	if rule.FixTemplate.Kind != codeguard.FixTemplateKindGuided {
		t.Fatalf("expected guided change safety fix template, got %q", rule.FixTemplate.Kind)
	}
}

func TestSDKRuleMetadataForTestabilityRule(t *testing.T) {
	rule := requireRuleMetadata(t, "testing.behavior-change-without-test")
	assertExecutionModel(t, rule, codeguard.RuleExecutionModelLanguageAgnostic)
	assertLanguageCoverage(
		t,
		rule,
		codeguard.RuleLanguageCoverageFixed,
		codeguard.RuleLanguageCPP,
		codeguard.RuleLanguageGo,
		codeguard.RuleLanguageJavaScript,
		codeguard.RuleLanguagePython,
		codeguard.RuleLanguageTypeScript,
	)
	if rule.FixTemplate.Kind != codeguard.FixTemplateKindGuided {
		t.Fatalf("expected guided testability fix template, got %q", rule.FixTemplate.Kind)
	}
}

func TestSDKRuleMetadataForRefactorRule(t *testing.T) {
	rule := requireRuleMetadata(t, "refactor.behavior-change-detected")
	assertExecutionModel(t, rule, codeguard.RuleExecutionModelLanguageAgnostic)
	assertLanguageCoverage(
		t,
		rule,
		codeguard.RuleLanguageCoverageFixed,
		codeguard.RuleLanguageCPP,
		codeguard.RuleLanguageGo,
		codeguard.RuleLanguageJavaScript,
		codeguard.RuleLanguagePython,
		codeguard.RuleLanguageTypeScript,
	)
	if rule.FixTemplate.Kind != codeguard.FixTemplateKindGuided {
		t.Fatalf("expected guided refactor fix template, got %q", rule.FixTemplate.Kind)
	}
}

func TestSDKRuleMetadataForOperabilityAndDeliveryRules(t *testing.T) {
	cases := []struct {
		ruleID string
	}{
		{ruleID: "observability.sensitive-log-data"},
		{ruleID: "operations.missing-runbook"},
		{ruleID: "delivery.missing-rollback-strategy"},
		{ruleID: "design.unreachable-module"},
		{ruleID: "design.stability-direction"},
	}

	for _, tc := range cases {
		t.Run(tc.ruleID, func(t *testing.T) {
			rule := requireRuleMetadata(t, tc.ruleID)
			assertExecutionModel(t, rule, codeguard.RuleExecutionModelLanguageAgnostic)
			if rule.FixTemplate.Kind == "" {
				t.Fatalf("expected %s to expose a fix template", tc.ruleID)
			}
		})
	}
}

func TestSDKRuleMetadataForNonExpandContractMigration(t *testing.T) {
	rule := requireRuleMetadata(t, "contracts.non-expand-contract-migration")
	assertExecutionModel(t, rule, codeguard.RuleExecutionModelLanguageAgnostic)
	assertLanguageCoverage(t, rule, codeguard.RuleLanguageCoverageRepositoryWide)
	if rule.FixTemplate.Kind != codeguard.FixTemplateKindGuided {
		t.Fatalf("expected guided migration fix template, got %q", rule.FixTemplate.Kind)
	}
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
