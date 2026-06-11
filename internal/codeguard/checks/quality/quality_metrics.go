package quality

import (
	"fmt"
	"strings"

	"github.com/devr-tools/codeguard/internal/codeguard/checks/support"
	"github.com/devr-tools/codeguard/internal/codeguard/core"
)

type functionMetrics struct {
	Name       string
	StartLine  int
	Length     int
	Params     int
	Complexity int
}

func fileLengthFinding(env support.Context, file string, data []byte) []core.Finding {
	lineCount := env.CountLines(data)
	if lineCount <= env.Config.Checks.QualityRules.MaxFileLines {
		return nil
	}
	return []core.Finding{env.NewFinding(support.FindingInput{
		RuleID:  "quality.max-file-lines",
		Level:   "warn",
		Path:    file,
		Line:    lineCount,
		Column:  1,
		Message: fmt.Sprintf("file has %d lines; max is %d", lineCount, env.Config.Checks.QualityRules.MaxFileLines),
	})}
}

func maintainabilityFindings(env support.Context, file string, fn functionMetrics) []core.Finding {
	findings := make([]core.Finding, 0, 3)
	if fn.Length > env.Config.Checks.QualityRules.MaxFunctionLines {
		findings = append(findings, env.NewFinding(support.FindingInput{
			RuleID:  "quality.max-function-lines",
			Level:   "warn",
			Path:    file,
			Line:    fn.StartLine,
			Column:  1,
			Message: fmt.Sprintf("function %s has %d lines; max is %d", fn.Name, fn.Length, env.Config.Checks.QualityRules.MaxFunctionLines),
		}))
	}
	if fn.Params > env.Config.Checks.QualityRules.MaxParameters {
		findings = append(findings, env.NewFinding(support.FindingInput{
			RuleID:  "quality.max-parameters",
			Level:   "warn",
			Path:    file,
			Line:    fn.StartLine,
			Column:  1,
			Message: fmt.Sprintf("function %s has %d parameters; max is %d", fn.Name, fn.Params, env.Config.Checks.QualityRules.MaxParameters),
		}))
	}
	if fn.Complexity > env.Config.Checks.QualityRules.MaxCyclomaticComplexity {
		findings = append(findings, env.NewFinding(support.FindingInput{
			RuleID:  "quality.cyclomatic-complexity",
			Level:   "warn",
			Path:    file,
			Line:    fn.StartLine,
			Column:  1,
			Message: fmt.Sprintf("function %s has cyclomatic complexity %d; max is %d", fn.Name, fn.Complexity, env.Config.Checks.QualityRules.MaxCyclomaticComplexity),
		}))
	}
	return findings
}

func countParameters(signature string) int {
	signature = strings.TrimSpace(signature)
	if signature == "" {
		return 0
	}
	parts := strings.Split(signature, ",")
	count := 0
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		count++
	}
	return count
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}
