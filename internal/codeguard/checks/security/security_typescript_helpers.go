package security

import (
	"fmt"

	"github.com/devr-tools/codeguard/internal/codeguard/checks/support"
	"github.com/devr-tools/codeguard/internal/codeguard/core"
)

func newTypeScriptSecurityFinding(ctx typeScriptScanContext, ruleID string, line int, message string) core.Finding {
	return ctx.env.NewFinding(support.FindingInput{
		RuleID:  ruleID,
		Level:   "warn",
		Path:    ctx.file,
		Line:    line,
		Column:  1,
		Message: message,
	})
}

func securityRuleID(path string, suffix string) string {
	return support.RuleIDForScript(path, "security.typescript."+suffix, "security.javascript."+suffix)
}

func dedupeTypeScriptFindings(findings []core.Finding) []core.Finding {
	if len(findings) <= 1 {
		return findings
	}
	seen := make(map[string]struct{}, len(findings))
	deduped := make([]core.Finding, 0, len(findings))
	for _, finding := range findings {
		key := finding.RuleID + "|" + finding.Path + "|" + fmt.Sprintf("%d", finding.Line)
		if _, exists := seen[key]; exists {
			continue
		}
		seen[key] = struct{}{}
		deduped = append(deduped, finding)
	}
	return deduped
}
