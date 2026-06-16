package ci

import (
	"regexp"

	"github.com/devr-tools/codeguard/internal/codeguard/checks/support"
	"github.com/devr-tools/codeguard/internal/codeguard/core"
)

// testQualityFindings applies the regex-based test assertion rules
// (ci.test-without-assertion, ci.always-true-test-assertion,
// ci.conditional-assertion) to a target's test files.
func testQualityFindings(env support.Context, target core.TargetConfig) []core.Finding {
	rules := env.Config.Checks.CIRules.TestQuality
	if rules.Enabled != nil && !*rules.Enabled {
		return nil
	}
	language := normalizedLanguage(target.Language)
	patterns, ok := testQualityPatternsFor(language)
	if !ok {
		return nil
	}
	helpers := assertionHelperPattern(rules.AssertionHelpers)
	return env.ScanTargetFiles(target, "ci", func(rel string) bool {
		return isTargetTestFile(target.Language, rel)
	}, func(file string, data []byte) []core.Finding {
		findings := make([]core.Finding, 0)
		for _, block := range extractTestBlocks(language, string(data)) {
			if isHelperProcessBlock(block) {
				continue
			}
			findings = append(findings, evaluateTestBlock(env, patterns, helpers, file, block)...)
		}
		return findings
	})
}

func evaluateTestBlock(env support.Context, patterns testQualityPatterns, helpers *regexp.Regexp, file string, block testBlock) []core.Finding {
	asserts := assertionLinesForBlock(patterns, helpers, block)
	if len(asserts) == 0 {
		return []core.Finding{env.NewFinding(support.FindingInput{
			RuleID:  "ci.test-without-assertion",
			Level:   "warn",
			Path:    file,
			Line:    block.startLine,
			Column:  1,
			Message: "test " + block.name + " contains no recognizable assertion",
		})}
	}
	if allConstantAssertions(asserts) {
		return []core.Finding{env.NewFinding(support.FindingInput{
			RuleID:  "ci.always-true-test-assertion",
			Level:   "warn",
			Path:    file,
			Line:    asserts[0].line,
			Column:  1,
			Message: "test " + block.name + " only asserts on constants and can never fail",
		})}
	}
	if line, flagged := conditionalAssertionLine(asserts, block); flagged {
		return []core.Finding{env.NewFinding(support.FindingInput{
			RuleID:  "ci.conditional-assertion",
			Level:   "warn",
			Path:    file,
			Line:    line,
			Column:  1,
			Message: "test " + block.name + " wraps all assertions in a conditional without an else branch, so they may never run",
		})}
	}
	return nil
}

func allConstantAssertions(asserts []assertionLine) bool {
	for _, assertion := range asserts {
		if !assertion.constant {
			return false
		}
	}
	return true
}

// conditionalAssertionLine reports the first conditionally executed assertion
// when every non-idiomatic assertion in the block is wrapped in a conditional
// and the block has no else branch.
func conditionalAssertionLine(asserts []assertionLine, block testBlock) (int, bool) {
	if block.hasElse {
		return 0, false
	}
	first := 0
	for _, assertion := range asserts {
		if assertion.idiomatic {
			continue
		}
		if !assertion.conditional {
			return 0, false
		}
		if first == 0 {
			first = assertion.line
		}
	}
	return first, first != 0
}
