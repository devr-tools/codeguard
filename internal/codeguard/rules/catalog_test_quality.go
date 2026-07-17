package rules

import "github.com/devr-tools/codeguard/internal/codeguard/core"

var testQualityCatalog = map[string]core.RuleMetadata{
	"ci.test-without-assertion": {
		ID:             "ci.test-without-assertion",
		Section:        "CI/CD",
		DefaultLevel:   "warn",
		ExecutionModel: core.RuleExecutionModelLanguageAgnostic,
		LanguageCoverage: core.FixedRuleLanguageCoverage(
			core.RuleLanguageGo,
			core.RuleLanguagePython,
			core.RuleLanguageTypeScript,
			core.RuleLanguageJavaScript,
			core.RuleLanguageCPP,
		),
		Title:       "Test without assertion",
		Description: "Warns when a test function contains no recognizable assertion. Custom assertion helper names can be registered via ci_rules.test_quality.assertion_helpers.",
		HowToFix:    "Assert on the behavior under test, or register the project's assertion helper names in the configuration.",
	},
	"ci.always-true-test-assertion": {
		ID:             "ci.always-true-test-assertion",
		Section:        "CI/CD",
		DefaultLevel:   "warn",
		ExecutionModel: core.RuleExecutionModelLanguageAgnostic,
		LanguageCoverage: core.FixedRuleLanguageCoverage(
			core.RuleLanguageGo,
			core.RuleLanguagePython,
			core.RuleLanguageTypeScript,
			core.RuleLanguageJavaScript,
			core.RuleLanguageCPP,
		),
		Title:       "Always-true test assertion",
		Description: "Warns when every assertion in a test only checks constants (for example EXPECT_TRUE(true), expect(true).toBe(true), or assert 1 == 1), so the test can never fail.",
		HowToFix:    "Replace constant assertions with assertions on values produced by the code under test.",
	},
	"ci.conditional-assertion": {
		ID:             "ci.conditional-assertion",
		Section:        "CI/CD",
		DefaultLevel:   "warn",
		ExecutionModel: core.RuleExecutionModelLanguageAgnostic,
		LanguageCoverage: core.FixedRuleLanguageCoverage(
			core.RuleLanguageGo,
			core.RuleLanguagePython,
			core.RuleLanguageTypeScript,
			core.RuleLanguageJavaScript,
			core.RuleLanguageCPP,
		),
		Title:       "Conditionally executed assertions",
		Description: "Warns when every assertion in a test sits inside a conditional without an else branch, so the assertions may silently never run. Explicit failure calls such as Go t.Fatal and C++ ADD_FAILURE/FAIL are not flagged.",
		HowToFix:    "Move assertions out of the conditional, or fail the test explicitly in the branch where the assertions are skipped.",
	},
}
