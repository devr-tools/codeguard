package rules

import "github.com/devr-tools/codeguard/internal/codeguard/core"

// performanceMeasuredCatalog covers the measurement-based performance gates:
// artifact size budgets and benchmark regression. Unlike the pattern rules in
// performanceCatalog, these compare real measurements (file sizes, bundler
// stats, go test -bench timings) against configured limits.
var performanceMeasuredCatalog = map[string]core.RuleMetadata{
	"performance.budget": {
		ID:             "performance.budget",
		Section:        "Performance",
		DefaultLevel:   "warn",
		ExecutionModel: core.RuleExecutionModelLanguageAgnostic,
		Title:          "Performance budget exceeded",
		Description:    "Compares measured artifacts against the budgets configured in performance_rules.budgets: on-disk file or glob-total sizes (kind file-size), bundler stats totals or per-asset sizes from an esbuild metafile or webpack stats JSON (kind bundle-stats), Clang -ftime-trace durations for whole traces or named events (kind clang-time-trace), and Cargo --timings HTML reports for whole-build or per-crate Rust compile time (kind cargo-timings). A missing artifact reports as a warn finding, never a hard error; a budget entry may set level fail to gate the scan.",
		HowToFix:       "Shrink the artifact below the budget (trim dependencies, split bundles, strip debug info, compress assets, or reduce compile-time hotspots) or, if the growth is intentional, raise the matching budget entry deliberately.",
	},
	"performance.benchmark-regression": {
		ID:             "performance.benchmark-regression",
		Section:        "Performance",
		DefaultLevel:   "warn",
		ExecutionModel: core.RuleExecutionModelCommandDriven,
		Title:          "Benchmark regression",
		Description:    "Runs go test -run=^$ -bench=. -benchmem over the configured packages and warns when a benchmark's ns/op regresses beyond performance_rules.benchmarks.max_regression_percent relative to the stored baseline. The first run writes the baseline and reports nothing. Off by default because it executes the repository's own test code (performance_rules.benchmarks.enabled).",
		HowToFix:       "Profile the regressed benchmark (go test -bench=<name> -cpuprofile) and fix the slowdown, or refresh the baseline by deleting the baseline file if the new cost is accepted.",
	},
	"performance.build-regression": {
		ID:             "performance.build-regression",
		Section:        "Performance",
		DefaultLevel:   "warn",
		ExecutionModel: core.RuleExecutionModelCommandDriven,
		Title:          "Build regression",
		Description:    "Runs the configured performance_rules.build_regression.commands, measures each command's wall-clock duration, and warns when a command regresses beyond performance_rules.build_regression.max_regression_percent relative to the stored baseline. The first run writes the baseline and reports nothing. Off by default because it executes repository-configured commands (and therefore requires trusting the repository config).",
		HowToFix:       "Trim the regressed build command's work (incrementalize, reduce invalidation, cache better, or split heavy steps), or refresh the baseline by deleting the baseline file if the new cost is accepted.",
	},
}
