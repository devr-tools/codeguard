package support

import "strings"

func ParsePythonFunctions(source string) []ParsedFunction {
	source = strings.ReplaceAll(source, "\r\n", "\n")
	lines := strings.Split(source, "\n")
	functions := make([]ParsedFunction, 0)
	for idx := 0; idx < len(lines); idx++ {
		name, params, ok := parsePythonFunctionHeader(lines, idx)
		if !ok {
			continue
		}
		startIndent := pythonIndentationWidth(lines[idx])
		headerEnd := idx
		if strings.Count(strings.Join(lines[idx:], "\n"), "\n") > 0 {
			headerEnd = pythonHeaderEnd(lines, idx)
		}
		endIdx := len(lines) - 1
		for j := headerEnd + 1; j < len(lines); j++ {
			trimmed := strings.TrimSpace(lines[j])
			if trimmed == "" {
				continue
			}
			if pythonIndentationWidth(lines[j]) <= startIndent {
				endIdx = j - 1
				break
			}
		}
		bodyStart := min(headerEnd+1, len(lines))
		bodyEnd := min(endIdx+1, len(lines))
		functions = append(functions, ParsedFunction{
			Name:       name,
			StartLine:  idx + 1,
			EndLine:    endIdx + 1,
			Parameters: params,
			Body:       strings.Join(lines[bodyStart:bodyEnd], "\n"),
		})
		idx = headerEnd
	}
	return functions
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func parsePythonFunctionHeader(lines []string, start int) (string, string, bool) {
	headerEnd := pythonHeaderEnd(lines, start)
	if headerEnd < start {
		return "", "", false
	}
	header := strings.Join(lines[start:headerEnd+1], "\n")
	trimmed := strings.TrimSpace(header)
	switch {
	case strings.HasPrefix(trimmed, "async def "):
		trimmed = strings.TrimPrefix(trimmed, "async def ")
	case strings.HasPrefix(trimmed, "def "):
		trimmed = strings.TrimPrefix(trimmed, "def ")
	default:
		return "", "", false
	}
	openIdx := strings.Index(trimmed, "(")
	if openIdx <= 0 {
		return "", "", false
	}
	name := strings.TrimSpace(trimmed[:openIdx])
	closeIdx := findBalancedPythonDelimiter(trimmed, openIdx, '(', ')')
	if closeIdx < 0 {
		return "", "", false
	}
	colonIdx := closeIdx + 1
	for colonIdx < len(trimmed) && (trimmed[colonIdx] == ' ' || trimmed[colonIdx] == '\t') {
		colonIdx++
	}
	if colonIdx >= len(trimmed) || trimmed[colonIdx] != ':' {
		return "", "", false
	}
	return name, trimmed[openIdx+1 : closeIdx], name != ""
}

func pythonHeaderEnd(lines []string, start int) int {
	parenDepth := 0
	inString := byte(0)
	for idx := start; idx < len(lines); idx++ {
		line := lines[idx]
		for pos := 0; pos < len(line); pos++ {
			ch := line[pos]
			if inString != 0 {
				if ch == '\\' && pos+1 < len(line) {
					pos++
					continue
				}
				if ch == inString {
					inString = 0
				}
				continue
			}
			switch ch {
			case '\'', '"':
				inString = ch
			case '#':
				pos = len(line)
			case '(':
				parenDepth++
			case ')':
				if parenDepth > 0 {
					parenDepth--
				}
			case ':':
				if parenDepth == 0 {
					return idx
				}
			}
		}
	}
	return -1
}

func findBalancedPythonDelimiter(source string, start int, open rune, close rune) int {
	depth := 0
	inString := rune(0)
	for idx, ch := range source[start:] {
		switch {
		case inString != 0:
			if ch == inString {
				inString = 0
			}
		case ch == '"' || ch == '\'':
			inString = ch
		case ch == open:
			depth++
		case ch == close:
			depth--
			if depth == 0 {
				return start + idx
			}
		}
	}
	return -1
}

func pythonIndentationWidth(line string) int {
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
