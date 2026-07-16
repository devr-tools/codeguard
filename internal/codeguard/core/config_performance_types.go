package core

// PerformanceBudgetKindFileSize budgets the on-disk size of a file (or the
// summed size of a glob's matches); PerformanceBudgetKindBundleStats budgets
// sizes recorded in a bundler stats JSON file (esbuild metafile or webpack
// stats).
const (
	PerformanceBudgetKindFileSize    = "file-size"
	PerformanceBudgetKindBundleStats = "bundle-stats"
)

// PerformanceBudgetConfig is one entry of performance_rules.budgets: a named
// size gate over a build artifact. Path is resolved relative to the target
// directory and must stay inside it. A missing artifact is reported as a warn
// finding, never a hard error, so budgets on optional build outputs (e.g.
// dist/ that only exists after a release build) stay usable.
type PerformanceBudgetConfig struct {
	// Name identifies the budget in findings and must be non-empty.
	Name string `json:"name" yaml:"name"`
	// Kind is "file-size" or "bundle-stats".
	Kind string `json:"kind" yaml:"kind"`
	// Path is the target-relative artifact path. For "file-size" it may be a
	// glob (the matched sizes are summed); for "bundle-stats" it names the
	// stats JSON file.
	Path string `json:"path" yaml:"path"`
	// Asset (bundle-stats only) budgets a single named asset/output from the
	// stats file instead of the total across all of them.
	Asset string `json:"asset,omitempty" yaml:"asset,omitempty"`
	// MaxBytes is the budget; it must be positive.
	MaxBytes int64 `json:"max_bytes" yaml:"max_bytes"`
	// Level is the finding level when the budget is exceeded: "warn" (default)
	// or "fail".
	Level string `json:"level,omitempty" yaml:"level,omitempty"`
}

// PerformanceBenchmarksConfig tunes performance_rules.benchmarks, the
// benchmark-regression gate. It is off by default because it runs the target
// repository's own test binaries (go test -bench), which executes repository
// code.
type PerformanceBenchmarksConfig struct {
	// Enabled turns the gate on. Defaults to false.
	Enabled *bool `json:"enabled,omitempty" yaml:"enabled,omitempty"`
	// Packages lists the Go package patterns to benchmark (e.g. "./internal/...").
	// Full scans require an explicit list; diff scans default to the packages
	// containing changed .go files.
	Packages []string `json:"packages,omitempty" yaml:"packages,omitempty"`
	// MaxRegressionPercent is the ns/op slowdown tolerated per benchmark before
	// a finding is emitted. Defaults to 20.
	MaxRegressionPercent float64 `json:"max_regression_percent,omitempty" yaml:"max_regression_percent,omitempty"`
	// BaselinePath stores the benchmark baseline JSON. Defaults to a file
	// derived from cache.path (e.g. .codeguard/cache.bench-baseline.json) and,
	// like the other config-controlled artifact paths, must stay inside the
	// config directory.
	BaselinePath string `json:"baseline_path,omitempty" yaml:"baseline_path,omitempty"`
}
