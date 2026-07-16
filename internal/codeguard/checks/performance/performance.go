// Package performance implements the performance check section: N+1 query
// patterns, allocation-heavy loops, blocking I/O in request paths, and
// unbounded concurrency. These rules previously lived in the quality section
// under quality.* rule IDs; they moved here as performance.* in v0.9.0.
package performance

import (
	"context"
	"strings"

	"github.com/devr-tools/codeguard/internal/codeguard/checks/support"
	"github.com/devr-tools/codeguard/internal/codeguard/core"
)

func Run(ctx context.Context, env support.Context) core.SectionResult {
	findings := support.CollectTargetFindings(ctx, env, performanceTargetFindings)
	return env.FinalizeSection("performance", "Performance", findings)
}

func performanceTargetFindings(ctx context.Context, env support.Context, target core.TargetConfig) []core.Finding {
	findings := make([]core.Finding, 0)
	switch support.NormalizedLanguage(target.Language) {
	case "", "go":
		findings = append(findings, env.ScanTargetFiles(target, "performance", func(rel string) bool {
			return strings.HasSuffix(rel, ".go")
		}, func(file string, data []byte) []core.Finding {
			return goFindingsForFile(env, file, data)
		})...)
	case "python", "py":
		findings = append(findings, env.ScanTargetFiles(target, "performance", func(rel string) bool {
			return strings.HasSuffix(strings.ToLower(rel), ".py")
		}, func(file string, data []byte) []core.Finding {
			return pythonPerformanceFindings(env, file, data)
		})...)
	case "typescript", "javascript", "ts", "tsx", "js", "jsx":
		findings = append(findings, typeScriptPerformanceTargetFindings(env, target)...)
	}
	// Measurement-based gates: artifact size budgets are language-agnostic and
	// run for every target; the benchmark-regression gate only applies to Go
	// targets (it shells out to go test -bench).
	findings = append(findings, budgetFindings(env, target)...)
	findings = append(findings, benchmarkFindings(ctx, env, target)...)
	return findings
}

func goFindingsForFile(env support.Context, file string, data []byte) []core.Finding {
	fset, parsed, err := support.ParseGoSource(env, file, data)
	if err != nil {
		// Unparseable Go is the quality section's problem (quality.parse-error);
		// the performance pass just has nothing to inspect.
		return nil
	}
	return goPerformanceFindings(env, file, fset, parsed)
}

// warnFinding builds a warn-level finding; every performance rule reports at
// warn severity with the unspecified/medium confidence default.
func warnFinding(env support.Context, ruleID string, file string, line int, column int, message string) core.Finding {
	return env.NewFinding(support.FindingInput{
		RuleID:  ruleID,
		Level:   "warn",
		Path:    file,
		Line:    line,
		Column:  column,
		Message: message,
	})
}

func isTypeScriptLikeFile(rel string) bool {
	return support.IsTypeScriptLikeFile(rel)
}
