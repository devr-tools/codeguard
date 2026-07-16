package rules

import "github.com/devr-tools/codeguard/internal/codeguard/core"

// performanceAICatalog covers the optional LLM-assisted performance lens of
// the command-backed semantic review runtime. The lens shares one semantic
// request with the quality section's lenses; its verdicts are routed into the
// performance section by rule id.
var performanceAICatalog = map[string]core.RuleMetadata{
	"performance.ai.semantic-perf": {
		ID:             "performance.ai.semantic-perf",
		Section:        "Performance",
		DefaultLevel:   "warn",
		ExecutionModel: core.RuleExecutionModelCommandDriven,
		LanguageCoverage: core.FixedRuleLanguageCoverage(
			core.RuleLanguageGo,
			core.RuleLanguagePython,
			core.RuleLanguageTypeScript,
			core.RuleLanguageJavaScript,
		),
		Title:       "AI-assisted performance review",
		Description: "Warns when optional LLM-assisted semantic review finds a performance concern in changed functions that static rules cannot judge: repeated expensive calls that want caching or memoization, algorithmic complexity out of line with plausible input sizes, or obviously redundant work across the change.",
		HowToFix:    "Cache or memoize the repeated expensive work, hoist it out of the loop or request path, batch the calls, or switch to an algorithm suited to the realistic input sizes.",
	},
	"performance.ai.semantic-runtime": {
		ID:             "performance.ai.semantic-runtime",
		Section:        "Performance",
		DefaultLevel:   "fail",
		ExecutionModel: core.RuleExecutionModelCommandDriven,
		LanguageCoverage: core.FixedRuleLanguageCoverage(
			core.RuleLanguageGo,
			core.RuleLanguagePython,
			core.RuleLanguageTypeScript,
			core.RuleLanguageJavaScript,
		),
		Title:       "Semantic performance review runtime failure",
		Description: "Fails when the AI-assisted performance lens was enabled but the configured semantic command was missing, crashed, or returned invalid output, so the performance section would otherwise lose semantic coverage silently.",
		HowToFix:    "Configure a valid semantic command, then fix any runtime or JSON response errors so semantic review can run deterministically.",
	},
}
