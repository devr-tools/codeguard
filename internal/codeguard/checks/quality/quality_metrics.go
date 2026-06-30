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

// parsedFunctionMetrics converts structured-parser functions into the shared
// functionMetrics shape, computing complexity from each masked body.
func parsedFunctionMetrics(file *support.ParsedFile, complexityFn func(string) int) []functionMetrics {
	parsed := file.AllFunctions()
	functions := make([]functionMetrics, 0, len(parsed))
	for _, fn := range parsed {
		functions = append(functions, functionMetrics{
			Name:       fn.Name,
			StartLine:  fn.StartLine,
			Length:     fn.LineCount(),
			Params:     len(fn.Params),
			Complexity: complexityFn(maskedFunctionBody(fn)),
		})
	}
	return functions
}

func fileLengthFindingWithSignals(env support.Context, file string, data []byte, findings []core.Finding) []core.Finding {
	lineCount := env.CountLines(data)
	if lineCount <= env.Config.Checks.QualityRules.MaxFileLines {
		return nil
	}
	level := "warn"
	message := fmt.Sprintf("file has %d lines; max is %d", lineCount, env.Config.Checks.QualityRules.MaxFileLines)
	if fileHasComplexityFinding(findings, file) {
		level = "fail"
		message = fmt.Sprintf("file has %d lines; max is %d, and the file also exceeds cyclomatic complexity limits", lineCount, env.Config.Checks.QualityRules.MaxFileLines)
	}
	return []core.Finding{env.NewFinding(support.FindingInput{
		RuleID:  "quality.max-file-lines",
		Level:   level,
		Path:    file,
		Line:    lineCount,
		Column:  1,
		Message: message,
	})}
}

func fileHasComplexityFinding(findings []core.Finding, file string) bool {
	for _, finding := range findings {
		if finding.Path == file && finding.RuleID == "quality.cyclomatic-complexity" {
			return true
		}
	}
	return false
}

func maintainabilityFindings(env support.Context, file string, fn functionMetrics) []core.Finding {
	findings := make([]core.Finding, 0, 3)
	if fn.Length > env.Config.Checks.QualityRules.MaxFunctionLines {
		findings = append(findings, warnFinding(env, "quality.max-function-lines", file, fn.StartLine, 1,
			fmt.Sprintf("function %s has %d lines; max is %d", fn.Name, fn.Length, env.Config.Checks.QualityRules.MaxFunctionLines)))
	}
	if fn.Params > env.Config.Checks.QualityRules.MaxParameters {
		findings = append(findings, warnFinding(env, "quality.max-parameters", file, fn.StartLine, 1,
			fmt.Sprintf("function %s has %d parameters; max is %d", fn.Name, fn.Params, env.Config.Checks.QualityRules.MaxParameters)))
	}
	if fn.Complexity > env.Config.Checks.QualityRules.MaxCyclomaticComplexity {
		findings = append(findings, warnFinding(env, "quality.cyclomatic-complexity", file, fn.StartLine, 1,
			fmt.Sprintf("function %s has cyclomatic complexity %d; max is %d", fn.Name, fn.Complexity, env.Config.Checks.QualityRules.MaxCyclomaticComplexity)))
	}
	return findings
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
