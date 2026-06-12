package rules

import "github.com/devr-tools/codeguard/internal/codeguard/core"

var coverageCatalog = map[string]core.RuleMetadata{
	"quality.coverage-delta": {
		ID:               "quality.coverage-delta",
		Section:          "Code Quality",
		DefaultLevel:     "warn",
		ExecutionModel:   core.RuleExecutionModelCommandDriven,
		LanguageCoverage: core.ConfigurableRuleLanguageCoverage(),
		Title:            "Changed-line test coverage",
		Description:      "Warns when the test coverage of changed lines in a diff scan falls below the configured threshold. Opt-in (quality_rules.coverage_delta.enabled) because it runs the target's tests during the scan; only active in diff mode. Go targets run go test -coverprofile natively, other languages run a configured coverage command and parse its lcov report.",
		HowToFix:         "Add or extend tests so the changed lines are exercised, or raise the configured threshold intentionally.",
	},
}
