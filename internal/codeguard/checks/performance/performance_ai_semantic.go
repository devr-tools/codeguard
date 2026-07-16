package performance

import (
	"context"
	"fmt"
	"strings"

	"github.com/devr-tools/codeguard/internal/codeguard/ai/semantic"
	"github.com/devr-tools/codeguard/internal/codeguard/checks/semanticreview"
	"github.com/devr-tools/codeguard/internal/codeguard/checks/support"
	"github.com/devr-tools/codeguard/internal/codeguard/core"
)

// semanticPerformanceFindings runs the shared command-backed semantic review
// and emits only the performance.* verdicts (performance.ai.semantic-perf).
// The section registry already gates this pass on checks.performance, and the
// same guards the quality section uses apply on top: the AI runtime plus the
// semantic runtime must be enabled (ai.semantic.enabled or the
// CODEGUARD_SEMANTIC_CHECKS env gate) and a semantic command configured.
//
// Both sections build a byte-identical combined request through
// semanticreview.Options, so the verdict cache plus the in-process
// single-flight in the semantic package guarantee one runtime invocation per
// scan even though sections run in parallel; each side then demultiplexes the
// response by rule-id prefix.
func semanticPerformanceFindings(ctx context.Context, env support.Context, target core.TargetConfig) []core.Finding {
	if !semanticreview.Enabled(env) {
		return nil
	}
	opts := semanticreview.Options(env, target, "performance.")
	if strings.TrimSpace(opts.Command) == "" {
		return []core.Finding{semanticPerformanceRuntimeFinding(env, "semantic review is enabled but no semantic command is configured")}
	}
	findings, err := semantic.Analyze(ctx, opts)
	if err != nil {
		return []core.Finding{semanticPerformanceRuntimeFinding(env, fmt.Sprintf("semantic review command failed for target %q: %v", target.Name, err))}
	}
	return findings
}

func semanticPerformanceRuntimeFinding(env support.Context, message string) core.Finding {
	return env.NewFinding(support.FindingInput{
		RuleID:  "performance.ai.semantic-runtime",
		Level:   "fail",
		Path:    "",
		Line:    0,
		Column:  0,
		Message: message,
	})
}
