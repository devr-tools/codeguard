package ci

import (
	"path/filepath"
	"regexp"
	"strings"

	"github.com/devr-tools/codeguard/internal/codeguard/checks/support"
	"github.com/devr-tools/codeguard/internal/codeguard/core"
)

type testQualitySpec struct {
	testDefinitionPatterns []*regexp.Regexp
	assertionTokens        []string
	assertionPatterns      []*regexp.Regexp
	alwaysTruePatterns     []*regexp.Regexp
}

var testQualitySpecs = map[string]testQualitySpec{
	"go": {
		testDefinitionPatterns: []*regexp.Regexp{
			regexp.MustCompile(`(?m)^\s*func\s+Test[[:word:]]*\s*\(`),
		},
		assertionTokens: []string{
			"t.Fatal(", "t.Fatalf(", "t.Error(", "t.Errorf(", "t.Fail(", "t.FailNow(",
			"assert.", "require.", "cmp.Diff(", "panic(",
		},
		assertionPatterns: []*regexp.Regexp{
			regexp.MustCompile(`\bassert[A-Z]\w*\s*\(`),
			regexp.MustCompile(`\brequire[A-Z]\w*\s*\(`),
		},
		alwaysTruePatterns: []*regexp.Regexp{
			regexp.MustCompile(`\b(?:assert|require)\.True\s*\(\s*t\s*,\s*true\s*(?:,|\))`),
			regexp.MustCompile(`\b(?:assert|require)\.(?:Equal|Exactly)\s*\(\s*t\s*,\s*true\s*,\s*true\s*(?:,|\))`),
		},
	},
	"python": {
		testDefinitionPatterns: []*regexp.Regexp{
			regexp.MustCompile(`(?m)^\s*def\s+test_[[:word:]]*\s*\(`),
			regexp.MustCompile(`(?m)^\s*class\s+Test[[:word:]]*[\(:]`),
		},
		assertionTokens: []string{
			"assert ", "self.assert", "pytest.fail(", "pytest.raises(", "raise AssertionError",
		},
		alwaysTruePatterns: []*regexp.Regexp{
			regexp.MustCompile(`^\s*assert\s+True\b`),
			regexp.MustCompile(`\bself\.assertTrue\s*\(\s*True\s*\)`),
		},
	},
	"typescript": {
		testDefinitionPatterns: []*regexp.Regexp{
			regexp.MustCompile(`\b(?:it|test)\s*\(`),
		},
		assertionTokens: []string{
			"expect(", "assert.", "assert(", "should.", ".should(", "toThrow(",
		},
		alwaysTruePatterns: []*regexp.Regexp{
			regexp.MustCompile(`\bexpect\s*\(\s*true\s*\)\s*\.(?:toBe|toEqual|toStrictEqual)\s*\(\s*true\s*\)`),
			regexp.MustCompile(`\bassert(?:\.ok)?\s*\(\s*true\s*(?:,|\))`),
		},
	},
	"javascript": {
		testDefinitionPatterns: []*regexp.Regexp{
			regexp.MustCompile(`\b(?:it|test)\s*\(`),
		},
		assertionTokens: []string{
			"expect(", "assert.", "assert(", "should.", ".should(", "toThrow(",
		},
		alwaysTruePatterns: []*regexp.Regexp{
			regexp.MustCompile(`\bexpect\s*\(\s*true\s*\)\s*\.(?:toBe|toEqual|toStrictEqual)\s*\(\s*true\s*\)`),
			regexp.MustCompile(`\bassert(?:\.ok)?\s*\(\s*true\s*(?:,|\))`),
		},
	},
	"rust": {
		testDefinitionPatterns: []*regexp.Regexp{
			regexp.MustCompile(`(?m)^\s*#\s*\[\s*test\s*\]`),
		},
		assertionTokens: []string{
			"assert!(", "assert_eq!(", "assert_ne!(", "panic!(",
		},
		alwaysTruePatterns: []*regexp.Regexp{
			regexp.MustCompile(`\bassert!\s*\(\s*true\s*\)`),
			regexp.MustCompile(`\bassert_eq!\s*\(\s*true\s*,\s*true\s*\)`),
		},
	},
	"java": {
		testDefinitionPatterns: []*regexp.Regexp{
			regexp.MustCompile(`(?m)^\s*@Test\b`),
			regexp.MustCompile(`(?m)^\s*public\s+void\s+test[[:word:]]*\s*\(`),
		},
		assertionTokens: []string{
			"assert", "Assertions.", "assertThat(", "fail(",
		},
		alwaysTruePatterns: []*regexp.Regexp{
			regexp.MustCompile(`\bassertTrue\s*\(\s*true\s*\)`),
			regexp.MustCompile(`\bAssertions\.assertTrue\s*\(\s*true\s*\)`),
		},
	},
	"csharp": {
		testDefinitionPatterns: []*regexp.Regexp{
			regexp.MustCompile(`(?m)^\s*\[\s*(?:Fact|Theory|Test)\s*\]`),
		},
		assertionTokens: []string{
			"Assert.", "Should().", "FluentActions.",
		},
		alwaysTruePatterns: []*regexp.Regexp{
			regexp.MustCompile(`\bAssert\.True\s*\(\s*true\s*\)`),
		},
	},
	"ruby": {
		testDefinitionPatterns: []*regexp.Regexp{
			regexp.MustCompile(`(?m)^\s*def\s+test_[[:word:]]*[!?=]?\s*$`),
			regexp.MustCompile(`\b(?:it|specify|test)\s+["']`),
		},
		assertionTokens: []string{
			"assert", "refute", "expect(", "raise_error",
		},
		alwaysTruePatterns: []*regexp.Regexp{
			regexp.MustCompile(`\bassert\s*\(?\s*true\s*\)?`),
			regexp.MustCompile(`\bexpect\s*\(\s*true\s*\)\.to\s+eq\s*\(\s*true\s*\)`),
		},
	},
}

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
	i := 0
	inSingleQuote := false
	inDoubleQuote := false
	inBacktick := false
	for i < len(line) {
		if inBlockComment {
			end := strings.Index(line[i:], "*/")
			if end == -1 {
				return out.String(), true
			}
			i += end + 2
			inBlockComment = false
			continue
		}

		switch {
		case inSingleQuote:
			out.WriteByte(line[i])
			if line[i] == '\\' && i+1 < len(line) {
				i++
				out.WriteByte(line[i])
			} else if line[i] == '\'' {
				inSingleQuote = false
			}
			i++
		case inDoubleQuote:
			out.WriteByte(line[i])
			if line[i] == '\\' && i+1 < len(line) {
				i++
				out.WriteByte(line[i])
			} else if line[i] == '"' {
				inDoubleQuote = false
			}
			i++
		case inBacktick:
			out.WriteByte(line[i])
			if line[i] == '`' {
				inBacktick = false
			}
			i++
		case strings.HasPrefix(line[i:], "//"):
			return out.String(), false
		case strings.HasPrefix(line[i:], "/*"):
			inBlockComment = true
			i += 2
		case line[i] == '\'':
			inSingleQuote = true
			out.WriteByte(line[i])
			i++
		case line[i] == '"':
			inDoubleQuote = true
			out.WriteByte(line[i])
			i++
		case line[i] == '`':
			inBacktick = true
			out.WriteByte(line[i])
			i++
		default:
			out.WriteByte(line[i])
			i++
		}
	}

	return out.String(), inBlockComment
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
