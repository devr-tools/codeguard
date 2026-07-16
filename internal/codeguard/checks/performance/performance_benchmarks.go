package performance

import (
	"context"
	"fmt"
	"path"
	"sort"
	"strings"

	"github.com/devr-tools/codeguard/internal/codeguard/checks/support"
	"github.com/devr-tools/codeguard/internal/codeguard/core"
	"github.com/devr-tools/codeguard/internal/codeguard/runner/benchregression"
)

// benchmarkFindings runs the opt-in benchmark-regression gate for Go targets.
// The heavy lifting (subprocess, parsing, baseline persistence, comparison)
// lives in runner/benchregression — a govulncheck-style dedicated runner —
// so this function only wires config to the runner and turns regressions into
// findings. Findings are pathless (details live in the message) so they are
// reported in diff mode too, where a path outside the diff scope would drop
// them.
func benchmarkFindings(ctx context.Context, env support.Context, target core.TargetConfig) []core.Finding {
	cfg := env.Config.Checks.PerformanceRules.Benchmarks
	if cfg.Enabled == nil || !*cfg.Enabled || !isGoBenchmarkTarget(target) {
		return nil
	}
	packages, finding := benchmarkPackages(env, cfg)
	if finding != nil {
		return []core.Finding{*finding}
	}
	baselinePath := benchmarkBaselinePath(env, cfg)
	if baselinePath == "" {
		return []core.Finding{benchmarkWarn(env, "no baseline path available: set performance_rules.benchmarks.baseline_path or enable cache.path")}
	}
	output, err := benchregression.RunBenchmarks(ctx, target.Path, packages)
	results := benchregression.ParseOutput(output)
	if err != nil && len(results) == 0 {
		return []core.Finding{benchmarkWarn(env, fmt.Sprintf("benchmark run failed: %v", err))}
	}
	if len(results) == 0 {
		return []core.Finding{benchmarkWarn(env, fmt.Sprintf("no benchmarks found in %s; disable performance_rules.benchmarks or point packages at code with Benchmark functions", strings.Join(packages, ", ")))}
	}
	return compareAgainstBaseline(env, cfg, baselinePath, results)
}

// compareAgainstBaseline loads (or seeds) the baseline and reports regressions
// beyond the configured threshold. The first run writes the baseline and emits
// nothing; later runs never overwrite existing baseline entries — a regressed
// measurement must not become the new normal — but do record benchmarks that
// appear for the first time.
func compareAgainstBaseline(env support.Context, cfg core.PerformanceBenchmarksConfig, baselinePath string, results []benchregression.Result) []core.Finding {
	baseline, ok := benchregression.LoadBaseline(baselinePath)
	if !ok {
		if err := benchregression.WriteBaseline(baselinePath, results); err != nil {
			return []core.Finding{benchmarkWarn(env, fmt.Sprintf("could not write benchmark baseline %q: %v", baselinePath, err))}
		}
		return nil
	}
	findings := make([]core.Finding, 0)
	for _, regression := range benchregression.Compare(baseline, results, cfg.MaxRegressionPercent) {
		findings = append(findings, benchmarkWarn(env, fmt.Sprintf(
			"benchmark %s regressed: %.1f ns/op vs baseline %.1f ns/op (+%.1f%%, threshold %.0f%%)",
			regression.Name, regression.CurrentNsPerOp, regression.BaselineNsPerOp, regression.Percent, cfg.MaxRegressionPercent)))
	}
	if _, err := benchregression.MergeNewBenchmarks(baselinePath, baseline, results); err != nil {
		findings = append(findings, benchmarkWarn(env, fmt.Sprintf("could not update benchmark baseline %q: %v", baselinePath, err)))
	}
	return findings
}

// benchmarkPackages resolves which packages to benchmark: the explicit config
// list wins; in diff mode the default is the packages containing changed .go
// files; a full scan with no explicit list reports a warn finding, since
// benchmarking an entire unknown repository by default would be far too
// expensive.
func benchmarkPackages(env support.Context, cfg core.PerformanceBenchmarksConfig) ([]string, *core.Finding) {
	if len(cfg.Packages) > 0 {
		return cfg.Packages, nil
	}
	if env.Mode == core.ScanModeDiff {
		if packages := changedGoPackages(env.ChangedFiles); len(packages) > 0 {
			return packages, nil
		}
		return nil, nil // no Go changes in the diff: nothing to benchmark, nothing to report
	}
	finding := benchmarkWarn(env, "full scans require an explicit performance_rules.benchmarks.packages list (e.g. [\"./...\"]); diff scans default to the packages containing changed Go files")
	return nil, &finding
}

// changedGoPackages maps changed .go files to their containing package
// patterns ("./dir"), deduplicated and sorted. Test files count too: a slower
// helper in a _test.go file regresses benchmarks just as much.
func changedGoPackages(changedFiles []string) []string {
	seen := map[string]struct{}{}
	for _, file := range changedFiles {
		if !strings.HasSuffix(file, ".go") {
			continue
		}
		dir := path.Dir(strings.ReplaceAll(file, "\\", "/"))
		pkg := "./" + dir
		if dir == "." {
			pkg = "."
		}
		seen[pkg] = struct{}{}
	}
	packages := make([]string, 0, len(seen))
	for pkg := range seen {
		packages = append(packages, pkg)
	}
	sort.Strings(packages)
	return packages
}

// benchmarkBaselinePath resolves where the baseline JSON lives: the explicit
// config path (contained within the config directory at load time, like the
// other artifact paths) or a sibling of the scan cache file.
func benchmarkBaselinePath(env support.Context, cfg core.PerformanceBenchmarksConfig) string {
	if trimmed := strings.TrimSpace(cfg.BaselinePath); trimmed != "" {
		return trimmed
	}
	return benchregression.BaselinePathForBase(env.Config.Cache.Path)
}

func benchmarkWarn(env support.Context, message string) core.Finding {
	return env.NewFinding(support.FindingInput{
		RuleID:  "performance.benchmark-regression",
		Level:   "warn",
		Message: message,
	})
}

func isGoBenchmarkTarget(target core.TargetConfig) bool {
	switch support.NormalizedLanguage(target.Language) {
	case "", "go":
		return true
	default:
		return false
	}
}
