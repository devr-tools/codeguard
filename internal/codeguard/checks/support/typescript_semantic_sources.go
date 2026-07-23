package support

import (
	"context"

	"github.com/devr-tools/codeguard/internal/codeguard/core"
)

// AnalyzeTypeScriptTargetForContext uses the runner's already-filtered corpus
// as the TypeScript program roots. Calling TypeScript's recursive discovery
// directly would bypass Codeguard's target exclusions.
func AnalyzeTypeScriptTargetForContext(ctx context.Context, env Context, target core.TargetConfig) (TypeScriptSemanticResults, bool, error) {
	return analyzeTypeScriptTarget(ctx, target, env.Config, TypeScriptTargetSourceFiles(env, target))
}

// TypeScriptTargetSourceFiles filters the shared corpus list for semantic
// analysis. A nil result retains the direct-call fallback for unit consumers
// that do not construct a runner Context.
func TypeScriptTargetSourceFiles(env Context, target core.TargetConfig) []string {
	if env.ListTargetFiles == nil {
		return nil
	}
	files, err := env.ListTargetFiles(target)
	if err != nil {
		return nil
	}
	sourceFiles := make([]string, 0, len(files))
	for _, file := range files {
		if IsTypeScriptLikeFile(file) {
			sourceFiles = append(sourceFiles, file)
		}
	}
	return sourceFiles
}
