package semanticreview

import (
	"context"
	"fmt"
	"strings"

	"github.com/devr-tools/codeguard/internal/codeguard/ai/semantic"
	"github.com/devr-tools/codeguard/internal/codeguard/checks/support"
	"github.com/devr-tools/codeguard/internal/codeguard/core"
)

func Findings(ctx context.Context, env support.Context, target core.TargetConfig, rulePrefix string, runtimeRuleID string) []core.Finding {
	if !Enabled(env) {
		return nil
	}
	opts := Options(env, target, rulePrefix)
	if strings.TrimSpace(opts.Command) == "" {
		return []core.Finding{runtimeFinding(env, runtimeRuleID, "semantic review is enabled but no semantic command is configured")}
	}
	findings, err := semantic.Analyze(ctx, opts)
	if err != nil {
		return []core.Finding{runtimeFinding(env, runtimeRuleID, fmt.Sprintf("semantic review command failed for target %q: %v", target.Name, err))}
	}
	return findings
}

func runtimeFinding(env support.Context, ruleID string, message string) core.Finding {
	return env.NewFinding(support.FindingInput{
		RuleID:  ruleID,
		Level:   "fail",
		Message: message,
	})
}
