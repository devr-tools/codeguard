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
)

func partialFailureHiddenFindings(env support.Context, file string, data []byte) []core.Finding {
	if !enabled(env.Config.Checks.ReliabilityRules.DetectPartialFailureHidden) {
		return nil
	}
	lines := strings.Split(strings.ReplaceAll(string(data), "\r\n", "\n"), "\n")
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
			findings = append(findings, partialFailureFinding(env, file, pending, "logged failure is skipped and batch processing continues without surfacing partial failure"))
			pending = 0
			continue
		}
		if pending > 0 && lineNo <= pending+12 && partialFailureSuccessReturn.MatchString(trimmed) {
			findings = append(findings, partialFailureFinding(env, file, pending, "logged failure is followed by a success return, hiding partial failure from callers"))
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

func partialFailureFinding(env support.Context, file string, line int, message string) core.Finding {
	return newFinding(env, "reliability.partial-failure-hidden", "fail", file, line, 1, message, "medium", "failure_mode", "partial-failure-hidden")
}
