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
		findings := support.FindingsFromInputs(env, filterTypeScriptQualitySemanticInputs(qualityTypeScriptTargetExtract(results)))
		findings = append(findings, env.ScanTargetFiles(target, "quality-typescript-file-length", isTypeScriptLikeFile, func(file string, data []byte) []core.Finding {
			return fileLengthFindingWithSignals(env, file, data, findings)
		})...)
		findings = append(findings, env.ScanTargetFiles(target, "quality-typescript-ai", isTypeScriptLikeFile, func(file string, data []byte) []core.Finding {
			return typeScriptAIOnlyFindingsForFile(env, file, data)
		})...)
		if localPrecisionEnabled(env) {
			findings = append(findings, env.ScanTargetFiles(target, "quality-typescript-local-precision", isTypeScriptLikeFile, func(file string, data []byte) []core.Finding {
				parsed := support.ParseCLike(string(data), support.CLikeTypeScript)
				localFindings := parsedPrecisionFindings(env, file, parsed)
				localFindings = append(localFindings, parsedStructuralSmellFindings(env, file, parsed)...)
				return localFindings
			})...)
		}
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

func filterTypeScriptQualitySemanticInputs(inputs []support.FindingInput) []support.FindingInput {
	out := inputs[:0]
	for _, input := range inputs {
		if isScriptTestOrHelperFile(input.Path) &&
			(input.RuleID == "quality.typescript.non-null-assertion" ||
				input.RuleID == "quality.javascript.non-null-assertion" ||
				input.RuleID == "quality.typescript.double-assertion" ||
				input.RuleID == "quality.javascript.double-assertion") {
			continue
		}
		out = append(out, input)
	}
	return out
}
