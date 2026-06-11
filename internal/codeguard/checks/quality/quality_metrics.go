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

func parsedFunctionMetrics(functions []support.ParsedFunction, countParams func(string) int, complexityFn func(string) int) []functionMetrics {
	metrics := make([]functionMetrics, 0, len(functions))
	for _, fn := range functions {
		metrics = append(metrics, functionMetrics{
			Name:       fn.Name,
			StartLine:  fn.StartLine,
			Length:     max(1, fn.EndLine-fn.StartLine+1),
			Params:     countParams(fn.Parameters),
			Complexity: complexityFn(fn.Body),
		})
	}
	return metrics
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
	count := 0
	for range splitTopLevelDelimited(signature) {
		count++
	}
	return count
}

func splitTopLevelDelimited(signature string) []string {
	signature = strings.TrimSpace(signature)
	if signature == "" {
		return nil
	}
	parts := make([]string, 0)
	start := 0
	state := delimiterState{}
	for idx := 0; idx < len(signature); idx++ {
		ch := signature[idx]
		if state.inString != 0 {
			if shouldSkipDelimitedStringByte(signature, idx, &state) {
				idx++
			}
			continue
		}
		if ch == ',' && state.atTopLevel() {
			parts = appendDelimitedPart(parts, signature[start:idx])
			start = idx + 1
			continue
		}
		state.advance(ch)
	}
	return appendDelimitedPart(parts, signature[start:])
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
