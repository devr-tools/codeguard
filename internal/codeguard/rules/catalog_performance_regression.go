package rules

import "github.com/devr-tools/codeguard/internal/codeguard/core"

// performanceRegressionCatalog covers the diff-only performance regression
// rules. Kept separate from performanceCatalog so parallel additions to the
// performance family do not conflict.
var performanceRegressionCatalog = map[string]core.RuleMetadata{
	"performance.complexity-regression": {
		ID:             "performance.complexity-regression",
		Section:        "Performance",
		DefaultLevel:   "warn",
		ExecutionModel: core.RuleExecutionModelGoNative,
		Title:          "Loop-nesting complexity regression",
		Description:    "Warns in diff scans when a changed function's maximum loop-nesting depth increased relative to the diff base ref (performance_rules.detect_complexity_regression, on by default). Functions that do not exist at the base ref are skipped, and full scans are unaffected: the rule only activates in diff mode.",
		HowToFix:       "Verify the added loop nesting does not iterate over unbounded data; hoist invariant work out of the inner loop, batch lookups, or restructure the iteration (e.g. index by key first) so the nesting does not multiply the input size.",
	},
}
