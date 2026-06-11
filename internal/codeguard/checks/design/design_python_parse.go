package design

import (
	"regexp"
	"strings"
)

var (
	pythonImportPattern     = regexp.MustCompile(`^\s*import\s+(.+)$`)
	pythonFromImportPattern = regexp.MustCompile(`^\s*from\s+([A-Za-z0-9_\.]+|\.+)\s+import\s+(.+)$`)
)

type pythonImportStatement struct {
	line    int
	modules []string
	from    string
	names   []string
}

func pythonImportStatements(currentModule string, currentPackage string, data []byte) []pythonImportStatement {
	logical := pythonLogicalImportStatements(string(data))
	statements := make([]pythonImportStatement, 0, len(logical))
	for _, statement := range logical {
		parsed, ok := parsePythonImportStatement(currentModule, currentPackage, statement.text, statement.line)
		if ok {
			statements = append(statements, parsed)
		}
	}
	return statements
}

func pythonLogicalImportStatements(data string) []struct {
	line int
	text string
} {
	lines := strings.Split(strings.ReplaceAll(data, "\r\n", "\n"), "\n")
	statements := make([]struct {
		line int
		text string
	}, 0)
	var builder strings.Builder
	startLine := 0
	depth := 0
	for idx, raw := range lines {
		lineNo := idx + 1
		trimmed := strings.TrimSpace(stripPythonInlineComment(raw))
		if builder.Len() == 0 {
			if !strings.HasPrefix(trimmed, "import ") && !strings.HasPrefix(trimmed, "from ") {
				continue
			}
			startLine = lineNo
		}
		if trimmed == "" && builder.Len() == 0 {
			continue
		}
		if trimmed != "" {
			if builder.Len() > 0 {
				builder.WriteByte(' ')
			}
			builder.WriteString(trimmed)
			depth += strings.Count(trimmed, "(")
			depth -= strings.Count(trimmed, ")")
		}
		if builder.Len() == 0 {
			continue
		}
		if strings.HasSuffix(trimmed, "\\") || depth > 0 {
			continue
		}
		statements = append(statements, struct {
			line int
			text string
		}{line: startLine, text: builder.String()})
		builder.Reset()
		startLine = 0
		depth = 0
	}
	return statements
}

func parsePythonImportStatement(currentModule string, currentPackage string, statement string, line int) (pythonImportStatement, bool) {
	if match := pythonImportPattern.FindStringSubmatch(statement); len(match) == 2 {
		modules := splitPythonImportList(match[1])
		if len(modules) == 0 {
			return pythonImportStatement{}, false
		}
		return pythonImportStatement{line: line, modules: modules}, true
	}
	if match := pythonFromImportPattern.FindStringSubmatch(statement); len(match) == 3 {
		module := resolvePythonImportModule(currentModule, currentPackage, strings.TrimSpace(match[1]))
		names := splitPythonImportList(match[2])
		return pythonImportStatement{line: line, from: module, names: names}, module != "" || len(names) > 0
	}
	return pythonImportStatement{}, false
}

func splitPythonImportList(raw string) []string {
	cleaned := strings.NewReplacer("(", "", ")", "", "\\", "", ";", "").Replace(raw)
	parts := strings.Split(cleaned, ",")
	values := make([]string, 0, len(parts))
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		fields := strings.Fields(part)
		if len(fields) > 0 {
			values = append(values, fields[0])
		}
	}
	return values
}

func stripPythonInlineComment(line string) string {
	inSingle := false
	inDouble := false
	escaped := false
	for idx, r := range line {
		switch {
		case escaped:
			escaped = false
		case r == '\\':
			escaped = true
		case r == '\'' && !inDouble:
			inSingle = !inSingle
		case r == '"' && !inSingle:
			inDouble = !inDouble
		case r == '#' && !inSingle && !inDouble:
			return line[:idx]
		}
	}
	return line
}

func resolvePythonImportModule(currentModule string, currentPackage string, imported string) string {
	if !strings.HasPrefix(imported, ".") {
		return imported
	}
	dots := 0
	for dots < len(imported) && imported[dots] == '.' {
		dots++
	}
	remainder := strings.TrimPrefix(imported, strings.Repeat(".", dots))
	packageName := currentPackage
	if packageName == "" {
		if cut := strings.LastIndex(currentModule, "."); cut >= 0 {
			packageName = currentModule[:cut]
		}
	}
	if packageName == "" {
		return remainder
	}
	parts := strings.Split(packageName, ".")
	limit := len(parts) - dots
	if dots > 0 {
		limit++
	}
	if limit < 0 {
		limit = 0
	}
	base := parts[:limit]
	if remainder != "" {
		base = append(base, remainder)
	}
	return strings.Join(base, ".")
}
