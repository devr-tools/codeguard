package core

// CoverageDeltaConfig controls the opt-in quality.coverage-delta check that
// gates changed-line test coverage during diff-mode scans. Running tests
// during a scan is expensive, so the check is disabled by default.
type CoverageDeltaConfig struct {
	// Enabled turns the check on. Defaults to false because the check runs
	// the target's test suite during the scan.
	Enabled *bool `json:"enabled,omitempty"`
	// MinChangedLineCoverage is the warn threshold (percent, default 60):
	// files whose changed lines are covered below this emit a warn finding.
	MinChangedLineCoverage *int `json:"min_changed_line_coverage,omitempty"`
	// FailUnder, when set, escalates findings to fail for files whose
	// changed-line coverage falls below this percentage.
	FailUnder *int `json:"fail_under,omitempty"`
	// LanguageCommands configures non-Go targets: a coverage command to run
	// plus the coverage report it produces. Go targets run
	// `go test -coverprofile` natively and need no entry here.
	LanguageCommands map[string]CoverageCommandConfig `json:"language_commands,omitempty"`
}

// CoverageCommandConfig describes how to produce and read a coverage report
// for a non-Go language target.
type CoverageCommandConfig struct {
	Name    string   `json:"name,omitempty"`
	Command string   `json:"command"`
	Args    []string `json:"args,omitempty"`
	// ReportPath is the coverage report the command writes, relative to the
	// target path.
	ReportPath string `json:"report_path"`
	// Format of the report. Only "lcov" is supported (the default).
	Format string `json:"format,omitempty"`
}

// TestQualityRulesConfig controls the regex-based test assertion rules in the
// CI section (ci.test-without-assertion, ci.always-true-test-assertion,
// ci.conditional-assertion).
type TestQualityRulesConfig struct {
	// Enabled defaults to true.
	Enabled *bool `json:"enabled,omitempty"`
	// AssertionHelpers lists custom assertion helper function names
	// (for example "assertValid") that count as real assertions.
	AssertionHelpers []string `json:"assertion_helpers,omitempty"`
}
