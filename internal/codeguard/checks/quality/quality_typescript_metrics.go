package quality

import (
	"regexp"
	"strings"
)

var (
	tsFunctionPattern = regexp.MustCompile(`^\s*(?:export\s+)?(?:async\s+)?function\s+([A-Za-z_$][\w$]*)\s*(?:<[^>]+>)?\s*\(([^)]*)\)`)
	tsArrowPattern    = regexp.MustCompile(`^\s*(?:export\s+)?(?:const|let|var)\s+([A-Za-z_$][\w$]*)\s*=\s*(?:async\s*)?\(([^)]*)\)\s*(?::[^=]+)?=>`)
	tsMethodPattern   = regexp.MustCompile(`^\s*(?:public|private|protected|static|readonly|async|\s)*([A-Za-z_$][\w$]*)\s*\(([^)]*)\)\s*(?::[^{]+)?\{`)
)

func typeScriptFunctions(source string) []functionMetrics {
	lines := strings.Split(source, "\n")
	functions := make([]functionMetrics, 0)
	for idx, line := range lines {
		name, params, matched := matchedTypeScriptFunction(line)
		if !matched {
			continue
		}
		openIdx := strings.LastIndex(line, "{")
		if openIdx < 0 {
			continue
		}
		endIdx := findBraceBlockEnd(lines, idx, openIdx)
		body := strings.Join(lines[min(idx+1, len(lines)):endIdx+1], "\n")
		functions = append(functions, functionMetrics{
			Name:       name,
			StartLine:  idx + 1,
			Length:     max(1, endIdx-idx+1),
			Params:     countParameters(params),
			Complexity: typeScriptComplexity(body),
		})
	}
	return functions
}

func matchedTypeScriptFunction(line string) (string, string, bool) {
	if match := tsFunctionPattern.FindStringSubmatch(line); match != nil {
		return match[1], match[2], true
	}
	if match := tsArrowPattern.FindStringSubmatch(line); match != nil {
		return match[1], match[2], true
	}
	if match := tsMethodPattern.FindStringSubmatch(line); match != nil && !isControlKeyword(match[1]) {
		return match[1], match[2], true
	}
	return "", "", false
}

func findBraceBlockEnd(lines []string, start int, openIdx int) int {
	depth := 0
	for i := start; i < len(lines); i++ {
		startColumn := 0
		if i == start {
			startColumn = openIdx
		}
		for _, ch := range lines[i][startColumn:] {
			switch ch {
			case '{':
				depth++
			case '}':
				depth--
				if depth == 0 {
					return i
				}
			}
		}
	}
	return len(lines) - 1
}

func typeScriptComplexity(body string) int {
	complexity := 1
	for _, pattern := range []string{"if (", "for (", "while (", "case ", "catch (", "&&", "||", " ? "} {
		complexity += strings.Count(body, pattern)
	}
	return complexity
}

func isControlKeyword(name string) bool {
	switch name {
	case "if", "for", "while", "switch", "catch", "constructor":
		return true
	default:
		return false
	}
}
