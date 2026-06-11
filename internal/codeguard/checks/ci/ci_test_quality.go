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
