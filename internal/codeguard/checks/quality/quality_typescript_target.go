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
	results, ok, err := support.AnalyzeTypeScriptTargetForContext(ctx, env, target)
	if err == nil && ok {
		findings := support.FindingsFromInputs(env, qualityTypeScriptTargetExtract(results))
		findings = append(findings, env.ScanTargetFiles(target, "quality-typescript-file-length", isTypeScriptLikeFile, func(file string, data []byte) []core.Finding {
			return fileLengthFindingWithSignals(env, file, data, findings)
		})...)
		findings = append(findings, env.ScanTargetFiles(target, "quality-typescript-ai", isTypeScriptLikeFile, func(file string, data []byte) []core.Finding {
			return typeScriptAIOnlyFindingsForFile(env, file, data)
		})...)
		return findings
	}
	return support.TypeScriptTargetFindings(ctx, env, target, support.TypeScriptTargetScan{
		SectionID: "quality",
		Extract:   qualityTypeScriptTargetExtract,
		Include:   isTypeScriptLikeFile,
		Evaluator: func(file string, data []byte) []core.Finding {
			return typeScriptFindingsForFile(env, file, data)
		},
	})
}
