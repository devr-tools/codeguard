package support

import (
	"regexp"
	"strings"
)

var (
	pythonImportPattern     = regexp.MustCompile(`^\s*import\s+(.+)$`)
	pythonFromImportPattern = regexp.MustCompile(`^\s*from\s+([\w.]+)\s+import\s+(.+)$`)
	identifierPattern       = regexp.MustCompile(`^[A-Za-z_]\w*$`)
)

func (b *pythonBuilder) collectImports(logical logicalLine) {
	text := strings.Join(strings.Fields(logical.masked), " ")
	if match := pythonFromImportPattern.FindStringSubmatch(text); match != nil {
		b.file.Imports = append(b.file.Imports, pythonFromImports(match[1], match[2], logical.startLine)...)
		return
	}
	if match := pythonImportPattern.FindStringSubmatch(text); match != nil {
		b.file.Imports = append(b.file.Imports, pythonPlainImports(match[1], logical.startLine)...)
	}
}

func pythonPlainImports(clause string, line int) []ParsedImport {
	imports := make([]ParsedImport, 0, 1)
	for _, part := range strings.Split(clause, ",") {
		module, alias := splitAsAlias(strings.TrimSpace(part))
		if module == "" {
			continue
		}
		if alias == "" {
			alias = strings.Split(module, ".")[0]
		}
		imports = append(imports, ParsedImport{Module: module, Alias: alias, Line: line})
	}
	return imports
}

func pythonFromImports(module string, clause string, line int) []ParsedImport {
	clause = strings.Trim(strings.TrimSpace(clause), "()")
	imports := make([]ParsedImport, 0, 1)
	for _, part := range strings.Split(clause, ",") {
		name, alias := splitAsAlias(strings.TrimSpace(part))
		if name == "" {
			continue
		}
		if alias == "" {
			alias = name
		}
		imports = append(imports, ParsedImport{Module: module, Name: name, Alias: alias, Line: line})
	}
	return imports
}

func splitAsAlias(part string) (string, string) {
	fields := strings.Fields(part)
	if len(fields) == 3 && fields[1] == "as" {
		return fields[0], fields[2]
	}
	if len(fields) == 1 {
		return fields[0], ""
	}
	return "", ""
}

// pythonAssignments extracts simple and tuple assignments from one masked
// logical statement. Subscript or attribute targets are ignored.
func pythonAssignments(statement ParsedStatement) []ParsedAssignment {
	opIdx, opLen, augmented := findAssignmentOperator(statement.Text)
	if opIdx < 0 {
		return nil
	}
	lhs := strings.TrimSpace(statement.Text[:opIdx])
	rhs := strings.TrimSpace(statement.Text[opIdx+opLen:])
	if colon := strings.IndexByte(lhs, ':'); colon >= 0 {
		lhs = strings.TrimSpace(lhs[:colon])
	}
	assignments := make([]ParsedAssignment, 0, 1)
	for _, target := range strings.Split(lhs, ",") {
		target = strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(target), "*"))
		if !identifierPattern.MatchString(target) || isPythonKeyword(target) {
			continue
		}
		assignments = append(assignments, ParsedAssignment{Name: target, Expr: rhs, Line: statement.Line, Augmented: augmented})
	}
	return assignments
}

// findAssignmentOperator locates the first top-level assignment in masked
// text, returning its index, operator length, and whether it is augmented.
func findAssignmentOperator(text string) (int, int, bool) {
	depth := 0
	for i := 0; i < len(text); i++ {
		switch text[i] {
		case '(', '[', '{':
			depth++
		case ')', ']', '}':
			depth--
		case '=':
			if depth != 0 {
				continue
			}
			if idx, length, augmented, ok := classifyEquals(text, i); ok {
				return idx, length, augmented
			}
			if i+1 < len(text) && text[i+1] == '=' {
				i++
			}
		}
	}
	return -1, 0, false
}

func classifyEquals(text string, i int) (int, int, bool, bool) {
	if i+1 < len(text) && text[i+1] == '=' {
		return 0, 0, false, false
	}
	if i == 0 {
		return 0, 0, false, false
	}
	prev := text[i-1]
	if strings.IndexByte("=!<>:", prev) >= 0 {
		return 0, 0, false, false
	}
	if strings.IndexByte("+-*/%&|^@", prev) >= 0 {
		return i - 1, 2, true, true
	}
	return i, 1, false, true
}

func isPythonKeyword(word string) bool {
	switch word {
	case "if", "elif", "else", "for", "while", "return", "yield", "assert",
		"lambda", "and", "or", "not", "in", "is", "with", "as", "def", "class",
		"print", "del", "raise", "except", "try", "from", "import", "pass",
		"break", "continue", "global", "nonlocal", "await", "async", "match", "case":
		return true
	default:
		return false
	}
}
