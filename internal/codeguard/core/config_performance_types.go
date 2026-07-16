package core

// PerformanceBudgetKindFileSize budgets the on-disk size of a file (or the
// summed size of a glob's matches); PerformanceBudgetKindBundleStats budgets
// sizes recorded in a bundler stats JSON file (esbuild metafile or webpack
// stats); PerformanceBudgetKindCargoTimings budgets build times recorded in a
// Cargo --timings HTML report.
const (
	PerformanceBudgetKindFileSize       = "file-size"
	PerformanceBudgetKindBundleStats    = "bundle-stats"
	PerformanceBudgetKindClangTimeTrace = "clang-time-trace"
	PerformanceBudgetKindCargoTimings   = "cargo-timings"
)

// PerformanceBudgetConfig is one entry of performance_rules.budgets: a named
// size gate over a build artifact. Path is resolved relative to the target
// directory and must stay inside it. A missing artifact is reported as a warn
// finding, never a hard error, so budgets on optional build outputs (e.g.
// dist/ that only exists after a release build) stay usable.
type PerformanceBudgetConfig struct {
	// Name identifies the budget in findings and must be non-empty.
	Name string `json:"name" yaml:"name"`
	// Kind is "file-size", "bundle-stats", "clang-time-trace", or
	// "cargo-timings".
	Kind string `json:"kind" yaml:"kind"`
	// Path is the target-relative artifact path. For "file-size" it may be a
	// glob (the matched sizes are summed); for "bundle-stats" it names the
	// stats JSON file.
	Path string `json:"path" yaml:"path"`
	// Asset (bundle-stats only) budgets a single named asset/output from the
	// stats file instead of the total across all of them.
	Asset string `json:"asset,omitempty" yaml:"asset,omitempty"`
	// Event (clang-time-trace only) budgets the summed duration of matching
	// trace events instead of the whole trace span.
	Event string `json:"event,omitempty" yaml:"event,omitempty"`
	// Crate (cargo-timings only) budgets the summed compile time of one crate
	// from a Cargo timings report instead of the whole build span.
	Crate string `json:"crate,omitempty" yaml:"crate,omitempty"`
	// MaxBytes is the budget; it must be positive.
	MaxBytes int64 `json:"max_bytes" yaml:"max_bytes"`
	// MaxMilliseconds applies to timing-based budgets such as clang-time-trace
	// and cargo-timings.
	MaxMilliseconds int64 `json:"max_milliseconds,omitempty" yaml:"max_milliseconds,omitempty"`
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

// PerformanceBuildRegressionConfig tunes performance_rules.build_regression,
// the generic build-time regression gate. It is off by default because it
// runs repository-configured build commands, which execute repository code.
type PerformanceBuildRegressionConfig struct {
	// Enabled turns the gate on. Defaults to false.
	Enabled *bool `json:"enabled,omitempty" yaml:"enabled,omitempty"`
	// Commands lists the build commands to time. Each command's Name must be
	// unique within the list because the baseline is keyed by command name.
	Commands []CommandCheckConfig `json:"commands,omitempty" yaml:"commands,omitempty"`
	// MaxRegressionPercent is the wall-clock slowdown tolerated per command
	// before a finding is emitted. Defaults to 20.
	MaxRegressionPercent float64 `json:"max_regression_percent,omitempty" yaml:"max_regression_percent,omitempty"`
	// BaselinePath stores the build-regression baseline JSON. Defaults to a
	// file derived from cache.path (e.g. .codeguard/cache.build-baseline.json)
	// and, like the other config-controlled artifact paths, must stay inside
	// the config directory.
	BaselinePath string `json:"baseline_path,omitempty" yaml:"baseline_path,omitempty"`
}
