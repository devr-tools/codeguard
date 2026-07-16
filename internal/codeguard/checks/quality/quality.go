package quality

import (
	"context"

	"github.com/devr-tools/codeguard/internal/codeguard/checks/support"
	"github.com/devr-tools/codeguard/internal/codeguard/core"
)

func Run(ctx context.Context, env support.Context) core.SectionResult {
	return runQualitySection(ctx, env)
}

func runQualitySection(ctx context.Context, env support.Context) core.SectionResult {
	findings := support.CollectTargetFindings(ctx, env, qualityTargetFindings)
	findings = append(findings, provenancePolicyFindings(env, findings)...) //nolint:contextcheck // git helpers use a contained timeout; deeper ctx threading is a tracked follow-up
	return env.FinalizeSection("quality", "Code Quality", findings)
}

func qualityTargetFindings(ctx context.Context, env support.Context, target core.TargetConfig) []core.Finding {
	findings := languageQualityFindings(ctx, env, target)
	findings = append(findings, cloneFindingsForTarget(env, target)...)
	findings = append(findings, aiTargetFindings(env, target)...)
	findings = append(findings, semanticFindings(ctx, env, target)...)
	findings = append(findings, commandFindings(ctx, env, target)...)
	findings = append(findings, coverageDeltaFindings(ctx, env, target)...)
	maybePutAISlopArtifact(env, target, findings)
	findings = append(findings, changeRiskFindings(env, target, findings)...) //nolint:contextcheck // git helpers use a contained timeout; deeper ctx threading is a tracked follow-up
	return findings
}

func commandFindings(ctx context.Context, env support.Context, target core.TargetConfig) []core.Finding {
	return support.SectionCommandFindings(ctx, env, target, support.SectionCommandSpec{
		Checks:  env.Config.Checks.QualityRules.LanguageCommands[support.NormalizedLanguage(target.Language)],
		RuleID:  "quality.command-check",
		Section: "quality",
	})
}
