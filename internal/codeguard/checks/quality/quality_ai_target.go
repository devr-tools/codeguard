package quality

import (
	"github.com/devr-tools/codeguard/internal/codeguard/checks/support"
	"github.com/devr-tools/codeguard/internal/codeguard/core"
)

func aiTargetFindings(env support.Context, target core.TargetConfig) []core.Finding {
	switch support.NormalizedLanguage(target.Language) {
	case "", "go":
		return goAITargetFindings(env, target)
	case "python", "py":
		return pythonAITargetFindings(env, target)
	case "typescript", "javascript", "ts", "tsx", "js", "jsx":
		return typeScriptAITargetFindings(env, target)
	default:
		return nil
	}
}
