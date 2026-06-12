package quality

import (
	"regexp"
	"strings"
)

var (
	aiNarrativeCommentPattern = regexp.MustCompile(`(?i)^(initialize|create|set|get|call|return|check|convert|update|build|iterate|loop|run|assign|store)\b`)
	aiRationalePattern        = regexp.MustCompile(`(?i)\b(because|so that|why|ensure|ensures|avoid|must|needed|required|reason|safely|in order to)\b`)
	aiEmptyCatchPattern       = regexp.MustCompile(`(?s)\bcatch\s*(?:\([^)]*\))?\s*\{\s*(?:(?://[^\n]*\n)|(?:/\*.*?\*/\s*))*\}`)
	aiPythonPassExceptPattern = regexp.MustCompile(`(?m)^\s*except(?:\s+[^\n:]+)?\s*:\s*(?:#.*)?\n\s*(pass|\.\.\.)\b`)
)

var aiSlopRuleWeights = map[string]int{
	"quality.ai.swallowed-error":     4,
	"quality.ai.narrative-comment":   1,
	"quality.ai.hallucinated-import": 5,
	"quality.ai.dead-code":           3,
	"quality.ai.over-mocked-test":    3,
	"quality.ai.local-idiom-drift":   2,
	"quality.ai.provenance-policy":   2,
}

func artifactSafeID(value string) string {
	replacer := strings.NewReplacer(" ", "-", "/", "-", "\\", "-", "_", "-")
	out := strings.Trim(replacer.Replace(strings.ToLower(strings.TrimSpace(value))), "-")
	if out == "" {
		return "target"
	}
	return out
}

func isNarrativeComment(text string) bool {
	trimmed := strings.TrimSpace(text)
	if trimmed == "" || aiRationalePattern.MatchString(trimmed) || !aiNarrativeCommentPattern.MatchString(trimmed) {
		return false
	}
	words := strings.Fields(trimmed)
	return len(words) >= 2 && len(words) <= 10
}

func regexLineMatches(pattern *regexp.Regexp, source string) []int {
	indices := pattern.FindAllStringIndex(source, -1)
	lines := make([]int, 0, len(indices))
	seen := map[int]struct{}{}
	for _, idx := range indices {
		line := 1 + strings.Count(source[:idx[0]], "\n")
		if _, ok := seen[line]; ok {
			continue
		}
		seen[line] = struct{}{}
		lines = append(lines, line)
	}
	return lines
}

func extractScriptCommentText(line string) (string, bool) {
	trimmed := strings.TrimSpace(line)
	switch {
	case strings.HasPrefix(trimmed, "//"):
		return strings.TrimSpace(strings.TrimPrefix(trimmed, "//")), true
	case strings.HasPrefix(trimmed, "/*"):
		text := strings.TrimSpace(strings.TrimPrefix(trimmed, "/*"))
		text = strings.TrimSpace(strings.TrimSuffix(text, "*/"))
		return text, true
	case strings.HasPrefix(trimmed, "*"):
		return strings.TrimSpace(strings.TrimPrefix(trimmed, "*")), true
	default:
		return "", false
	}
}

func minInt(a int, b int) int {
	if a < b {
		return a
	}
	return b
}

func firstSegment(value string) string {
	parts := strings.Split(value, "/")
	if len(parts) == 0 {
		return ""
	}
	return parts[0]
}

func containsAny(source string, needles []string) bool {
	for _, needle := range needles {
		if strings.Contains(source, needle) {
			return true
		}
	}
	return false
}

func firstLineContaining(source string, needles []string) int {
	for idx, line := range strings.Split(source, "\n") {
		if containsAny(line, needles) {
			return idx + 1
		}
	}
	return 1
}

func scoreFindings(findings []string) int {
	total := 0
	for _, ruleID := range findings {
		total += aiSlopRuleWeights[ruleID]
	}
	return minInt(total*10, 100)
}
