package quality

import (
	"context"

	"github.com/devr-tools/codeguard/internal/codeguard/checks/support"
	"github.com/devr-tools/codeguard/internal/codeguard/core"
)

var qualityTypeScriptTargetExtract = func(results support.TypeScriptSemanticResults) []support.FindingInput {
	return results.Quality
}

func typeScriptTargetFindings(ctx context.Context, env support.Context, target core.TargetConfig) []core.Finding {
	return support.TypeScriptTargetFindings(ctx, env, target, support.TypeScriptTargetScan{
		SectionID: "quality",
		Extract:   qualityTypeScriptTargetExtract,
		Include:   isTypeScriptLikeFile,
		Evaluator: func(file string, data []byte) []core.Finding {
			return typeScriptFindingsForFile(env, file, data)
		},
	})
}
