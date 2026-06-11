package design

import (
	"context"
	"strings"

	"github.com/devr-tools/codeguard/internal/codeguard/checks/support"
	"github.com/devr-tools/codeguard/internal/codeguard/core"
)

func Run(_ context.Context, env support.Context) core.SectionResult {
	findings := make([]core.Finding, 0)
	for _, target := range env.Config.Targets {
		switch normalizedLanguage(target.Language) {
		case "", "go":
			findings = append(findings, goTargetFindings(env, target)...)
		case "typescript", "javascript", "ts", "tsx", "js", "jsx":
			findings = append(findings, typeScriptTargetFindings(env, target)...)
		case "python", "py":
			findings = append(findings, pythonTargetFindings(env, target)...)
		}
	}
	return env.FinalizeSection("design", "Design Patterns", findings)
}

func normalizedLanguage(language string) string {
	return strings.ToLower(strings.TrimSpace(language))
}

func typeScriptTargetFindings(env support.Context, target core.TargetConfig) []core.Finding {
	return typeScriptTargetFindingsImpl(env, target)
}
