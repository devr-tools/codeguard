package quality

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/devr-tools/codeguard/internal/codeguard/checks/support"
	"github.com/devr-tools/codeguard/internal/codeguard/core"
)

// --- TypeScript/JavaScript: lexical unreachable statements ---

var (
	scriptTerminatorPattern    = regexp.MustCompile(`^(?:return\b[^;{}]*;|throw\b[^;{}]*;|break\s*;|continue\s*;|return;?$|break$|continue$)`)
	scriptBlockResumePattern   = regexp.MustCompile(`^(?:\}|case\b|default\s*:|else\b|catch\b|finally\b)`)
	scriptLocalFunctionPattern = regexp.MustCompile(`(?m)^[ \t]*(?:async[ \t]+)?function[ \t]+([A-Za-z_$][\w$]*)[ \t]*\(`)
)

// unreachableStatementFinding builds the shared dead-code finding emitted when
// a statement follows an unconditional block terminator.
func unreachableStatementFinding(env support.Context, file string, line int) core.Finding {
	return env.NewFinding(support.FindingInput{
		RuleID:  "quality.ai.dead-code",
		Level:   "warn",
		Path:    file,
		Line:    line,
		Column:  1,
		Message: "statement is unreachable because the previous statement unconditionally exits the block",
	})
}

func scriptUnreachableFindings(env support.Context, file string, source string) []core.Finding {
	findings := make([]core.Finding, 0)
	sanitized := sanitizeScriptSource(source)
	depth := 0
	pendingDepth := -1
	for idx, line := range strings.Split(sanitized, "\n") {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			continue
		}
		startDepth := depth
		depth += strings.Count(line, "{") - strings.Count(line, "}")
		if pendingDepth >= 0 {
			if startDepth == pendingDepth && !scriptBlockResumePattern.MatchString(trimmed) {
				findings = append(findings, unreachableStatementFinding(env, file, idx+1))
			}
			pendingDepth = -1
		}
		if scriptTerminatorPattern.MatchString(trimmed) && balancedParens(trimmed) {
			pendingDepth = depth
		}
	}
	return findings
}

func balancedParens(line string) bool {
	return strings.Count(line, "(") == strings.Count(line, ")")
}

// --- TypeScript/JavaScript: unused file-local function declarations ---

func scriptUnusedFunctionFindings(env support.Context, file string, source string) []core.Finding {
	sanitized := sanitizeScriptSource(source)
	findings := make([]core.Finding, 0)
	for _, match := range scriptLocalFunctionPattern.FindAllStringSubmatchIndex(sanitized, -1) {
		name := sanitized[match[2]:match[3]]
		lineStart := strings.LastIndexByte(sanitized[:match[0]], '\n') + 1
		declLine := sanitized[lineStart:lineEnd(sanitized, match[0])]
		if strings.Contains(declLine, "export") {
			continue
		}
		if countWordOccurrences(sanitized, name) > 1 {
			continue
		}
		line := 1 + strings.Count(sanitized[:match[2]], "\n")
		findings = append(findings, env.NewFinding(support.FindingInput{
			RuleID:  "quality.ai.dead-code",
			Level:   "warn",
			Path:    file,
			Line:    line,
			Column:  1,
			Message: fmt.Sprintf("file-local function %q is declared but never referenced in this file", name),
		}))
	}
	return findings
}

func lineEnd(source string, from int) int {
	if idx := strings.IndexByte(source[from:], '\n'); idx >= 0 {
		return from + idx
	}
	return len(source)
}

func countWordOccurrences(source string, word string) int {
	pattern := regexp.MustCompile(`\b` + regexp.QuoteMeta(word) + `\b`)
	return len(pattern.FindAllStringIndex(source, -1))
}
