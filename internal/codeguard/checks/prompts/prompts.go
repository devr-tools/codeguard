package prompts

import (
	"context"

	"github.com/devr-tools/codeguard/internal/codeguard/checks/support"
	"github.com/devr-tools/codeguard/internal/codeguard/core"
)

func Run(_ context.Context, env support.Context) core.SectionResult {
	findings := make([]core.Finding, 0)
	for _, target := range env.Config.Targets {
		findings = append(findings, env.ScanTargetFiles(target, "prompts", func(rel string) bool {
			return isGovernedPromptFile(env, rel)
		}, func(file string, data []byte) []core.Finding {
			return findingsForFile(env, file, data)
		})...)
	}
	return env.FinalizeSection("prompts", "AI Prompts", findings)
}

func findingsForFile(env support.Context, file string, data []byte) []core.Finding {
	findings := basePromptFindings(env, file, data)
	findings = append(findings, agentConfigFindings(env, file, data)...)
	findings = append(findings, mcpConfigFindings(env, file, data)...)
	return findings
}
