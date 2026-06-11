package ci

import (
	"path/filepath"
	"regexp"
	"strings"

	"github.com/devr-tools/codeguard/internal/codeguard/checks/support"
	"github.com/devr-tools/codeguard/internal/codeguard/core"
)

func testQualityFindings(env support.Context, target core.TargetConfig) []core.Finding {
	spec, ok := testQualitySpecs[normalizedLanguage(target.Language)]
	if !ok {
		return nil
	}

	return env.ScanTargetFiles(target, "ci-test-quality", func(rel string) bool {
		return isTargetTestFile(target.Language, rel)
	}, func(file string, data []byte) []core.Finding {
		return testQualityFindingsForFile(env, file, string(data), spec)
	})
}

func testQualityFindingsForFile(env support.Context, file string, text string, spec testQualitySpec) []core.Finding {
	if !hasTestDefinition(text, spec.testDefinitionPatterns) {
		return nil
	}

	findings := make([]core.Finding, 0, 2)
	if !containsAnyAssertion(text, spec.assertionTokens, spec.assertionPatterns) {
		findings = append(findings, env.NewFinding(support.FindingInput{
			RuleID:  "ci.test-without-assertion",
			Level:   "fail",
			Path:    file,
			Line:    firstMatchLine(text, spec.testDefinitionPatterns),
			Column:  1,
			Message: "test file defines tests but contains no recognizable assertion or failure signal",
		}))
	}

	for lineNumber, line := range sanitizedLines(text, fileExtension(file)) {
		if strings.TrimSpace(line) == "" {
			continue
		}
		for _, pattern := range spec.alwaysTruePatterns {
			if pattern.MatchString(line) {
				findings = append(findings, env.NewFinding(support.FindingInput{
					RuleID:  "ci.always-true-test-assertion",
					Level:   "fail",
					Path:    file,
					Line:    lineNumber + 1,
					Column:  1,
					Message: "test assertion is always true and does not verify behavior",
				}))
				break
			}
		}
	}

	return findings
}

func hasTestDefinition(text string, patterns []*regexp.Regexp) bool {
	for _, pattern := range patterns {
		if pattern.MatchString(text) {
			return true
		}
	}
	return false
}

func containsAnyAssertion(text string, tokens []string, patterns []*regexp.Regexp) bool {
	for _, line := range sanitizedLines(text, "") {
		if strings.TrimSpace(line) == "" {
			continue
		}
		for _, token := range tokens {
			if strings.Contains(line, token) {
				return true
			}
		}
		for _, pattern := range patterns {
			if pattern.MatchString(line) {
				return true
			}
		}
	}
	return false
}

func firstMatchLine(text string, patterns []*regexp.Regexp) int {
	lines := strings.Split(text, "\n")
	for index, line := range lines {
		for _, pattern := range patterns {
			if pattern.MatchString(line) {
				return index + 1
			}
		}
	}
	return 1
}

func sanitizedLines(text string, ext string) []string {
	lines := strings.Split(text, "\n")
	result := make([]string, len(lines))
	inBlockComment := false
	for index, line := range lines {
		sanitized, blockState := stripCommentContent(line, ext, inBlockComment)
		inBlockComment = blockState
		result[index] = sanitized
	}
	return result
}

func stripCommentContent(line string, ext string, inBlockComment bool) (string, bool) {
	if ext == ".py" || ext == ".rb" {
		return stripLineComment(line, "#"), false
	}

	var out strings.Builder
	state := commentStripState{inBlockComment: inBlockComment}
	for i := 0; i < len(line); {
		next, done := state.advance(line, i, &out)
		if done {
			return out.String(), state.inBlockComment
		}
		i = next
	}

	return out.String(), state.inBlockComment
}

type commentStripState struct {
	inBlockComment bool
	inSingleQuote  bool
	inDoubleQuote  bool
	inBacktick     bool
}

func (state *commentStripState) advance(line string, i int, out *strings.Builder) (int, bool) {
	if state.inBlockComment {
		end := strings.Index(line[i:], "*/")
		if end == -1 {
			return len(line), true
		}
		state.inBlockComment = false
		return i + end + 2, false
	}
	if quote, ok := state.activeQuote(); ok {
		return state.advanceQuoted(line, i, quote, out), false
	}
	if strings.HasPrefix(line[i:], "//") {
		return len(line), true
	}
	if strings.HasPrefix(line[i:], "/*") {
		state.inBlockComment = true
		return i + 2, false
	}
	return state.advanceCode(line, i, out), false
}

func (state *commentStripState) activeQuote() (byte, bool) {
	switch {
	case state.inSingleQuote:
		return '\'', true
	case state.inDoubleQuote:
		return '"', true
	case state.inBacktick:
		return '`', true
	default:
		return 0, false
	}
}

func (state *commentStripState) advanceQuoted(line string, i int, quote byte, out *strings.Builder) int {
	out.WriteByte(line[i])
	if line[i] == '\\' && quote != '`' && i+1 < len(line) {
		out.WriteByte(line[i+1])
		return i + 2
	}
	if line[i] == quote {
		state.clearQuote(quote)
	}
	return i + 1
}

func (state *commentStripState) advanceCode(line string, i int, out *strings.Builder) int {
	switch line[i] {
	case '\'':
		state.inSingleQuote = true
	case '"':
		state.inDoubleQuote = true
	case '`':
		state.inBacktick = true
	}
	out.WriteByte(line[i])
	return i + 1
}

func (state *commentStripState) clearQuote(quote byte) {
	switch quote {
	case '\'':
		state.inSingleQuote = false
	case '"':
		state.inDoubleQuote = false
	case '`':
		state.inBacktick = false
	}
}

func stripLineComment(line string, marker string) string {
	index := strings.Index(line, marker)
	if index == -1 {
		return line
	}
	return line[:index]
}

func fileExtension(path string) string {
	return strings.ToLower(filepath.Ext(path))
}
