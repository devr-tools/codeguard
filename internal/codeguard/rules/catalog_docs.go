package rules

import "strings"

// DocsURL is the published rule reference. SARIF helpUri values and other
// rule links point at per-rule anchors inside it.
const DocsURL = "https://github.com/devr-tools/codeguard/blob/main/docs/checks.md"

// DocsAnchor returns the stable docs/checks.md anchor for ruleID: "rule-"
// followed by the id with dots replaced by hyphens, so security.taint.go
// becomes rule-security-taint-go. The scheme is deliberately mechanical so
// anyone adding a rule can write its anchor without a generator.
func DocsAnchor(ruleID string) string {
	return "rule-" + strings.ReplaceAll(strings.TrimSpace(ruleID), ".", "-")
}

// DocsRuleURL returns the absolute help link for ruleID.
func DocsRuleURL(ruleID string) string {
	return DocsURL + "#" + DocsAnchor(ruleID)
}
