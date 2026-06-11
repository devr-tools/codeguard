package design

import (
	"context"

	"github.com/devr-tools/codeguard/internal/codeguard/checks/support"
	"github.com/devr-tools/codeguard/internal/codeguard/core"
)

func Run(ctx context.Context, env support.Context) core.SectionResult {
	findings := support.CollectTargetFindings(ctx, env, func(ctx context.Context, env support.Context, target core.TargetConfig) []core.Finding {
		findings := make([]core.Finding, 0)
		switch support.NormalizedLanguage(target.Language) {
		case "", "go":
			findings = append(findings, goTargetFindings(env, target)...)
		case "typescript", "javascript", "ts", "tsx", "js", "jsx":
			findings = append(findings, typeScriptTargetFindings(ctx, env, target)...)
		case "python", "py":
			findings = append(findings, pythonTargetFindings(env, target)...)
		}
		findings = append(findings, commandFindings(ctx, env, target)...)
		return findings
	})
	return env.FinalizeSection("design", "Design Patterns", findings)
}

func typeScriptTargetFindings(ctx context.Context, env support.Context, target core.TargetConfig) []core.Finding {
	return typeScriptTargetFindingsImpl(ctx, env, target)
}

func commandFindings(ctx context.Context, env support.Context, target core.TargetConfig) []core.Finding {
	language := support.NormalizedLanguage(target.Language)
	findings := support.SectionCommandFindings(ctx, env, target, support.SectionCommandSpec{
		Checks:  env.Config.Checks.DesignRules.LanguageCommands[language],
		RuleID:  "design.command-check",
		Section: "design",
	})
	findings = append(findings, support.SectionDiffCommandFindings(ctx, env, target, support.SectionCommandSpec{
		Checks:  env.Config.Checks.DesignRules.LanguageDiffCommands[language],
		RuleID:  "design.diff-command-check",
		Section: "design",
	})...)
	return findings
}
