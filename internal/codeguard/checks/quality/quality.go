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

func languageQualityFindings(ctx context.Context, env support.Context, target core.TargetConfig) []core.Finding {
	findings := make([]core.Finding, 0)
	switch support.NormalizedLanguage(target.Language) {
	case "", "go":
		findings = append(findings, support.ScanGoFiles(env, target, "quality", func(file string, data []byte) []core.Finding {
			return goFindingsForFile(env, file, data)
		})...)
	case "python", "py":
		findings = append(findings, support.ScanPythonFiles(env, target, "quality", func(file string, data []byte) []core.Finding {
			return pythonFindingsForFile(env, file, data)
		})...)
	case "typescript", "javascript", "ts", "tsx", "js", "jsx":
		findings = append(findings, typeScriptTargetFindings(ctx, env, target)...)
	case "rust", "rs":
		findings = append(findings, support.ScanRustFiles(env, target, "quality", func(file string, data []byte) []core.Finding {
			return rustFindingsForFile(env, file, data)
		})...)
	case "java":
		findings = append(findings, env.ScanTargetFiles(target, "quality", isJavaFile, func(file string, data []byte) []core.Finding {
			return javaFindingsForFile(env, file, data)
		})...)
	case "csharp", "c#", "cs", "dotnet":
		findings = append(findings, env.ScanTargetFiles(target, "quality", isCSharpFile, func(file string, data []byte) []core.Finding {
			return csharpFindingsForFile(env, file, data)
		})...)
	case "ruby", "rb":
		findings = append(findings, env.ScanTargetFiles(target, "quality", isRubyFile, func(file string, data []byte) []core.Finding {
			return rubyFindingsForFile(env, file, data)
		})...)
	}
	return findings
}

func commandFindings(ctx context.Context, env support.Context, target core.TargetConfig) []core.Finding {
	return support.SectionCommandFindings(ctx, env, target, support.SectionCommandSpec{
		Checks:  env.Config.Checks.QualityRules.LanguageCommands[support.NormalizedLanguage(target.Language)],
		RuleID:  "quality.command-check",
		Section: "quality",
	})
}
