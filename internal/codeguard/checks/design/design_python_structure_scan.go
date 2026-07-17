package design

import (
	"strings"

	"github.com/devr-tools/codeguard/internal/codeguard/checks/support"
)

type pythonTypeScanner struct {
	stack    []pythonTypeBlock
	finished []pythonTypeBlock
}

func pythonTypeBlocks(source string) []pythonTypeBlock {
	masked := support.MaskPythonSource(strings.ReplaceAll(source, "\r\n", "\n"))
	scanner := pythonTypeScanner{}
	for _, line := range pythonTypeLogicalLines(masked) {
		scanner.consume(line)
	}
	return scanner.finish()
}

func (scanner *pythonTypeScanner) consume(line pythonTypeLogicalLine) {
	scanner.closeFinishedBlocks(line.indent)

	text := pythonCompactWhitespace(line.text)
	if text == "" {
		return
	}
	if scanner.openClassBlock(line, text) {
		return
	}
	scanner.countTopLevelMember(line, text)
}

func (scanner *pythonTypeScanner) closeFinishedBlocks(indent int) {
	for len(scanner.stack) > 0 && indent <= scanner.stack[len(scanner.stack)-1].headerIndent {
		scanner.finished = append(scanner.finished, scanner.stack[len(scanner.stack)-1])
		scanner.stack = scanner.stack[:len(scanner.stack)-1]
	}
}

func (scanner *pythonTypeScanner) openClassBlock(line pythonTypeLogicalLine, text string) bool {
	name, bases, ok := parsePythonClassDecl(text)
	if !ok {
		return false
	}
	scanner.stack = append(scanner.stack, pythonTypeBlock{
		kind:         pythonTypeBlockForBases(bases),
		name:         name,
		line:         line.startLine,
		headerIndent: line.indent,
		bodyIndent:   -1,
	})
	return true
}

func (scanner *pythonTypeScanner) countTopLevelMember(line pythonTypeLogicalLine, text string) {
	if len(scanner.stack) == 0 {
		return
	}

	top := &scanner.stack[len(scanner.stack)-1]
	if line.indent <= top.headerIndent {
		return
	}
	if top.bodyIndent < 0 {
		top.bodyIndent = line.indent
	}
	if line.indent != top.bodyIndent {
		return
	}
	if methodName, ok := parsePythonMethodDecl(text); ok {
		if top.kind != pythonTypeBlockClass || methodName != "__init__" {
			top.memberCount++
		}
		return
	}
	if top.kind == pythonTypeBlockProtocol && isPythonProtocolAttribute(text) {
		top.memberCount++
	}
}

func (scanner *pythonTypeScanner) finish() []pythonTypeBlock {
	for len(scanner.stack) > 0 {
		scanner.finished = append(scanner.finished, scanner.stack[len(scanner.stack)-1])
		scanner.stack = scanner.stack[:len(scanner.stack)-1]
	}
	return scanner.finished
}

func pythonTypeLogicalLines(masked string) []pythonTypeLogicalLine {
	lines := strings.Split(masked, "\n")
	logical := make([]pythonTypeLogicalLine, 0, len(lines))
	current := pythonTypeLogicalLine{}
	depth := 0
	open := false

	for idx, line := range lines {
		if !open {
			current = pythonTypeLogicalLine{startLine: idx + 1, indent: pythonIndentWidth(line)}
		} else {
			current.text += "\n"
		}
		current.text += line
		depth += pythonBracketDelta(line)
		open = depth > 0 || strings.HasSuffix(strings.TrimRight(line, " \t"), "\\")
		if open {
			continue
		}
		if strings.TrimSpace(current.text) != "" {
			logical = append(logical, current)
		}
	}

	if open && strings.TrimSpace(current.text) != "" {
		logical = append(logical, current)
	}
	return logical
}

func pythonBracketDelta(line string) int {
	delta := 0
	for idx := 0; idx < len(line); idx++ {
		switch line[idx] {
		case '(', '[', '{':
			delta++
		case ')', ']', '}':
			delta--
		}
	}
	return delta
}

func pythonIndentWidth(line string) int {
	width := 0
	for _, ch := range line {
		switch ch {
		case ' ':
			width++
		case '\t':
			width += 4
		default:
			return width
		}
	}
	return width
}
