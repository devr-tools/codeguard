// Package ruledocs builds links into codeguard's rule reference. It is a leaf
// package (standard library only) so lightweight binaries such as the GitHub
// Action helpers can link rules without depending on the rule catalog and,
// through it, internal/codeguard/core.
package ruledocs

import "strings"

// Path is the repository-relative rule reference: every catalog rule has a
// row there carrying its stable anchor.
const Path = "docs/rule-index.md"

// URL is the published rule reference. SARIF helpUri values, `explain` docs
// links and other rule links point at per-rule anchors inside it.
const URL = "https://github.com/devr-tools/codeguard/blob/main/" + Path

// Anchor returns the stable anchor for ruleID in the rule index: "rule-"
// followed by the id with dots replaced by hyphens, so security.taint.go
// becomes rule-security-taint-go. The scheme is deliberately mechanical so
// anyone adding a rule can write its anchor without a generator.
func Anchor(ruleID string) string {
	return "rule-" + strings.ReplaceAll(strings.TrimSpace(ruleID), ".", "-")
}

// RuleURL returns the absolute help link for ruleID.
func RuleURL(ruleID string) string {
	return URL + "#" + Anchor(ruleID)
}
