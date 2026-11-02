package support

import (
	"regexp"
	"strings"
)

// Go repositories that already run gosec through golangci-lint review each
// subprocess call site once and record the decision as a justified directive.
// Requiring a second, codeguard-specific comment on the same line would only
// duplicate that review, so a justified gosec directive is accepted as an
// inline suppression, but only for the codeguard rule(s) that report the same
// risk at the same call site, and only when the directive states why.
//
// The mapping is deliberately small. `//nolint:gosec` silences every gosec
// rule on its line, so it is translated only for security.shell-execution: an
// exec call site is where gosec's G204 fires, and a nolint:gosec on that line
// is that review. Taint, secret, and crypto findings are never covered,
// because a linter-wide nolint says nothing about which risk was reviewed.
var gosecRuleEquivalents = map[string][]string{
	"G204": {"security.shell-execution"},
}

// nolintGosecRuleIDs are the codeguard rules a justified //nolint:gosec covers.
var nolintGosecRuleIDs = gosecRuleEquivalents["G204"]

var (
	// nolintDirectivePattern is golangci-lint's machine-readable form: no space
	// after the slashes, an explicit linter list, and a "// reason" trailer.
	nolintDirectivePattern = regexp.MustCompile(`//nolint:([A-Za-z0-9_,\-]+)[ \t]*//[ \t]*(.*)`)
	// nosecDirectivePattern is gosec's own form: "#nosec G204 -- reason".
	nosecDirectivePattern = regexp.MustCompile(`#nosec\b([^\n]*?)--[ \t]*(.*)`)
	justificationLetter   = regexp.MustCompile(`[A-Za-z]`)
)

// SuppressionMatchGosecJustification labels a finding suppressed by a
// justified gosec directive in core.Suppression.Match.
const SuppressionMatchGosecJustification = "gosec-justification"

// gosecJustifiedSuppressions translates the justified gosec directives on one
// Go source line into codeguard inline suppressions. A trailing directive
// covers its own line; a comment-only line covers the line below it, which is
// the statement golangci-lint and gosec attach a standalone directive to.
func gosecJustifiedSuppressions(line string, lineNo int) []inlineSuppression {
	if !strings.Contains(line, "nolint:") && !strings.Contains(line, "#nosec") {
		return nil
	}
	target := lineNo
	if trimmed := strings.TrimSpace(line); strings.HasPrefix(trimmed, "//") || strings.HasPrefix(trimmed, "/*") {
		target = lineNo + 1
	}
	ruleIDs := justifiedGosecRuleIDs(line)
	out := make([]inlineSuppression, 0, len(ruleIDs))
	for _, ruleID := range ruleIDs {
		out = append(out, inlineSuppression{ruleID: ruleID, line: target, exactLine: true, match: SuppressionMatchGosecJustification})
	}
	return out
}

// justifiedGosecRuleIDs returns the codeguard rule IDs covered by the gosec
// directives on line, ignoring any directive without a justification.
func justifiedGosecRuleIDs(line string) []string {
	var ruleIDs []string
	if match := nolintDirectivePattern.FindStringSubmatch(line); match != nil && hasJustification(match[2]) && listsLinter(match[1], "gosec") {
		ruleIDs = append(ruleIDs, nolintGosecRuleIDs...)
	}
	if match := nosecDirectivePattern.FindStringSubmatch(line); match != nil && hasJustification(match[2]) {
		for _, gosecRule := range strings.FieldsFunc(match[1], func(r rune) bool { return r == ' ' || r == '\t' || r == ',' }) {
			ruleIDs = append(ruleIDs, gosecRuleEquivalents[strings.ToUpper(gosecRule)]...)
		}
	}
	return ruleIDs
}

func listsLinter(list string, linter string) bool {
	for _, name := range strings.Split(list, ",") {
		if strings.EqualFold(strings.TrimSpace(name), linter) {
			return true
		}
	}
	return false
}

// hasJustification rejects an empty or punctuation-only reason, so a bare
// "//nolint:gosec //" or "#nosec G204 --" is not an accepted review record.
func hasJustification(reason string) bool {
	reason = strings.TrimSpace(strings.TrimSuffix(strings.TrimSpace(reason), "*/"))
	return justificationLetter.MatchString(reason)
}
