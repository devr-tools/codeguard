package quality

import (
	"regexp"
	"strings"

	"github.com/devr-tools/codeguard/internal/codeguard/checks/support"
	"github.com/devr-tools/codeguard/internal/codeguard/core"
)

var pythonFunctionPattern = regexp.MustCompile(`^\s*(?:async\s+def|def)\s+([A-Za-z_]\w*)\s*\((.*)\)\s*:`)

func pythonFindingsForFile(env support.Context, file string, data []byte) []core.Finding {
	findings := fileLengthFinding(env, file, data)
	for _, fn := range pythonFunctions(string(data)) {
		findings = append(findings, maintainabilityFindings(env, file, fn)...)
	}
	return findings
}

func pythonFunctions(source string) []functionMetrics {
	lines := strings.Split(source, "\n")
	functions := make([]functionMetrics, 0)
	for idx, line := range lines {
		match := pythonFunctionPattern.FindStringSubmatch(line)
		if match == nil {
			continue
		}
		startIndent := indentationWidth(line)
		endIdx := len(lines) - 1
		for j := idx + 1; j < len(lines); j++ {
			trimmed := strings.TrimSpace(lines[j])
			if trimmed == "" {
				continue
			}
			if indentationWidth(lines[j]) <= startIndent {
				endIdx = j - 1
				break
			}
		}
		body := strings.Join(lines[min(idx+1, len(lines)):endIdx+1], "\n")
		functions = append(functions, functionMetrics{
			Name:       match[1],
			StartLine:  idx + 1,
			Length:     max(1, endIdx-idx+1),
			Params:     countParameters(match[2]),
			Complexity: pythonComplexity(body),
		})
	}
	return functions
}

func pythonComplexity(body string) int {
	complexity := 1
	for _, pattern := range []string{" if ", " elif ", " for ", " while ", " except ", " case ", " and ", " or "} {
		complexity += strings.Count(" "+body+" ", pattern)
	}
	return complexity
}

func indentationWidth(line string) int {
	width := 0
	for _, ch := range line {
		if ch == ' ' {
			width++
			continue
		}
		if ch == '\t' {
			width += 4
			continue
		}
		break
	}
	return width
}
