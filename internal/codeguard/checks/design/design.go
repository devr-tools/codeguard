package design

import (
	"context"
	"fmt"
	"strings"

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
			graphs = append(graphs, targetModuleGraph{target: target, graph: graph})
		}
		findings = append(findings, commandFindings(ctx, env, target)...)
	}
	findings = append(findings, changeImpactFindings(env, graphs)...)
	return env.FinalizeSection("design", "Design Patterns", findings)
}

func targetLanguageFindings(ctx context.Context, env support.Context, target core.TargetConfig) ([]core.Finding, *moduleGraph) {
	switch normalizedLanguage(target.Language) {
	case "", "go":
		return goTargetFindings(env, target), buildGoImportGraph(env, target)
	case "typescript", "javascript", "ts", "tsx", "js", "jsx":
		return typeScriptTargetFindings(ctx, env, target), buildTypeScriptImportGraph(env, target)
	case "python", "py":
		graph := buildPythonImportGraph(env, target)
		return pythonTargetFindings(env, target, graph), moduleGraphFromPython(graph)
	case "rust", "rs":
		return nil, buildRustImportGraph(env, target)
	case "java":
		return nil, buildJavaImportGraph(env, target)
	default:
		return nil, nil
	}
}

func normalizedLanguage(language string) string {
	return strings.ToLower(strings.TrimSpace(language))
}

func typeScriptTargetFindings(ctx context.Context, env support.Context, target core.TargetConfig) []core.Finding {
	return typeScriptTargetFindingsImpl(ctx, env, target)
}

func commandFindings(ctx context.Context, env support.Context, target core.TargetConfig) []core.Finding {
	checks := env.Config.Checks.DesignRules.LanguageCommands[normalizedLanguage(target.Language)]
	findings := make([]core.Finding, 0, len(checks))
	for _, check := range checks {
		output, err := env.RunCommandCheck(ctx, target.Path, check)
		if err == nil {
			continue
		}
		findings = append(findings, env.NewFinding(support.FindingInput{
			RuleID:  "design.command-check",
			Level:   "fail",
			Message: commandFailureMessage(target, check, output, err),
		}))
	}
	return findings
}

func commandFailureMessage(target core.TargetConfig, check core.CommandCheckConfig, output string, err error) string {
	message := fmt.Sprintf("target %q design command %q failed", target.Name, check.Name)
	output = trimmedOutput(output)
	if output != "" {
		message += ": " + output
	} else if err != nil {
		message += ": " + err.Error()
	}
	return message
}

func trimmedOutput(output string) string {
	output = strings.TrimSpace(output)
	if output == "" {
		return ""
	}
	output = strings.Join(strings.Fields(output), " ")
	if len(output) > 240 {
		return output[:237] + "..."
	}
	return output
}
