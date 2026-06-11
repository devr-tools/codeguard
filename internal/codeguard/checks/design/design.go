package design

import (
	"context"

	"github.com/devr-tools/codeguard/internal/codeguard/checks/support"
	"github.com/devr-tools/codeguard/internal/codeguard/core"
)

func Run(ctx context.Context, env support.Context) core.SectionResult {
	findings := make([]core.Finding, 0)
	for _, target := range env.Config.Targets {
		switch support.NormalizedLanguage(target.Language) {
		case "", "go":
			findings = append(findings, goTargetFindings(env, target)...)
		case "typescript", "javascript", "ts", "tsx", "js", "jsx":
			findings = append(findings, typeScriptTargetFindings(ctx, env, target)...)
		case "python", "py":
			findings = append(findings, pythonTargetFindings(env, target)...)
		}
		findings = append(findings, commandFindings(ctx, env, target)...)
	}
	return env.FinalizeSection("design", "Design Patterns", findings)
}

func typeScriptTargetFindings(ctx context.Context, env support.Context, target core.TargetConfig) []core.Finding {
	return typeScriptTargetFindingsImpl(ctx, env, target)
}

func commandFindings(ctx context.Context, env support.Context, target core.TargetConfig) []core.Finding {
	language := support.NormalizedLanguage(target.Language)
	findings := support.RunCommandChecks(ctx, env, target, env.Config.Checks.DesignRules.LanguageCommands[language], func(check core.CommandCheckConfig, output string, err error) core.Finding {
		return env.NewFinding(support.FindingInput{
			RuleID:  "design.command-check",
			Level:   "fail",
			Message: support.CommandFailureMessage("design", target, check, output, err),
		})
	})
	findings = append(findings, support.RunDiffCommandChecks(ctx, env, target, env.Config.Checks.DesignRules.LanguageDiffCommands[language], func(check core.CommandCheckConfig, output string, err error) core.Finding {
		return env.NewFinding(support.FindingInput{
			RuleID:  "design.diff-command-check",
			Level:   "fail",
			Message: support.DiffCommandFailureMessage("design", target, check, output, err),
		})
	})...)
	return findings
}
