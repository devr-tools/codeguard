package support

import (
	"regexp"
	"strings"
	"unicode"
)

var pythonCallPattern = regexp.MustCompile(`([A-Za-z_]\w*(?:\s*\.\s*[A-Za-z_]\w*)*)\s*\(`)

// ExtractCalls extracts call expressions with their argument texts from
// masked statement or expression text.
func ExtractCalls(text string, startLine int) []ParsedCall {
	return maskedCalls(text, startLine)
}

// maskedCalls extracts call expressions from masked statement text.
func maskedCalls(text string, startLine int) []ParsedCall {
	matches := pythonCallPattern.FindAllStringSubmatchIndex(text, -1)
	spans := pythonCallSpans(text, matches)
	calls := make([]ParsedCall, 0, len(matches))
	trimmedEnds := make(map[int]int)
	line, lineOffset := startLine, 0
	for matchIndex, match := range matches {
		callee := strings.Join(strings.Fields(strings.ReplaceAll(text[match[2]:match[3]], " .", ".")), "")
		base := callee
		if dot := strings.IndexByte(base, '.'); dot >= 0 {
			base = base[:dot]
		}
		if isPythonKeyword(base) {
			continue
		}
		line += strings.Count(text[lineOffset:match[2]], "\n")
		lineOffset = match[2]
		args := spans[matchIndex].args(text, trimmedEnds)
		calls = append(calls, ParsedCall{Callee: callee, Args: args, Line: line})
	}
	return calls
}

type pythonCallSpan struct {
	open   int
	close  int
	commas []int
}

func (span pythonCallSpan) args(text string, trimmedEnds map[int]int) []string {
	if span.close <= span.open+1 {
		return nil
	}
	args := make([]string, 0, len(span.commas)+1)
	start := span.open + 1
	for _, end := range append(span.commas, span.close) {
		trimmedEnd, ok := trimmedEnds[end]
		if !ok {
			trimmedEnd = start + len(strings.TrimRightFunc(text[start:end], unicode.IsSpace))
			trimmedEnds[end] = trimmedEnd
		}
		if trimmedEnd < start {
			trimmedEnd = start
		}
		if arg := strings.TrimLeftFunc(text[start:trimmedEnd], unicode.IsSpace); arg != "" {
			args = append(args, arg)
		}
		start = end + 1
	}
	return args
}

// pythonCallSpans finds the closing parenthesis and top-level commas for all
// calls in one pass. In particular, it avoids rescanning the remainder of a
// malformed or deeply nested statement once for every call expression.
func pythonCallSpans(text string, matches [][]int) []pythonCallSpan {
	spans := make([]pythonCallSpan, len(matches))
	callAt := make(map[int]int, len(matches))
	for index, match := range matches {
		open := match[1] - 1
		spans[index] = pythonCallSpan{open: open, close: len(text)}
		callAt[open] = index
	}

	type bracket struct {
		call int
	}
	stack := make([]bracket, 0, 8)
	for offset := 0; offset < len(text); offset++ {
		switch text[offset] {
		case '(', '[', '{':
			call := -1
			if index, ok := callAt[offset]; ok {
				call = index
			}
			stack = append(stack, bracket{call: call})
		case ',':
			if len(stack) > 0 && stack[len(stack)-1].call >= 0 {
				index := stack[len(stack)-1].call
				spans[index].commas = append(spans[index].commas, offset)
			}
		case ')', ']', '}':
			if len(stack) == 0 {
				continue
			}
			top := stack[len(stack)-1]
			stack = stack[:len(stack)-1]
			if top.call >= 0 {
				spans[top.call].close = offset
			}
		}
	}
	return spans
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
