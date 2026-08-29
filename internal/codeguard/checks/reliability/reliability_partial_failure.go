package reliability

import (
	"regexp"
	"strings"

	"github.com/devr-tools/codeguard/internal/codeguard/checks/support"
	"github.com/devr-tools/codeguard/internal/codeguard/core"
)

var (
	partialFailureLoopPattern      = regexp.MustCompile(`(?i)\b(for|while)\b|\.forEach\s*\(|\.map\s*\(`)
	partialFailureLogPattern       = regexp.MustCompile(`(?i)\b(log|logger|logging|console\.(?:log|warn|error)|print|fprintf|cerr|cout)\b.*\b(err|error|exception|fail(?:ed|ure)?)\b`)
	partialFailureContinuePattern  = regexp.MustCompile(`^\s*continue\s*;?\s*(?://.*)?$`)
	partialFailureSuccessReturn    = regexp.MustCompile(`(?i)^\s*return(?:\s+(?:nil|none|null|true|0|\{\}))?\s*;?\s*$`)
	partialFailurePropagatePattern = regexp.MustCompile(`(?i)\b(return\s+err|return\s+error|raise\b|throw\b)`)
	partialFailureGoErrorFunc      = regexp.MustCompile(`^\s*func\s+(?:\([^)]*\)\s*)?[A-Za-z_]\w*\s*\([^)]*\)\s*(?:error|\(\s*error\s*\))\s*\{`)
	partialFailureScriptVoidFunc   = regexp.MustCompile(`\bfunction\s+[A-Za-z_$][\w$]*\s*\([^)]*\)\s*:\s*(?:Promise\s*<\s*void\s*>|void)\s*\{`)
	partialFailureCPPVoidFunc      = regexp.MustCompile(`^\s*(?:[\w:<>]+\s+)*void\s+[A-Za-z_]\w*\s*\([^)]*\)\s*(?:const\s*)?\{`)
	partialFailurePythonDef        = regexp.MustCompile(`^\s*def\s+[A-Za-z_]\w*\s*\([^)]*\)\s*(?:->\s*None\s*)?:`)
	partialFailureValuedReturn     = regexp.MustCompile(`(?i)^\s*return\s+(.+?)\s*;?\s*$`)
)

func partialFailureHiddenFindings(env support.Context, file string, data []byte) []core.Finding {
	if !enabled(env.Config.Checks.ReliabilityRules.DetectPartialFailureHidden) {
		return nil
	}
	lines := strings.Split(strings.ReplaceAll(string(data), "\r\n", "\n"), "\n")
	contracts := partialFailureNonResultFunctionLines(lines)
	loopDepth := 0
	pending := 0
	findings := make([]core.Finding, 0, 1)
	for idx, line := range lines {
		lineNo := idx + 1
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			continue
		}
		if partialFailureLoopPattern.MatchString(line) {
			loopDepth = 8
		} else if loopDepth > 0 {
			loopDepth--
		}
		if loopDepth > 0 && partialFailureLogPattern.MatchString(line) && !partialFailurePropagatePattern.MatchString(line) {
			pending = lineNo
		}
		if pending > 0 && partialFailureContinuePattern.MatchString(trimmed) {
			if !contracts[pending] {
				findings = append(findings, partialFailureFinding(env, file, pending, "logged failure is skipped and batch processing continues without surfacing partial failure"))
			}
			pending = 0
			continue
		}
		if pending > 0 && lineNo <= pending+12 && partialFailureSuccessReturn.MatchString(trimmed) {
			if !contracts[pending] {
				findings = append(findings, partialFailureFinding(env, file, pending, "logged failure is followed by a success return, hiding partial failure from callers"))
			}
			pending = 0
			continue
		}
		if pending > 0 && lineNo > pending+12 {
			pending = 0
		}
	}
	return support.DedupeFindings(findings, func(finding core.Finding) string {
		return finding.RuleID + "|" + finding.Path + "|" + finding.Message
	})
}

func partialFailureNonResultFunctionLines(lines []string) map[int]bool {
	out := map[int]bool{}
	for start := 0; start < len(lines); start++ {
		line := lines[start]
		switch {
		case partialFailureGoErrorFunc.MatchString(line) || partialFailureScriptVoidFunc.MatchString(line) || partialFailureCPPVoidFunc.MatchString(line):
			end := partialFailureBraceFunctionEnd(lines, start)
			for idx := start + 1; idx <= end && idx < len(lines); idx++ {
				out[idx+1] = true
			}
			start = end
		case partialFailurePythonDef.MatchString(line):
			end := partialFailurePythonFunctionEnd(lines, start)
			if partialFailurePythonReturnsOnlyNone(lines[start+1 : end+1]) {
				for idx := start + 1; idx <= end && idx < len(lines); idx++ {
					out[idx+1] = true
				}
			}
			start = end
		}
	}
	return out
}

func partialFailureBraceFunctionEnd(lines []string, start int) int {
	depth := 0
	seenOpen := false
	for idx := start; idx < len(lines); idx++ {
		line := lines[idx]
		depth += strings.Count(line, "{")
		if strings.Contains(line, "{") {
			seenOpen = true
		}
		depth -= strings.Count(line, "}")
		if seenOpen && depth <= 0 {
			return idx
		}
	}
	return len(lines) - 1
}

func partialFailurePythonFunctionEnd(lines []string, start int) int {
	baseIndent := len(lines[start]) - len(strings.TrimLeft(lines[start], " \t"))
	for idx := start + 1; idx < len(lines); idx++ {
		trimmed := strings.TrimSpace(lines[idx])
		if trimmed == "" {
			continue
		}
		indent := len(lines[idx]) - len(strings.TrimLeft(lines[idx], " \t"))
		if indent <= baseIndent {
			return idx - 1
		}
	}
	return len(lines) - 1
}

func partialFailurePythonReturnsOnlyNone(lines []string) bool {
	for _, line := range lines {
		match := partialFailureValuedReturn.FindStringSubmatch(strings.TrimSpace(line))
		if len(match) != 2 {
			continue
		}
		value := strings.TrimSpace(strings.TrimSuffix(match[1], ";"))
		if !strings.EqualFold(value, "none") {
			return false
		}
	}
	return true
}

func partialFailureFinding(env support.Context, file string, line int, message string) core.Finding {
	return newFinding(env, "reliability.partial-failure-hidden", "fail", file, line, 1, message, "medium", "failure_mode", "partial-failure-hidden")
}
