package security

import (
	"context"

	"github.com/devr-tools/codeguard/internal/codeguard/checks/support"
	"github.com/devr-tools/codeguard/internal/codeguard/core"
)

func typeScriptTargetFindings(ctx context.Context, env support.Context, target core.TargetConfig) []core.Finding {
	return support.TypeScriptTargetFindings(ctx, env, target, "security", func(results support.TypeScriptSemanticResults) []support.FindingInput {
		return results.Security
	}, func(string) bool { return true }, func(file string, data []byte) []core.Finding {
		return findingsForFile(env, file, data)
	})
}
