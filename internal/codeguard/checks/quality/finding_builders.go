package quality

import (
	"github.com/devr-tools/codeguard/internal/codeguard/checks/support"
	"github.com/devr-tools/codeguard/internal/codeguard/core"
)

// newQualityWarnFinding builds a warn-level quality finding through the typed
// support.NewWarnFinding builder. Rules with a known precision profile pick up
// their confidence from aiRuleConfidence when the caller leaves Confidence
// empty; everything else stays at the unspecified/medium default.
func newQualityWarnFinding(env support.Context, input support.WarnFindingInput) core.Finding {
	if input.Confidence == "" {
		input.Confidence = aiRuleConfidence[input.RuleID]
	}
	return support.NewWarnFinding(env, input)
}
