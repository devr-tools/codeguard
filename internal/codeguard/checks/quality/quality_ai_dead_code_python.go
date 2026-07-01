package quality

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/devr-tools/codeguard/internal/codeguard/checks/support"
	"github.com/devr-tools/codeguard/internal/codeguard/core"
)

var pythonTerminatorPattern = regexp.MustCompile(`^(?:return\b|raise\b|break$|continue$|break\s|continue\s)`)
var pythonBlockResumePattern = regexp.MustCompile(`^(?:elif\b|else\s*:|except\b|finally\s*:|case\b)`)

func pythonDeadCodeFindings(env support.Context, file string, source string) []core.Finding {
	findings := make([]core.Finding, 0)
	pendingIndent := -1
	bracketDepth := 0
	continuation := false
	for idx, raw := range strings.Split(source, "\n") {
		line := stripPythonComment(raw)
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			continue
		}
		logicalStart := bracketDepth == 0 && !continuation
		bracketDepth += strings.Count(line, "(") + strings.Count(line, "[") + strings.Count(line, "{")
		bracketDepth -= strings.Count(line, ")") + strings.Count(line, "]") + strings.Count(line, "}")
		if bracketDepth < 0 {
			bracketDepth = 0
		}
		continuation = strings.HasSuffix(trimmed, "\\")
		if !logicalStart {
			continue
		}
		indent := len(line) - len(strings.TrimLeft(line, " \t"))
		if pendingIndent >= 0 {
			if indent == pendingIndent && !pythonBlockResumePattern.MatchString(trimmed) {
				findings = append(findings, unreachableStatementFinding(env, file, idx+1))
			}
			pendingIndent = -1
		}
		if pythonTerminatorPattern.MatchString(trimmed) && bracketDepth == 0 && !continuation {
			pendingIndent = indent
		}
	}
	return findings
}

// stripPythonComment removes a trailing comment that starts outside string
// literals on the line. Multi-line strings are not tracked; the analysis is
// intentionally conservative.
func stripPythonComment(line string) string {
	inSingle := false
	inDouble := false
	for idx := 0; idx < len(line); idx++ {
		switch line[idx] {
		case '\\':
			idx++
		case '\'':
			if !inDouble {
				inSingle = !inSingle
			}
		case '"':
			if !inSingle {
				inDouble = !inDouble
			}
		case '#':
			if !inSingle && !inDouble {
				return line[:idx]
			}
		}
	}
	return line
}

// --- Python: unused private functions ---

var pythonPrivateFunctionPattern = regexp.MustCompile(`(?m)^[ \t]*def[ \t]+(_[A-Za-z0-9_]*)[ \t]*\(`)

func pythonUnusedPrivateFunctionFindings(env support.Context, file string, source string) []core.Finding {
	findings := make([]core.Finding, 0)
	for _, match := range pythonPrivateFunctionPattern.FindAllStringSubmatchIndex(source, -1) {
		name := source[match[2]:match[3]]
		if strings.HasPrefix(name, "__") && strings.HasSuffix(name, "__") {
			continue
		}
		if countWordOccurrences(source, name) > 1 {
			continue
		}
		line := 1 + strings.Count(source[:match[2]], "\n")
		findings = append(findings, warnFinding(env, "quality.ai.dead-code", file, line, 1,
			fmt.Sprintf("private function %q is declared but never referenced in this file", name)))
	}
	return findings
}
