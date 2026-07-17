package security

import "regexp"

// stripTaintSanitizerCalls removes complete sanitizer-call spans before an
// expression is searched for tainted identifiers. Language-specific callers
// supply patterns whose first capture includes the opening parenthesis.
func stripTaintSanitizerCalls(text string, pattern *regexp.Regexp) string {
	for {
		match := pattern.FindStringSubmatchIndex(text)
		if match == nil {
			return text
		}
		openParen := match[3] - 1
		closeParen := matchingParenOffset(text, openParen)
		if closeParen < 0 {
			return text[:match[2]] + text[match[3]:]
		}
		text = text[:match[2]] + text[closeParen+1:]
	}
}
