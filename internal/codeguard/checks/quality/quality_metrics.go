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
	depthParen, depthBracket, depthBrace, depthAngle := 0, 0, 0, 0
	inString := byte(0)
	for idx := 0; idx < len(signature); idx++ {
		ch := signature[idx]
		if inString != 0 {
			if ch == '\\' && idx+1 < len(signature) {
				idx++
				continue
			}
			if ch == inString {
				inString = 0
			}
			continue
		}
		switch ch {
		case '"', '\'':
			inString = ch
		case '(':
			depthParen++
		case ')':
			if depthParen > 0 {
				depthParen--
			}
		case '[':
			depthBracket++
		case ']':
			if depthBracket > 0 {
				depthBracket--
			}
		case '{':
			depthBrace++
		case '}':
			if depthBrace > 0 {
				depthBrace--
			}
		case '<':
			depthAngle++
		case '>':
			if depthAngle > 0 {
				depthAngle--
			}
		case ',':
			if depthParen == 0 && depthBracket == 0 && depthBrace == 0 && depthAngle == 0 {
				part := strings.TrimSpace(signature[start:idx])
				if part != "" {
					parts = append(parts, part)
				}
				start = idx + 1
			}
		}
	}
	if tail := strings.TrimSpace(signature[start:]); tail != "" {
		parts = append(parts, tail)
	}
	return parts
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
