package design

import (
	"strings"
)

type pythonTypeBlockKind string

const (
	pythonTypeBlockClass    pythonTypeBlockKind = "class"
	pythonTypeBlockProtocol pythonTypeBlockKind = "protocol"
)

type pythonTypeBlock struct {
	kind         pythonTypeBlockKind
	name         string
	line         int
	headerIndent int
	bodyIndent   int
	memberCount  int
}

type pythonTypeLogicalLine struct {
	startLine int
	indent    int
	text      string
}

func parsePythonClassDecl(text string) (string, string, bool) {
	match := pythonClassDeclPattern.FindStringSubmatch(text)
	if len(match) != 3 {
		return "", "", false
	}
	return match[1], match[2], true
}

func parsePythonMethodDecl(text string) (string, bool) {
	match := pythonMethodDeclPattern.FindStringSubmatch(text)
	if len(match) != 2 {
		return "", false
	}
	return match[1], true
}

func pythonTypeBlockForBases(bases string) pythonTypeBlockKind {
	for _, base := range pythonSplitTopLevelCSV(bases) {
		base = strings.TrimSpace(base)
		if cut := strings.IndexByte(base, '['); cut >= 0 {
			base = base[:cut]
		}
		if base == "Protocol" || strings.HasSuffix(base, ".Protocol") {
			return pythonTypeBlockProtocol
		}
	}
	return pythonTypeBlockClass
}

func isPythonProtocolAttribute(text string) bool {
	if strings.HasPrefix(text, "@") {
		return false
	}
	match := pythonProtocolAttrPattern.FindStringSubmatch(text)
	return len(match) == 2 && match[1] != ""
}

func pythonSplitTopLevelCSV(text string) []string {
	parts := make([]string, 0, 2)
	depth := 0
	start := 0
	for idx := 0; idx < len(text); idx++ {
		switch text[idx] {
		case '(', '[', '{':
			depth++
		case ')', ']', '}':
			if depth > 0 {
				depth--
			}
		case ',':
			if depth == 0 {
				parts = append(parts, text[start:idx])
				start = idx + 1
			}
		}
	}
	if start <= len(text) {
		parts = append(parts, text[start:])
	}
	return parts
}

func pythonCompactWhitespace(text string) string {
	return strings.Join(strings.Fields(text), " ")
}

func filepathExt(path string) string {
	if idx := strings.LastIndexByte(path, '.'); idx >= 0 {
		return path[idx:]
	}
	return ""
}
