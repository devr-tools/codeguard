package prompts

import (
	"context"

	"github.com/devr-tools/codeguard/internal/codeguard/checks/support"
	"github.com/devr-tools/codeguard/internal/codeguard/core"
)

// Run is the prompts section entrypoint; file discovery is heuristic, so
// path and extension config must widen it rather than code changes here.
func Run(ctx context.Context, env support.Context) core.SectionResult {
	return support.RunTargetSection(ctx, env, "prompts", "AI Prompts", promptTargetFindings)
}

func promptTargetFindings(_ context.Context, env support.Context, target core.TargetConfig) []core.Finding {
	return env.ScanTargetFiles(target, "prompts", func(rel string) bool {
		return isGovernedPromptFile(env, rel)
	}, func(file string, data []byte) []core.Finding {
		return findingsForFile(env, file, data)
	})
}

func findingsForFile(env support.Context, file string, data []byte) []core.Finding {
	findings := basePromptFindings(env, file, data)
	findings = append(findings, agentConfigFindings(env, file, data)...)
	findings = append(findings, mcpConfigFindings(env, file, data)...)
	return findings
}
