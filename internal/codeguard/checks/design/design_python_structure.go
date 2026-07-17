package design

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/devr-tools/codeguard/internal/codeguard/checks/support"
	"github.com/devr-tools/codeguard/internal/codeguard/core"
)

var (
	pythonClassDeclPattern    = regexp.MustCompile(`^class\s+([A-Za-z_]\w*)\s*(?:\((.*)\))?\s*:`)
	pythonMethodDeclPattern   = regexp.MustCompile(`^(?:async\s+)?def\s+([A-Za-z_]\w*)\s*\(`)
	pythonProtocolAttrPattern = regexp.MustCompile(`^([A-Za-z_]\w*)\s*:`)
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

func pythonStructuralFindings(env support.Context, target core.TargetConfig) []core.Finding {
	return env.ScanTargetFiles(target, "design", func(rel string) bool {
		return strings.EqualFold(".py", filepathExt(rel))
	}, func(file string, data []byte) []core.Finding {
		return pythonStructuralFindingsForFile(env, file, data)
	})
}

func pythonStructuralFindingsForFile(env support.Context, file string, data []byte) []core.Finding {
	blocks := pythonTypeBlocks(string(data))
	findings := make([]core.Finding, 0, len(blocks))
	for _, block := range blocks {
		switch block.kind {
		case pythonTypeBlockClass:
			if block.memberCount <= env.Config.Checks.DesignRules.MaxMethodsPerType {
				continue
			}
			findings = append(findings, env.NewFinding(support.FindingInput{
				RuleID:  "design.python.max-methods-per-type",
				Level:   "warn",
				Path:    file,
				Line:    block.line,
				Column:  1,
				Message: fmt.Sprintf("class %s has %d methods; max is %d", block.name, block.memberCount, env.Config.Checks.DesignRules.MaxMethodsPerType),
			}))
		case pythonTypeBlockProtocol:
			if block.memberCount <= env.Config.Checks.DesignRules.MaxInterfaceMethods {
				continue
			}
			findings = append(findings, env.NewFinding(support.FindingInput{
				RuleID:  "design.python.max-protocol-members",
				Level:   "warn",
				Path:    file,
				Line:    block.line,
				Column:  1,
				Message: fmt.Sprintf("protocol %s has %d members; max is %d", block.name, block.memberCount, env.Config.Checks.DesignRules.MaxInterfaceMethods),
			}))
		}
	}
	return findings
}

func pythonTypeBlocks(source string) []pythonTypeBlock {
	masked := support.MaskPythonSource(strings.ReplaceAll(source, "\r\n", "\n"))
	logical := pythonTypeLogicalLines(masked)
	stack := make([]pythonTypeBlock, 0)
	finished := make([]pythonTypeBlock, 0)

	for _, line := range logical {
		for len(stack) > 0 && line.indent <= stack[len(stack)-1].headerIndent {
			finished = append(finished, stack[len(stack)-1])
			stack = stack[:len(stack)-1]
		}

		text := pythonCompactWhitespace(line.text)
		if text == "" {
			continue
		}
		if name, bases, ok := parsePythonClassDecl(text); ok {
			stack = append(stack, pythonTypeBlock{
				kind:         pythonTypeBlockForBases(bases),
				name:         name,
				line:         line.startLine,
				headerIndent: line.indent,
				bodyIndent:   -1,
			})
			continue
		}
		if len(stack) == 0 {
			continue
		}

		top := &stack[len(stack)-1]
		if line.indent <= top.headerIndent {
			continue
		}
		if top.bodyIndent < 0 {
			top.bodyIndent = line.indent
		}
		if line.indent != top.bodyIndent {
			continue
		}

		if methodName, ok := parsePythonMethodDecl(text); ok {
			if top.kind == pythonTypeBlockClass && methodName == "__init__" {
				continue
			}
			top.memberCount++
			continue
		}
		if top.kind == pythonTypeBlockProtocol && isPythonProtocolAttribute(text) {
			top.memberCount++
		}
	}

	for len(stack) > 0 {
		finished = append(finished, stack[len(stack)-1])
		stack = stack[:len(stack)-1]
	}
	return finished
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

func filepathExt(path string) string {
	if idx := strings.LastIndexByte(path, '.'); idx >= 0 {
		return path[idx:]
	}
	return ""
}
