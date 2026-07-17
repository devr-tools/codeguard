package design

import (
	"fmt"
	"strings"

	"github.com/devr-tools/codeguard/internal/codeguard/checks/support"
	"github.com/devr-tools/codeguard/internal/codeguard/core"
)

func pythonStructuralFindings(env support.Context, target core.TargetConfig) []core.Finding {
	return env.ScanTargetFiles(target, "design", func(rel string) bool {
		return strings.EqualFold(".py", filepathExt(rel))
	}, func(file string, data []byte) []core.Finding {
		return pythonStructuralFindingsForFile(env, file, data)
	})
}

func pythonStructuralFindingsForFile(env support.Context, file string, data []byte) []core.Finding {
	blocks := pythonTypeBlocks(string(data))
	findings := make([]core.Finding, 0, len(blocks))
	for _, block := range blocks {
		switch block.kind {
		case pythonTypeBlockClass:
			if block.memberCount <= env.Config.Checks.DesignRules.MaxMethodsPerType {
				continue
			}
			findings = append(findings, env.NewFinding(support.FindingInput{
				RuleID:  "design.python.max-methods-per-type",
				Level:   "warn",
				Path:    file,
				Line:    block.line,
				Column:  1,
				Message: fmt.Sprintf("class %s has %d methods; max is %d", block.name, block.memberCount, env.Config.Checks.DesignRules.MaxMethodsPerType),
			}))
		case pythonTypeBlockProtocol:
			if block.memberCount <= env.Config.Checks.DesignRules.MaxInterfaceMethods {
				continue
			}
			findings = append(findings, env.NewFinding(support.FindingInput{
				RuleID:  "design.python.max-protocol-members",
				Level:   "warn",
				Path:    file,
				Line:    block.line,
				Column:  1,
				Message: fmt.Sprintf("protocol %s has %d members; max is %d", block.name, block.memberCount, env.Config.Checks.DesignRules.MaxInterfaceMethods),
			}))
		}
	}
	return findings
}
