package quality

import (
	"github.com/devr-tools/codeguard/internal/codeguard/checks/support"
	"github.com/devr-tools/codeguard/internal/codeguard/core"
)

// warnFinding builds a warn-level finding. The quality checks construct the
// same support.FindingInput literal (Level "warn", a path, line, column and
// message) at dozens of call sites; this helper collapses that boilerplate
// while preserving the exact field values, so findings output is unchanged.
// Rules with a known precision profile pick up their confidence from
// aiRuleConfidence; everything else stays at the unspecified/medium default.
func warnFinding(env support.Context, args ...any) core.Finding {
	ruleID := args[0].(string)
	return env.NewFinding(support.FindingInput{
		RuleID:     ruleID,
		Level:      "warn",
		Path:       args[1].(string),
		Line:       args[2].(int),
		Column:     args[3].(int),
		Message:    args[4].(string),
		Confidence: aiRuleConfidence[ruleID],
	})
}
