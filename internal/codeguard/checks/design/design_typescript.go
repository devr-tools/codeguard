package design

import (
	"context"
	"fmt"
	"strings"

	"github.com/devr-tools/codeguard/internal/codeguard/checks/support"
	"github.com/devr-tools/codeguard/internal/codeguard/core"
)

func typeScriptTargetFindingsImpl(ctx context.Context, env support.Context, target core.TargetConfig) []core.Finding {
	return support.TypeScriptTargetFindings(ctx, env, target, "design", func(results support.TypeScriptSemanticResults) []support.FindingInput {
		return results.Design
	}, isTypeScriptLikeFile, func(file string, data []byte) []core.Finding {
		return typeScriptFindingsForFile(env, file, data)
	})
}

func typeScriptFindingsForFile(env support.Context, file string, data []byte) []core.Finding {
	findings := forbiddenTypeScriptModuleNameFindings(env, file)
	lines := strings.Split(strings.ReplaceAll(string(data), "\r\n", "\n"), "\n")

	depth := 0
	var active *typeScriptBlock

	for idx, line := range lines {
		active = nextTypeScriptBlock(active, line, idx+1)
		countTypeScriptBlockMember(active, depth, line)
		depth += braceDelta(line)
		openTypeScriptBlock(active, depth, line)
		findings, active = closeTypeScriptBlock(findings, env, file, active, depth)
	}

	if active != nil && !active.waiting {
		findings = append(findings, typeScriptBlockFindings(env, file, *active)...)
	}

	return findings
}

func nextTypeScriptBlock(active *typeScriptBlock, line string, lineNo int) *typeScriptBlock {
	if active != nil {
		return active
	}
	return newTypeScriptBlock(line, lineNo)
}

func countTypeScriptBlockMember(active *typeScriptBlock, depth int, line string) {
	if active == nil || active.waiting || depth != active.bodyDepth {
		return
	}
	switch active.kind {
	case typeScriptBlockClass:
		if name, ok := typeScriptMethodName(line); ok && name != "constructor" {
			active.count++
		}
	case typeScriptBlockInterface, typeScriptBlockObjectType:
		if isTypeScriptMemberLine(line) {
			active.count++
		}
	}
}

func openTypeScriptBlock(active *typeScriptBlock, depth int, line string) {
	if active == nil || !active.waiting || !strings.Contains(line, "{") {
		return
	}
	active.waiting = false
	active.bodyDepth = depth
}

func closeTypeScriptBlock(findings []core.Finding, env support.Context, file string, active *typeScriptBlock, depth int) ([]core.Finding, *typeScriptBlock) {
	if active == nil || active.waiting || depth >= active.bodyDepth {
		return findings, active
	}
	findings = append(findings, typeScriptBlockFindings(env, file, *active)...)
	return findings, nil
}

func typeScriptBlockFindings(env support.Context, file string, block typeScriptBlock) []core.Finding {
	switch block.kind {
	case typeScriptBlockClass:
		if block.count > env.Config.Checks.DesignRules.MaxMethodsPerType {
			return []core.Finding{env.NewFinding(support.FindingInput{
				RuleID:  "design.typescript.max-methods-per-type",
				Level:   "warn",
				Path:    file,
				Line:    block.line,
				Column:  1,
				Message: fmt.Sprintf("class %s has %d methods; max is %d", block.name, block.count, env.Config.Checks.DesignRules.MaxMethodsPerType),
			})}
		}
	case typeScriptBlockInterface, typeScriptBlockObjectType:
		if block.count > env.Config.Checks.DesignRules.MaxInterfaceMethods {
			kind := "interface"
			if block.kind == typeScriptBlockObjectType {
				kind = "type"
			}
			return []core.Finding{env.NewFinding(support.FindingInput{
				RuleID:  "design.typescript.max-interface-members",
				Level:   "warn",
				Path:    file,
				Line:    block.line,
				Column:  1,
				Message: fmt.Sprintf("%s %s has %d members; max is %d", kind, block.name, block.count, env.Config.Checks.DesignRules.MaxInterfaceMethods),
			})}
		}
	}
	return nil
}
