// Package performance implements the performance check section: N+1 query
// patterns, allocation-heavy loops, blocking I/O in request paths, and
// unbounded concurrency. These rules previously lived in the quality section
// under quality.* rule IDs; they moved here as performance.* in v0.9.0.
package performance

import (
	"context"

	"github.com/devr-tools/codeguard/internal/codeguard/checks/support"
	"github.com/devr-tools/codeguard/internal/codeguard/core"
)

func Run(ctx context.Context, env support.Context) core.SectionResult {
	findings := support.CollectTargetFindings(ctx, env, performanceTargetFindings)
	return env.FinalizeSection("performance", "Performance", findings)
}

func performanceTargetFindings(ctx context.Context, env support.Context, target core.TargetConfig) []core.Finding {
	findings := make([]core.Finding, 0)
	findings = append(findings, complexityRegressionFindings(env, target)...)
	findings = append(findings, scanLanguagePerformanceFindings(ctx, env, target)...)
	findings = append(findings, semanticPerformanceFindings(ctx, env, target)...)
	// Measurement-based gates: artifact size budgets are language-agnostic and
	// run for every target; the benchmark-regression gate only applies to Go
	// targets (it shells out to go test -bench).
	findings = append(findings, budgetFindings(env, target)...)
	findings = append(findings, buildRegressionFindings(ctx, env, target)...)
	findings = append(findings, benchmarkFindings(ctx, env, target)...)
	maybePutPerformanceScoreArtifact(env, target, findings)
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
func warnFinding(env support.Context, args ...any) core.Finding {
	return env.NewFinding(support.FindingInput{
		RuleID:  args[0].(string),
		Level:   "warn",
		Path:    args[1].(string),
		Line:    args[2].(int),
		Column:  args[3].(int),
		Message: args[4].(string),
	})
}

func isTypeScriptLikeFile(rel string) bool {
	return support.IsTypeScriptLikeFile(rel)
}
