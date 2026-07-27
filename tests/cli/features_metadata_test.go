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

func TestSDKRuleMetadataForReliabilityParityRules(t *testing.T) {
	for _, ruleID := range []string{
		"reliability.missing-cancellation",
		"reliability.missing-graceful-shutdown",
		"reliability.missing-concurrency-limit",
		"reliability.resource-leak",
		"reliability.swallowed-error",
		"reliability.lost-error-context",
	} {
		t.Run(ruleID, func(t *testing.T) {
			rule := requireRuleMetadata(t, ruleID)
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
			if rule.FixTemplate.Kind == "" {
				t.Fatalf("expected %s to expose a fix template", ruleID)
			}
		})
	}
}

func TestSDKRuleMetadataForLocalQualityPrecisionRules(t *testing.T) {
	for _, ruleID := range []string{
		"naming.generic-identifier",
		"naming.behavior-mismatch",
		"naming.boolean-not-predicate",
		"naming.domain-vocabulary-drift",
		"naming.unknown-abbreviation",
		"naming.cardinality-mismatch",
		"naming.implementation-leak",
		"naming.missing-unit",
		"naming.role-suffix-overuse",
		"naming.cross-layer-inconsistency",
		"function.excessive-parameters",
		"function.mixed-abstraction-level",
		"function.command-query-mix",
		"function.hidden-mutation",
		"function.inconsistent-return-contract",
		"function.multiple-responsibilities",
		"function.orchestration-domain-mix",
		"function.partial-result",
		"error.logged-and-ignored",
		"error.context-lost",
		"error.logged-and-returned",
		"error.generic-message",
		"error.wrong-abstraction-level",
		"error.inconsistent-wrapping",
		"error.retryable-not-distinguished",
		"error.user-message-leaks-internals",
		"error.partial-failure-hidden",
		"error.cleanup-error-ignored",
		"error.panic-on-recoverable-path",
		"error.exception-used-for-control-flow",
		"error.fallback-hides-corruption",
		"defensive.unchecked-type-assertion",
		"defensive.unsafe-numeric-conversion",
		"defensive.unvalidated-boundary-input",
		"defensive.invalid-state-representable",
		"defensive.null-assumption",
		"defensive.integer-overflow",
		"defensive.bounds-assumption",
		"defensive.unsafe-default",
		"defensive.non-exhaustive-branch",
		"defensive.unchecked-external-response",
		"defensive.missing-schema-validation",
		"defensive.missing-resource-limit",
		"defensive.invalid-state-transition",
		"defensive.fail-open-authorization",
		"maintainability.public-surface-growth",
		"maintainability.dependency-growth",
		"smell.shotgun-surgery-history",
		"smell.divergent-change-history",
	} {
		t.Run(ruleID, func(t *testing.T) {
			rule := requireRuleMetadata(t, ruleID)
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
			if rule.DefaultLevel != "warn" {
				t.Fatalf("%s default level = %q, want warn", ruleID, rule.DefaultLevel)
			}
			if rule.FixTemplate.Kind == "" {
				t.Fatalf("expected local-quality fix template for %s", ruleID)
			}
		})
	}
}

func TestSDKRuleMetadataForStructuralSmellRules(t *testing.T) {
	for _, ruleID := range []string{
		"smell.god-object",
		"smell.feature-envy",
		"smell.middle-man",
		"smell.message-chain",
		"smell.data-clump",
		"smell.switch-on-type",
	} {
		t.Run(ruleID, func(t *testing.T) {
			rule := requireRuleMetadata(t, ruleID)
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
			if rule.DefaultLevel != "warn" {
				t.Fatalf("%s default level = %q, want warn", ruleID, rule.DefaultLevel)
			}
			if rule.FixTemplate.Kind != codeguard.FixTemplateKindGuided {
				t.Fatalf("expected guided structural-smell fix template for %s, got %q", ruleID, rule.FixTemplate.Kind)
			}
		})
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
