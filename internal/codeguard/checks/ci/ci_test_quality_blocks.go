package ci

import (
	"regexp"
	"strings"
)

// testBlock is one test function extracted from a test file. lines holds the
// raw source lines of the block including the declaration line, and startLine
// is the 1-based file line of that declaration.
type testBlock struct {
	name      string
	startLine int
	lines     []string
	hasElse   bool
}

var (
	goTestDeclPattern     = regexp.MustCompile(`^func\s+(Test\w+)\s*\(`)
	jsTestDeclPattern     = regexp.MustCompile(`^\s*(?:it|test)(?:\.\w+)?\s*\(\s*(?:'([^']*)'|"([^"]*)")?`)
	pythonTestDeclPattern = regexp.MustCompile(`^(\s*)def\s+(test_\w*)\s*\(`)
	braceElsePattern      = regexp.MustCompile(`(?:^|\W)else(?:\W|$)`)
	pythonElsePattern     = regexp.MustCompile(`^\s*(?:else\s*:|elif\b)`)
	envVarGuardPattern    = regexp.MustCompile(`if\s+os\.Getenv\(\s*"[^"]*"\s*\)\s*[!=]=`)
)

// isHelperProcessBlock reports whether a test is a Go exec helper-process
// re-entry point: it either references the conventional GO_WANT_HELPER_PROCESS
// variable or opens with an environment-variable guard that returns
// immediately. Such functions only run as a re-invoked subprocess, so the
// assertion rules must not apply to them.
func isHelperProcessBlock(block testBlock) bool {
	for idx, line := range block.lines {
		if strings.Contains(line, "GO_WANT_HELPER_PROCESS") {
			return true
		}
		if envVarGuardPattern.MatchString(line) && nextNonBlankLineIsReturn(block.lines, idx+1) {
			return true
		}
	}
	return false
}

func shouldSkipTestQualityBlock(language string, block testBlock) bool {
	if isHelperProcessBlock(block) {
		return true
	}
	return isGoTestMainBlock(language, block)
}

func isGoTestMainBlock(language string, block testBlock) bool {
	if language != "" && language != "go" {
		return false
	}
	if block.name != "TestMain" || len(block.lines) == 0 {
		return false
	}
	return strings.Contains(block.lines[0], "*testing.M")
}

func nextNonBlankLineIsReturn(lines []string, start int) bool {
	for idx := start; idx < len(lines); idx++ {
		trimmed := strings.TrimSpace(lines[idx])
		if trimmed == "" {
			continue
		}
		return trimmed == "return"
	}
	return false
}

func extractTestBlocks(language string, text string) []testBlock {
	lines := strings.Split(text, "\n")
	switch language {
	case "", "go":
		return delimitedTestBlocks(lines, func(line string) (string, bool) {
			match := goTestDeclPattern.FindStringSubmatch(line)
			if match == nil {
				return "", false
			}
			return match[1], true
		}, '{', '}')
	case "typescript", "javascript", "ts", "tsx", "js", "jsx":
		return delimitedTestBlocks(lines, func(line string) (string, bool) {
			match := jsTestDeclPattern.FindStringSubmatch(line)
			if match == nil {
				return "", false
			}
			name := match[1] + match[2]
			if name == "" {
				name = "(unnamed)"
			}
			return name, true
		}, '(', ')')
	case "python", "py":
		return pythonTestBlocks(lines)
	default:
		return nil
	}
}

// delimitedTestBlocks collects blocks for brace or parenthesis delimited
// languages by balancing the open/close runes from the declaration line on.
func delimitedTestBlocks(lines []string, matchDecl func(string) (string, bool), open rune, closing rune) []testBlock {
	blocks := make([]testBlock, 0)
	for idx := 0; idx < len(lines); idx++ {
		name, ok := matchDecl(lines[idx])
		if !ok {
			continue
		}
		block := testBlock{name: name, startLine: idx + 1}
		depth := 0
		started := false
		for ; idx < len(lines); idx++ {
			block.lines = append(block.lines, lines[idx])
			if braceElsePattern.MatchString(lines[idx]) {
				block.hasElse = true
			}
			depth += strings.Count(lines[idx], string(open)) - strings.Count(lines[idx], string(closing))
			started = started || strings.ContainsRune(lines[idx], open)
			if started && depth <= 0 {
				break
			}
		}
		blocks = append(blocks, block)
	}
	return blocks
}

func pythonTestBlocks(lines []string) []testBlock {
	blocks := make([]testBlock, 0)
	for idx := 0; idx < len(lines); idx++ {
		match := pythonTestDeclPattern.FindStringSubmatch(lines[idx])
		if match == nil {
			continue
		}
		baseIndent := len(match[1])
		block := testBlock{name: match[2], startLine: idx + 1, lines: []string{lines[idx]}}
		end := idx
		for next := idx + 1; next < len(lines); next++ {
			if strings.TrimSpace(lines[next]) == "" {
				continue
			}
			if indentWidth(lines[next]) <= baseIndent {
				break
			}
			for fill := end + 1; fill <= next; fill++ {
				block.lines = append(block.lines, lines[fill])
				if pythonElsePattern.MatchString(lines[fill]) {
					block.hasElse = true
				}
			}
			end = next
		}
		blocks = append(blocks, block)
		idx = end
	}
	return blocks
}

func indentWidth(line string) int {
	width := 0
	for _, char := range line {
		switch char {
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
