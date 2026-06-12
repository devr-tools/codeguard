package support

import (
	"regexp"
	"strings"
)

var pythonDefPattern = regexp.MustCompile(`^(\s*)(?:async\s+)?def\s+([A-Za-z_]\w*)\s*\(`)

// ParsePython builds a lightweight AST for one Python source file.
func ParsePython(source string) *ParsedFile {
	source = strings.ReplaceAll(source, "\r\n", "\n")
	file := &ParsedFile{
		Language: "python",
		Source:   source,
		Masked:   MaskPythonSource(source),
		Module:   &ParsedFunction{Name: "<module>", StartLine: 1},
	}
	builder := &pythonBuilder{file: file}
	for _, logical := range pythonLogicalLines(source, file.Masked) {
		builder.consume(logical)
	}
	builder.closeFunctions(0, true)
	file.Module.EndLine = builder.lastContentLine
	return file
}

type logicalLine struct {
	startLine int
	indent    int
	masked    string
	raw       string
}

// pythonLogicalLines groups physical lines into logical statements by
// tracking bracket depth and trailing backslash continuations on the masked
// text, where string and comment contents cannot confuse the count.
func pythonLogicalLines(source string, masked string) []logicalLine {
	rawLines := strings.Split(source, "\n")
	maskedLines := strings.Split(masked, "\n")
	logical := make([]logicalLine, 0, len(rawLines))
	current := logicalLine{}
	depth := 0
	open := false
	for idx := range rawLines {
		if !open {
			current = logicalLine{startLine: idx + 1, indent: indentWidthOf(maskedLines[idx])}
		} else {
			current.masked += "\n"
			current.raw += "\n"
		}
		current.masked += maskedLines[idx]
		current.raw += rawLines[idx]
		depth += bracketDelta(maskedLines[idx])
		open = depth > 0 || strings.HasSuffix(strings.TrimRight(maskedLines[idx], " \t"), "\\")
		if open {
			continue
		}
		if strings.TrimSpace(current.masked) != "" {
			logical = append(logical, current)
		}
	}
	if open && strings.TrimSpace(current.masked) != "" {
		logical = append(logical, current)
	}
	return logical
}

type openPythonFunction struct {
	fn        *ParsedFunction
	defIndent int
}

type pythonBuilder struct {
	file            *ParsedFile
	stack           []openPythonFunction
	lastContentLine int
}

func (b *pythonBuilder) consume(logical logicalLine) {
	b.closeFunctions(logical.indent, false)
	if match := pythonDefPattern.FindStringSubmatch(logical.masked); match != nil {
		b.openFunction(logical, match[2])
		b.lastContentLine = logical.startLine + strings.Count(logical.masked, "\n")
		return
	}
	b.collectImports(logical)
	scope := b.currentScope()
	statement := ParsedStatement{Line: logical.startLine, Indent: logical.indent, Text: logical.masked, Raw: logical.raw}
	scope.Statements = append(scope.Statements, statement)
	scope.Assignments = append(scope.Assignments, pythonAssignments(statement)...)
	scope.Calls = append(scope.Calls, maskedCalls(logical.masked, logical.startLine)...)
	b.lastContentLine = logical.startLine + strings.Count(logical.masked, "\n")
}

func (b *pythonBuilder) openFunction(logical logicalLine, name string) {
	signature := pythonSignatureText(logical.masked)
	fn := &ParsedFunction{
		Name:      name,
		StartLine: logical.startLine,
		Signature: strings.TrimSpace(signature),
		Params:    parsePythonParams(signature),
	}
	if len(b.stack) > 0 {
		parent := b.stack[len(b.stack)-1].fn
		parent.Nested = append(parent.Nested, fn)
	} else {
		b.file.Functions = append(b.file.Functions, fn)
	}
	b.stack = append(b.stack, openPythonFunction{fn: fn, defIndent: logical.indent})
}

func (b *pythonBuilder) closeFunctions(indent int, all bool) {
	for len(b.stack) > 0 {
		top := b.stack[len(b.stack)-1]
		if !all && indent > top.defIndent {
			return
		}
		top.fn.EndLine = max(top.fn.StartLine, b.lastContentLine)
		b.stack = b.stack[:len(b.stack)-1]
	}
}

func (b *pythonBuilder) currentScope() *ParsedFunction {
	if len(b.stack) == 0 {
		return b.file.Module
	}
	return b.stack[len(b.stack)-1].fn
}

// pythonSignatureText extracts the parameter list between the def's parens.
func pythonSignatureText(maskedDef string) string {
	open := strings.IndexByte(maskedDef, '(')
	if open < 0 {
		return ""
	}
	depth := 0
	for i := open; i < len(maskedDef); i++ {
		switch maskedDef[i] {
		case '(', '[', '{':
			depth++
		case ')', ']', '}':
			depth--
			if depth == 0 {
				return maskedDef[open+1 : i]
			}
		}
	}
	return maskedDef[open+1:]
}
