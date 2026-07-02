package treesitter

import (
	"regexp"
	"strings"

	"github.com/devr-tools/codeguard/internal/codeguard/checks/support"
)

// This file replicates the production implementation of the two rules under
// test, byte for byte: the same regexes as
// internal/codeguard/checks/quality/quality_typescript_helpers.go
// (tsExplicitAnyPattern) and
// internal/codeguard/checks/security/security_typescript.go
// (typeScriptUnsafeHTMLPattern), applied to
// support.StripTypeScriptCommentsAndStrings output with per-line dedupe,
// exactly like support.ScriptRegexFindings does.

var (
	baselineExplicitAnyPattern = regexp.MustCompile(`(?:[:<,(]\s*any\b|\bas\s+any\b)`)
	baselineUnsafeHTMLPattern  = regexp.MustCompile(`(?:\.\s*(?:innerHTML|outerHTML)\s*=|\.\s*insertAdjacentHTML\s*\(|\bdocument\.(?:write|writeln)\s*\()`)
)

// BaselineScan runs the current production logic for both rules and returns
// normalized findings.
func BaselineScan(source []byte) []Finding {
	text := strings.ReplaceAll(string(source), "\r\n", "\n")
	code := support.StripTypeScriptCommentsAndStrings(text)
	findings := baselineRegexFindings(text, code, baselineExplicitAnyPattern, RuleExplicitAny)
	findings = append(findings, baselineRegexFindings(text, code, baselineUnsafeHTMLPattern, RuleUnsafeHTMLSink)...)
	return normalizeFindings(findings)
}

// baselineRegexFindings mirrors support.RegexLineFindings: match offsets on
// the stripped code, line numbers on the original source, one finding per
// line per rule.
func baselineRegexFindings(source string, code string, pattern *regexp.Regexp, rule string) []Finding {
	matches := pattern.FindAllStringIndex(code, -1)
	if len(matches) == 0 {
		return nil
	}
	findings := make([]Finding, 0, len(matches))
	seen := make(map[int]struct{}, len(matches))
	for _, match := range matches {
		line := support.LineNumberForOffset(source, match[0])
		if _, exists := seen[line]; exists {
			continue
		}
		seen[line] = struct{}{}
		findings = append(findings, Finding{Rule: rule, Line: line})
	}
	return findings
}
