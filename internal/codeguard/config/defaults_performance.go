package config

import "github.com/devr-tools/codeguard/internal/codeguard/core"

// DefaultBenchmarkMaxRegressionPercent is the ns/op slowdown tolerated per
// benchmark before performance.benchmark-regression fires, when the config
// does not set performance_rules.benchmarks.max_regression_percent.
const DefaultBenchmarkMaxRegressionPercent = 20

const (
	defaultHotPackageImporterThreshold = 8
	defaultRebuildAmplifierThreshold   = 20
)

// DefaultBuildRegressionMaxPercent keeps the first measured build-regression
// gate at the same noise floor as benchmark regression unless a repo opts in
// to a stricter policy.
const DefaultBuildRegressionMaxPercent = 20

func applyPerformanceMeasurementDefaults(dst *core.PerformanceRulesConfig) {
	defaultBoolPtr(&dst.Benchmarks.Enabled, false)
	if dst.Benchmarks.MaxRegressionPercent == 0 {
		dst.Benchmarks.MaxRegressionPercent = DefaultBenchmarkMaxRegressionPercent
	}
	defaultBoolPtr(&dst.BuildRegression.Enabled, false)
	if dst.BuildRegression.MaxRegressionPercent == 0 {
		dst.BuildRegression.MaxRegressionPercent = DefaultBuildRegressionMaxPercent
	}
	for i := range dst.Budgets {
		if dst.Budgets[i].Level == "" {
			dst.Budgets[i].Level = "warn"
		}
	}
}

func applyPerformanceGraphDefaults(dst *core.PerformanceRulesConfig) {
	defaultInt(&dst.HotPackageImporterThreshold, defaultHotPackageImporterThreshold)
	defaultInt(&dst.RebuildAmplifierThreshold, defaultRebuildAmplifierThreshold)
}
