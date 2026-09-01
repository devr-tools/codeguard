package quality

import (
	"context"
	"fmt"
	"sort"
	"strconv"

	"github.com/devr-tools/codeguard/internal/codeguard/checks/support"
	"github.com/devr-tools/codeguard/internal/codeguard/core"
)

func Run(ctx context.Context, env support.Context) core.SectionResult {
	return runQualitySection(ctx, env)
}

func runQualitySection(ctx context.Context, env support.Context) core.SectionResult {
	var unresolved []unresolvedMutationEvidence
	var diagnostics []core.Diagnostic
	findings := support.CollectTargetFindings(ctx, env, func(ctx context.Context, env support.Context, target core.TargetConfig) []core.Finding {
		analysis := qualityTargetAnalysis(ctx, env, target)
		unresolved = append(unresolved, analysis.unresolved...)
		diagnostics = append(diagnostics, analysis.diagnostics...)
		return analysis.findings
	})
	findings = append(findings, provenancePolicyFindings(env, findings)...) //nolint:contextcheck // git helpers use a contained timeout; deeper ctx threading is a tracked follow-up
	diagnostics = append(diagnostics, unresolvedMutationDiagnostics(unresolved)...)
	return env.FinalizeSectionWithDiagnostics("quality", "Code Quality", findings, diagnostics)
}

type qualityTargetScan struct {
	findings    []core.Finding
	unresolved  []unresolvedMutationEvidence
	diagnostics []core.Diagnostic
}

func qualityTargetAnalysis(ctx context.Context, env support.Context, target core.TargetConfig) qualityTargetScan {
	language := languageQualityAnalysis(ctx, env, target)
	findings := language.findings
	findings = append(findings, environmentBranchingFindings(env, target)...)
	findings = append(findings, cppToolingFindings(ctx, env, target)...)
	findings = append(findings, goToolchainDeadCodeFindings(ctx, env, target)...)
	findings = append(findings, rustToolchainDeadCodeFindings(ctx, env, target)...)
	findings = append(findings, cppToolchainDeadCodeFindings(env, target)...)
	findings = append(findings, pythonToolchainDeadCodeFindings(env, target)...)
	cloneAnalysis := cloneFindingsForTarget(env, target)
	findings = append(findings, cloneAnalysis.findings...)
	findings = append(findings, aiTargetFindings(env, target)...)
	findings = append(findings, semanticFindings(ctx, env, target)...)
	findings = append(findings, commandFindings(ctx, env, target)...)
	findings = append(findings, coverageDeltaFindings(ctx, env, target)...)
	if localPrecisionEnabled(env) {
		findings = append(findings, maintainabilityDeltaFindings(env, target)...)
		findings = append(findings, maintainabilityHistoryFindings(ctx, env, target)...)
	}
	maybePutAISlopArtifact(env, target, findings)
	findings = append(findings, changeRiskFindings(env, target, findings)...) //nolint:contextcheck // git helpers use a contained timeout; deeper ctx threading is a tracked follow-up
	return qualityTargetScan{findings: findings, unresolved: language.unresolved, diagnostics: cloneAnalysis.diagnostics}
}

func unresolvedMutationDiagnostics(unresolved []unresolvedMutationEvidence) []core.Diagnostic {
	counts := make(map[string]int)
	for _, evidence := range unresolved {
		if evidence.Language != "" {
			counts[evidence.Language]++
		}
	}
	languages := make([]string, 0, len(counts))
	for language := range counts {
		languages = append(languages, language)
	}
	sort.Strings(languages)
	diagnostics := make([]core.Diagnostic, 0, len(languages))
	for _, language := range languages {
		count := counts[language]
		diagnostics = append(diagnostics, core.Diagnostic{
			ID:      "quality.structural-unresolved-symbols",
			Level:   "info",
			Kind:    "analysis",
			Message: fmt.Sprintf("retained %d unresolved mutation symbol(s) during %s structural analysis", count, language),
			Metadata: map[string]string{
				"language": language,
				"count":    strconv.Itoa(count),
			},
		})
	}
	return diagnostics
}

func commandFindings(ctx context.Context, env support.Context, target core.TargetConfig) []core.Finding {
	return support.SectionCommandFindings(ctx, env, target, support.SectionCommandSpec{
		Checks:  env.Config.Checks.QualityRules.LanguageCommands[support.NormalizedLanguage(target.Language)],
		RuleID:  "quality.command-check",
		Section: "quality",
	})
}
