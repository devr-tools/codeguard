package agentcontext

import (
	"fmt"

	"github.com/devr-tools/codeguard/internal/codeguard/checks/support"
	"github.com/devr-tools/codeguard/internal/codeguard/core"
)

// defaultMaxAgentDocLines is the documented default budget for a single agent
// instruction file. Agent docs are loaded into every session verbatim, so a
// doc that sprawls consumes the very context window it exists to conserve.
const defaultMaxAgentDocLines = 600

// maxAgentDocLinesBudget resolves the configured agent-doc budget, falling
// back to the documented default for configs assembled without ApplyDefaults.
func maxAgentDocLinesBudget(rules core.ContextRulesConfig) int {
	if rules.MaxAgentDocLines > 0 {
		return rules.MaxAgentDocLines
	}
	return defaultMaxAgentDocLines
}

// oversizedAgentDocFindings reports agent instruction files whose line count
// exceeds the agent-doc budget. Only the recognized agent docs are measured;
// the README and linked reference material are free to be long.
func oversizedAgentDocFindings(env support.Context, root string, agentDocs []string, maxLines int) []core.Finding {
	findings := make([]core.Finding, 0)
	for _, rel := range agentDocs {
		data, ok := readCappedDocFile(root, rel)
		if !ok {
			continue
		}
		lines := env.CountLines(data)
		if lines <= maxLines {
			continue
		}
		findings = append(findings, env.NewFinding(support.FindingInput{
			RuleID: "context.oversized-agent-doc",
			Level:  "warn",
			Path:   rel,
			Line:   1,
			Column: 1,
			Message: fmt.Sprintf("agent instruction file has %d lines, exceeding the %d-line agent doc budget; "+
				"a doc this large consumes the context window it exists to save — keep the instruction file to essentials and move reference material into linked docs", lines, maxLines),
		}))
	}
	return findings
}
