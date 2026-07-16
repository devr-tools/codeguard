package config

import "github.com/devr-tools/codeguard/internal/codeguard/core"

// DefaultBenchmarkMaxRegressionPercent is the ns/op slowdown tolerated per
// benchmark before performance.benchmark-regression fires, when the config
// does not set performance_rules.benchmarks.max_regression_percent.
const DefaultBenchmarkMaxRegressionPercent = 20

// applyPerformanceMeasurementDefaults defaults the measurement-based
// performance gates (budgets and benchmark regression). Benchmarks stay off by
// default because they execute the scanned repository's own test code.
func applyPerformanceMeasurementDefaults(dst *core.PerformanceRulesConfig) {
	defaultBoolPtr(&dst.Benchmarks.Enabled, false)
	if dst.Benchmarks.MaxRegressionPercent == 0 {
		dst.Benchmarks.MaxRegressionPercent = DefaultBenchmarkMaxRegressionPercent
	}
	for i := range dst.Budgets {
		if dst.Budgets[i].Level == "" {
			dst.Budgets[i].Level = "warn"
		}
	}
}
