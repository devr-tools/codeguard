package quality

import (
	"context"
	"fmt"
	"strings"

	"github.com/devr-tools/codeguard/internal/codeguard/ai/semantic"
	"github.com/devr-tools/codeguard/internal/codeguard/checks/semanticreview"
	"github.com/devr-tools/codeguard/internal/codeguard/checks/support"
	"github.com/devr-tools/codeguard/internal/codeguard/core"
)

// semanticFindings runs the shared command-backed semantic review and emits
// the quality.* verdicts. The request itself is built centrally in
// semanticreview.Options and is shared with the performance section (which
// emits the performance.* verdicts from the same cached response).
func semanticFindings(ctx context.Context, env support.Context, target core.TargetConfig) []core.Finding {
	if !semanticreview.Enabled(env) {
		return nil
	}
	opts := semanticreview.Options(env, target, "quality.")
	if strings.TrimSpace(opts.Command) == "" {
		return []core.Finding{semanticRuntimeFinding(env, target, "semantic review is enabled but no semantic command is configured")}
	}
	findings, err := semantic.Analyze(ctx, opts)
	if err != nil {
		return []core.Finding{semanticRuntimeFinding(env, target, fmt.Sprintf("semantic review command failed for target %q: %v", target.Name, err))}
	}
	return findings
}

func semanticRuntimeFinding(env support.Context, _ core.TargetConfig, message string) core.Finding {
	return env.NewFinding(support.FindingInput{
		RuleID:  "quality.ai.semantic-runtime",
		Level:   "fail",
		Path:    "",
		Line:    0,
		Column:  0,
		Message: message,
	})
}
