package quality

import (
	"context"

	"github.com/devr-tools/codeguard/internal/codeguard/checks/support"
	"github.com/devr-tools/codeguard/internal/codeguard/core"
)

func typeScriptTargetFindings(ctx context.Context, env support.Context, target core.TargetConfig) []core.Finding {
	results, ok, err := support.AnalyzeTypeScriptTarget(ctx, target, env.Config)
	if err == nil && ok {
		return semanticFindings(env, results.Quality)
	}
	return env.ScanTargetFiles(target, "quality", isTypeScriptLikeFile, func(file string, data []byte) []core.Finding {
		return typeScriptFindingsForFile(env, file, data)
	})
}

func semanticFindings(env support.Context, inputs []support.FindingInput) []core.Finding {
	findings := make([]core.Finding, 0, len(inputs))
	for _, input := range inputs {
		findings = append(findings, env.NewFinding(input))
	}
	return findings
}
