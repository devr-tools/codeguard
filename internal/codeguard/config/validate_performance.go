package config

import (
	"fmt"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/devr-tools/codeguard/internal/codeguard/core"
)

// benchmarkPackagePattern restricts performance_rules.benchmarks.packages
// entries to plain relative Go package patterns. Because the entries are
// appended to codeguard's fixed `go test` invocation, the charset excludes
// anything that could smuggle a flag (leading '-'), an absolute path, or
// shell metacharacters; ".." segments are rejected separately so a pattern
// can never point outside the target.
var benchmarkPackagePattern = regexp.MustCompile(`^(\.|\./[A-Za-z0-9_./-]*)$`)

func validatePerformanceRules(rules core.PerformanceRulesConfig) error {
	if rules.HotPackageImporterThreshold < 0 {
		return fmt.Errorf("performance_rules.hot_package_importer_threshold must not be negative")
	}
	if rules.RebuildAmplifierThreshold < 0 {
		return fmt.Errorf("performance_rules.rebuild_amplifier_threshold must not be negative")
	}
	for idx, budget := range rules.Budgets {
		if err := validatePerformanceBudget(idx, budget); err != nil {
			return err
		}
	}
	if err := validatePerformanceBenchmarks(rules.Benchmarks); err != nil {
		return err
	}
	return validatePerformanceBuildRegression(rules.BuildRegression)
}

func validatePerformanceBudget(idx int, budget core.PerformanceBudgetConfig) error {
	label := fmt.Sprintf("performance_rules.budgets[%d]", idx)
	if strings.TrimSpace(budget.Name) == "" {
		return fmt.Errorf("%s.name is required", label)
	}
	if err := validatePerformanceBudgetKind(label, budget.Kind); err != nil {
		return err
	}
	if err := validatePerformanceBudgetLimit(label, budget); err != nil {
		return err
	}
	if err := validateBudgetPath(label, budget.Path); err != nil {
		return err
	}
	if err := validatePerformanceBudgetOptions(label, budget); err != nil {
		return err
	}
	switch budget.Level {
	case "", "warn", "fail":
	default:
		return fmt.Errorf("%s.level must be \"warn\" or \"fail\"", label)
	}
	return nil
}

func validatePerformanceBudgetKind(label string, kind string) error {
	switch kind {
	case core.PerformanceBudgetKindFileSize, core.PerformanceBudgetKindBundleStats, core.PerformanceBudgetKindClangTimeTrace, core.PerformanceBudgetKindCargoTimings:
		return nil
	default:
		return fmt.Errorf("%s.kind must be %q, %q, %q, or %q", label, core.PerformanceBudgetKindFileSize, core.PerformanceBudgetKindBundleStats, core.PerformanceBudgetKindClangTimeTrace, core.PerformanceBudgetKindCargoTimings)
	}
}

func validatePerformanceBudgetLimit(label string, budget core.PerformanceBudgetConfig) error {
	if budget.Kind == core.PerformanceBudgetKindClangTimeTrace || budget.Kind == core.PerformanceBudgetKindCargoTimings {
		if budget.MaxMilliseconds <= 0 {
			return fmt.Errorf("%s.max_milliseconds must be positive", label)
		}
		return nil
	}
	if budget.MaxBytes <= 0 {
		return fmt.Errorf("%s.max_bytes must be positive", label)
	}
	return nil
}

func validatePerformanceBudgetOptions(label string, budget core.PerformanceBudgetConfig) error {
	if budget.Asset != "" && budget.Kind != core.PerformanceBudgetKindBundleStats {
		return fmt.Errorf("%s.asset only applies to kind %q", label, core.PerformanceBudgetKindBundleStats)
	}
	if budget.Event != "" && budget.Kind != core.PerformanceBudgetKindClangTimeTrace {
		return fmt.Errorf("%s.event only applies to kind %q", label, core.PerformanceBudgetKindClangTimeTrace)
	}
	if budget.Crate != "" && budget.Kind != core.PerformanceBudgetKindCargoTimings {
		return fmt.Errorf("%s.crate only applies to kind %q", label, core.PerformanceBudgetKindCargoTimings)
	}
	return nil
}

// validateBudgetPath lexically rejects budget paths that could leave the
// target directory. The check package re-verifies containment at scan time
// (including symlink resolution); this validation just fails fast on configs
// that are wrong on their face.
func validateBudgetPath(label string, path string) error {
	trimmed := strings.TrimSpace(path)
	if trimmed == "" {
		return fmt.Errorf("%s.path is required", label)
	}
	if filepath.IsAbs(trimmed) {
		return fmt.Errorf("%s.path must be relative to the target directory", label)
	}
	for _, segment := range strings.Split(filepath.ToSlash(trimmed), "/") {
		if segment == ".." {
			return fmt.Errorf("%s.path must not contain \"..\" segments", label)
		}
	}
	return nil
}

func validatePerformanceBenchmarks(benchmarks core.PerformanceBenchmarksConfig) error {
	if benchmarks.MaxRegressionPercent < 0 {
		return fmt.Errorf("performance_rules.benchmarks.max_regression_percent must not be negative")
	}
	for idx, pkg := range benchmarks.Packages {
		if !benchmarkPackagePattern.MatchString(pkg) {
			return fmt.Errorf("performance_rules.benchmarks.packages[%d]: %q is not a valid relative package pattern (expected e.g. \"./...\" or \"./internal/...\")", idx, pkg)
		}
		if containsDotDotSegment(pkg) {
			return fmt.Errorf("performance_rules.benchmarks.packages[%d]: %q must not contain \"..\" path segments", idx, pkg)
		}
	}
	return nil
}

func validatePerformanceBuildRegression(build core.PerformanceBuildRegressionConfig) error {
	if build.Enabled != nil && *build.Enabled && len(build.Commands) == 0 {
		return fmt.Errorf("performance_rules.build_regression.commands must list at least one command when enabled")
	}
	if build.MaxRegressionPercent < 0 {
		return fmt.Errorf("performance_rules.build_regression.max_regression_percent must not be negative")
	}
	seen := make(map[string]struct{}, len(build.Commands))
	for idx, check := range build.Commands {
		label := fmt.Sprintf("performance_rules.build_regression.commands[%d]", idx)
		if strings.TrimSpace(check.Name) == "" {
			return fmt.Errorf("%s.name is required", label)
		}
		if strings.TrimSpace(check.Command) == "" {
			return fmt.Errorf("%s.command is required", label)
		}
		if _, ok := seen[check.Name]; ok {
			return fmt.Errorf("%s.name %q duplicates another build regression command name", label, check.Name)
		}
		seen[check.Name] = struct{}{}
	}
	return nil
}

// containsDotDotSegment reports whether the package pattern has a ".." path
// segment. Go's "..." wildcard is allowed; a bare ".." (or "../x") is not,
// since it would benchmark code outside the target directory.
func containsDotDotSegment(pkg string) bool {
	for _, segment := range strings.Split(pkg, "/") {
		if segment == ".." {
			return true
		}
	}
	return false
}
