package support

import (
	"regexp"
	"strings"
)

var pythonCallPattern = regexp.MustCompile(`([A-Za-z_]\w*(?:\s*\.\s*[A-Za-z_]\w*)*)\s*\(`)

// ExtractCalls extracts call expressions with their argument texts from
// masked statement or expression text.
func ExtractCalls(text string, startLine int) []ParsedCall {
	return maskedCalls(text, startLine)
}

// maskedCalls extracts call expressions from masked statement text.
func maskedCalls(text string, startLine int) []ParsedCall {
	calls := make([]ParsedCall, 0, 2)
	for _, match := range pythonCallPattern.FindAllStringSubmatchIndex(text, -1) {
		callee := strings.Join(strings.Fields(strings.ReplaceAll(text[match[2]:match[3]], " .", ".")), "")
		base := callee
		if dot := strings.IndexByte(base, '.'); dot >= 0 {
			base = base[:dot]
		}
		if isPythonKeyword(base) {
			continue
		}
		open := match[1] - 1
		args := splitTopLevelArgs(balancedSpan(text, open))
		line := startLine + strings.Count(text[:match[2]], "\n")
		calls = append(calls, ParsedCall{Callee: callee, Args: args, Line: line})
	}
	return calls
}

// balancedSpan returns the text between the opening bracket at open and its
// matching close bracket, exclusive.
func balancedSpan(text string, open int) string {
	depth := 0
	for i := open; i < len(text); i++ {
		switch text[i] {
		case '(', '[', '{':
			depth++
		case ')', ']', '}':
			depth--
			if depth == 0 {
				return text[open+1 : i]
			}
		}
	}
	if open+1 <= len(text) {
		return text[open+1:]
	}
	return ""
}

func splitTopLevelArgs(argText string) []string {
	if strings.TrimSpace(argText) == "" {
		return nil
	}
	args := make([]string, 0, 4)
	depth := 0
	start := 0
	appendArg := func(end int) {
		arg := strings.TrimSpace(argText[start:end])
		if arg != "" {
			args = append(args, arg)
		}
	}
	for i := 0; i < len(argText); i++ {
		switch argText[i] {
		case '(', '[', '{':
			depth++
		case ')', ']', '}':
			depth--
		case ',':
			if depth == 0 {
				appendArg(i)
				start = i + 1
			}
		}
	}
	appendArg(len(argText))
	return args
}

func parsePythonParams(signature string) []ParsedParam {
	params := make([]ParsedParam, 0, 4)
	for _, part := range splitTopLevelArgs(signature) {
		part = strings.TrimLeft(strings.TrimSpace(part), "*")
		if part == "" || part == "/" {
			continue
		}
		name := part
		paramType := ""
		if colon := topLevelIndex(part, ':'); colon >= 0 {
			name = strings.TrimSpace(part[:colon])
			paramType = strings.TrimSpace(part[colon+1:])
		}
		if eq := topLevelIndex(name, '='); eq >= 0 {
			name = strings.TrimSpace(name[:eq])
		}
		if eq := topLevelIndex(paramType, '='); eq >= 0 {
			paramType = strings.TrimSpace(paramType[:eq])
		}
		if identifierPattern.MatchString(name) {
			params = append(params, ParsedParam{Name: name, Type: paramType})
		}
	}
	return params
}

func topLevelIndex(text string, target byte) int {
	depth := 0
	for i := 0; i < len(text); i++ {
		switch text[i] {
		case '(', '[', '{':
			depth++
		case ')', ']', '}':
			depth--
		case target:
			if depth == 0 {
				return i
			}
		}
	}
	return -1
}
