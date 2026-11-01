package rules

import "strings"

// DocsPath is the repository-relative rule reference: every catalog rule has a
// row there carrying its stable anchor.
const DocsPath = "docs/rule-index.md"

// DocsURL is the published rule reference. SARIF helpUri values, `explain`
// docs links and other rule links point at per-rule anchors inside it.
const DocsURL = "https://github.com/devr-tools/codeguard/blob/main/" + DocsPath

// DocsAnchor returns the stable anchor for ruleID in the rule index: "rule-"
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
