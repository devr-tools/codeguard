// Package reliability implements production reliability checks: bounded
// outbound calls, cancellation propagation, bounded work, cleanup, and
// recoverable error handling.
package reliability

import (
	"context"

	"github.com/devr-tools/codeguard/internal/codeguard/checks/support"
	"github.com/devr-tools/codeguard/internal/codeguard/core"
)

func Run(ctx context.Context, env support.Context) core.SectionResult {
	return support.RunTargetSection(ctx, env, "reliability", "Reliability", reliabilityTargetFindings)
}

func reliabilityTargetFindings(_ context.Context, env support.Context, target core.TargetConfig) []core.Finding {
	switch support.NormalizedLanguage(target.Language) {
	case "go", "golang":
		return support.ScanGoFiles(env, target, "reliability", func(file string, data []byte) []core.Finding {
			return goFindingsForFile(env, file, data)
		})
	case "python", "py":
		return support.ScanPythonFiles(env, target, "reliability", func(file string, data []byte) []core.Finding {
			return pythonFindingsForFile(env, file, data)
		})
	case "typescript", "javascript", "ts", "tsx", "js", "jsx":
		return typeScriptTargetFindings(env, target)
	case "c++", "cpp", "cxx", "cc":
		return support.ScanCPPFiles(env, target, "reliability", func(file string, data []byte) []core.Finding {
			return cppFindingsForFile(env, file, data)
		})
	default:
		return nil
	}
}

func enabled(toggle *bool) bool {
	return toggle == nil || *toggle
}
