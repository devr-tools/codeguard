package design

import (
	"context"

	"github.com/devr-tools/codeguard/internal/codeguard/checks/support"
	"github.com/devr-tools/codeguard/internal/codeguard/core"
)

func Run(ctx context.Context, env support.Context) core.SectionResult {
	findings := make([]core.Finding, 0)
	graphs := make([]targetModuleGraph, 0, len(env.Config.Targets))
	for _, target := range env.Config.Targets {
		targetFindings, graph := targetLanguageFindings(ctx, env, target)
		findings = append(findings, targetFindings...)
		if graph != nil {
			findings = append(findings, importCycleFindings(env, graph)...)
			findings = append(findings, godModuleFindings(env, graph)...)
			findings = append(findings, architectureBoundaryFindings(env, target, graph)...)
			findings = append(findings, encapsulationBoundaryFindings(env, target, graph)...)
			findings = append(findings, graphPolicyFindings(env, target, graph)...)
			graphs = append(graphs, targetModuleGraph{target: target, graph: graph})
		}
		findings = append(findings, commandFindings(ctx, env, target)...)
	}
	findings = append(findings, changeImpactFindings(env, graphs)...)
	return env.FinalizeSection("design", "Design Patterns", findings)
}

func targetLanguageFindings(ctx context.Context, env support.Context, target core.TargetConfig) ([]core.Finding, *moduleGraph) {
	switch support.NormalizedLanguage(target.Language) {
	case "", "go":
		return goTargetFindings(env, target), buildGoImportGraph(env, target)
	case "typescript", "javascript", "ts", "tsx", "js", "jsx":
		return typeScriptTargetFindings(ctx, env, target), buildTypeScriptImportGraph(env, target)
	case "python", "py":
		graph := buildPythonImportGraph(env, target)
		return pythonTargetFindings(env, target, graph), moduleGraphFromPython(graph)
	case "rust", "rs":
		return rustTargetFindings(env, target), buildRustImportGraph(env, target)
	case "java":
		return nil, buildJavaImportGraph(env, target)
	case "c++", "cpp", "cxx", "cc":
		return cppTargetFindings(env, target), buildCPPImportGraph(env, target)
	default:
		return nil, nil
	}
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
