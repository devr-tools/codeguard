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
func warnFinding(env support.Context, ruleID string, file string, line int, column int, message string) core.Finding {
	return env.NewFinding(support.FindingInput{
		RuleID:     ruleID,
		Level:      "warn",
		Path:       file,
		Line:       line,
		Column:     column,
		Message:    message,
		Confidence: aiRuleConfidence[ruleID],
	})
}
