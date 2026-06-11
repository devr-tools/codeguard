package quality

import (
	"context"

	"github.com/devr-tools/codeguard/internal/codeguard/checks/support"
	"github.com/devr-tools/codeguard/internal/codeguard/core"
)

func typeScriptTargetFindings(ctx context.Context, env support.Context, target core.TargetConfig) []core.Finding {
	return support.TypeScriptTargetFindings(ctx, env, target, "quality", func(results support.TypeScriptSemanticResults) []support.FindingInput {
		return results.Quality
	}, isTypeScriptLikeFile, func(file string, data []byte) []core.Finding {
		return typeScriptFindingsForFile(env, file, data)
	})
}
