package performance

import (
	"context"
	"fmt"
	"strings"

	"github.com/devr-tools/codeguard/internal/codeguard/checks/support"
	"github.com/devr-tools/codeguard/internal/codeguard/core"
	"github.com/devr-tools/codeguard/internal/codeguard/runner/buildregression"
)

func buildRegressionFindings(ctx context.Context, env support.Context, target core.TargetConfig) []core.Finding {
	cfg := env.Config.Checks.PerformanceRules.BuildRegression
	if cfg.Enabled == nil || !*cfg.Enabled {
		return nil
	}
	if len(cfg.Commands) == 0 {
		return []core.Finding{buildRegressionWarn(env, "no build commands configured: set performance_rules.build_regression.commands")}
	}
	baselinePath := buildRegressionBaselinePath(env, cfg)
	if baselinePath == "" {
		return []core.Finding{buildRegressionWarn(env, "no baseline path available: set performance_rules.build_regression.baseline_path or enable cache.path")}
	}
	results := make([]buildregression.Result, 0, len(cfg.Commands))
	for _, check := range cfg.Commands {
		result, output, err := buildregression.RunCommand(ctx, target.Path, target, check)
		if err != nil {
			return []core.Finding{buildRegressionWarn(env, buildCommandFailureMessage(check.Name, output, err))}
		}
		results = append(results, result)
	}
	return compareBuildRegressionBaseline(env, cfg, baselinePath, results)
}

func buildCommandFailureMessage(name string, output string, err error) string {
	var message strings.Builder
	_, _ = fmt.Fprintf(&message, "build command %q failed: ", name)
	if trimmed := support.TrimmedOutput(output); trimmed != "" {
		message.WriteString(trimmed)
		return message.String()
	}
	message.WriteString(err.Error())
	return message.String()
}

func compareBuildRegressionBaseline(env support.Context, cfg core.PerformanceBuildRegressionConfig, baselinePath string, results []buildregression.Result) []core.Finding {
	baseline, ok := buildregression.LoadBaseline(baselinePath)
	if !ok {
		if err := buildregression.WriteBaseline(baselinePath, results); err != nil {
			return []core.Finding{buildRegressionWarn(env, fmt.Sprintf("could not write build regression baseline %q: %v", baselinePath, err))}
		}
		return nil
	}
	findings := make([]core.Finding, 0)
	for _, regression := range buildregression.Compare(baseline, results, cfg.MaxRegressionPercent) {
		findings = append(findings, buildRegressionWarn(env, fmt.Sprintf(
			"build command %s regressed: %.1f ms vs baseline %.1f ms (+%.1f%%, threshold %.0f%%)",
			regression.Name, regression.CurrentDurationMillis, regression.BaselineDurationMillis, regression.Percent, cfg.MaxRegressionPercent)))
	}
	if _, err := buildregression.MergeNewCommands(baselinePath, baseline, results); err != nil {
		findings = append(findings, buildRegressionWarn(env, fmt.Sprintf("could not update build regression baseline %q: %v", baselinePath, err)))
	}
	return findings
}

func buildRegressionBaselinePath(env support.Context, cfg core.PerformanceBuildRegressionConfig) string {
	if trimmed := strings.TrimSpace(cfg.BaselinePath); trimmed != "" {
		return trimmed
	}
	return buildregression.BaselinePathForBase(env.Config.Cache.Path)
}

func buildRegressionWarn(env support.Context, message string) core.Finding {
	return env.NewFinding(support.FindingInput{
		RuleID:  "performance.build-regression",
		Level:   "warn",
		Message: message,
	})
}
