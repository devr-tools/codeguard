package performance

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/devr-tools/codeguard/internal/codeguard/checks/support"
	"github.com/devr-tools/codeguard/internal/codeguard/core"
)

// budgetFindings evaluates every performance_rules.budgets entry against the
// target directory. Budget findings carry the artifact path in the message
// rather than in Finding.Path so they survive diff-mode scoping: a budget is
// a repository-level gate on a built artifact, not a per-changed-line lint.
func budgetFindings(env support.Context, target core.TargetConfig) []core.Finding {
	budgets := env.Config.Checks.PerformanceRules.Budgets
	if len(budgets) == 0 {
		return nil
	}
	findings := make([]core.Finding, 0)
	for _, budget := range budgets {
		findings = append(findings, evaluateBudget(env, target, budget)...)
	}
	return findings
}

func evaluateBudget(env support.Context, target core.TargetConfig, budget core.PerformanceBudgetConfig) []core.Finding {
	switch budget.Kind {
	case core.PerformanceBudgetKindFileSize:
		return fileSizeBudgetFindings(env, target, budget)
	case core.PerformanceBudgetKindBundleStats:
		return bundleStatsBudgetFindings(env, target, budget)
	case core.PerformanceBudgetKindClangTimeTrace:
		return clangTimeTraceBudgetFindings(env, target, budget)
	case core.PerformanceBudgetKindCargoTimings:
		return cargoTimingsBudgetFindings(env, target, budget)
	default:
		// Config validation rejects unknown kinds; a programmatically built
		// config that skipped validation still gets a diagnostic, not a panic.
		return []core.Finding{budgetIssueFinding(env, budget, fmt.Sprintf("unknown budget kind %q; budget skipped", budget.Kind))}
	}
}

func fileSizeBudgetFindings(env support.Context, target core.TargetConfig, budget core.PerformanceBudgetConfig) []core.Finding {
	paths, finding := resolveBudgetArtifacts(env, target, budget)
	if finding != nil {
		return []core.Finding{*finding}
	}
	var total int64
	for _, path := range paths {
		info, err := os.Stat(path) //nolint:gosec // path containment verified by resolveBudgetArtifacts
		if err != nil || !info.Mode().IsRegular() {
			continue
		}
		total += info.Size()
	}
	if total <= budget.MaxBytes {
		return nil
	}
	return []core.Finding{budgetExceededFinding(env, budget, fmt.Sprintf("%q totals %d bytes", budget.Path, total))}
}

// resolveBudgetArtifacts resolves the budget path (a literal path, or a glob
// for file-size budgets) within the target directory and enforces containment:
// every resolved artifact must stay inside the target after symlink
// resolution, mirroring the config.containConfigArtifactPaths threat model —
// the budget path comes from repository config, which is untrusted, and must
// never steer codeguard into reading outside the repository. When nothing
// resolves, the returned finding is the warn-level "not found" diagnostic.
func resolveBudgetArtifacts(env support.Context, target core.TargetConfig, budget core.PerformanceBudgetConfig) ([]string, *core.Finding) {
	pattern := filepath.ToSlash(strings.TrimSpace(budget.Path))
	if pattern == "" || filepath.IsAbs(budget.Path) || hasDotDotSegment(pattern) {
		finding := budgetIssueFinding(env, budget, fmt.Sprintf("path %q escapes the target directory; budget skipped", budget.Path))
		return nil, &finding
	}
	matches, err := filepath.Glob(filepath.Join(target.Path, filepath.FromSlash(pattern)))
	if err != nil || len(matches) == 0 {
		finding := budgetIssueFinding(env, budget, fmt.Sprintf("artifact %q not found; budget skipped", budget.Path))
		return nil, &finding
	}
	root, err := canonicalDir(target.Path)
	if err != nil {
		finding := budgetIssueFinding(env, budget, fmt.Sprintf("target directory could not be resolved (%v); budget skipped", err))
		return nil, &finding
	}
	contained := make([]string, 0, len(matches))
	for _, match := range matches {
		if pathWithinRoot(root, match) {
			contained = append(contained, match)
		}
	}
	if len(contained) == 0 {
		finding := budgetIssueFinding(env, budget, fmt.Sprintf("path %q resolves outside the target directory; budget skipped", budget.Path))
		return nil, &finding
	}
	return contained, nil
}

// pathWithinRoot reports whether path stays inside root once symlinks are
// resolved. The path is absolutized first: EvalSymlinks keeps a relative
// input relative (targets configured with relative paths hit this), and
// filepath.Rel cannot mix a relative path with the absolute root. Glob only
// returns existing paths, so resolution is expected to succeed; any failure
// is treated as escape (fail closed).
func pathWithinRoot(root string, path string) bool {
	abs, err := filepath.Abs(path)
	if err != nil {
		return false
	}
	resolved, err := filepath.EvalSymlinks(abs)
	if err != nil {
		return false
	}
	rel, err := filepath.Rel(root, resolved)
	if err != nil {
		return false
	}
	return rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator))
}

func canonicalDir(dir string) (string, error) {
	abs, err := filepath.Abs(dir)
	if err != nil {
		return "", err
	}
	return filepath.EvalSymlinks(abs)
}

func hasDotDotSegment(slashPath string) bool {
	for _, segment := range strings.Split(slashPath, "/") {
		if segment == ".." {
			return true
		}
	}
	return false
}

func budgetExceededFinding(env support.Context, budget core.PerformanceBudgetConfig, measurement string) core.Finding {
	return performanceBudgetLimitFinding(env, budget, measurement, "max_bytes", budget.MaxBytes)
}

func performanceBudgetLimitFinding(env support.Context, budget core.PerformanceBudgetConfig, measurement string, limitLabel string, limit int64) core.Finding {
	level := "warn"
	if budget.Level == "fail" {
		level = "fail"
	}
	return env.NewFinding(support.FindingInput{
		RuleID:  "performance.budget",
		Level:   level,
		Message: fmt.Sprintf("performance budget %q exceeded: %s, over the %s budget of %d", budget.Name, measurement, limitLabel, limit),
	})
}

// budgetIssueFinding reports a budget that could not be measured (missing
// artifact, unreadable stats, escaping path). Always warn-level, regardless of
// the entry's configured level: an absent artifact must never hard-fail a scan
// (dist/ may simply not be built in this environment).
func budgetIssueFinding(env support.Context, budget core.PerformanceBudgetConfig, detail string) core.Finding {
	return env.NewFinding(support.FindingInput{
		RuleID:  "performance.budget",
		Level:   "warn",
		Message: fmt.Sprintf("performance budget %q: %s", budget.Name, detail),
	})
}
