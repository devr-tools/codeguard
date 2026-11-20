package rules

import "github.com/devr-tools/codeguard/internal/codeguard/rules/ruledocs"

// DocsPath is the repository-relative rule reference (see ruledocs.Path).
const DocsPath = ruledocs.Path

// DocsURL is the published rule reference (see ruledocs.URL).
const DocsURL = ruledocs.URL

// DocsAnchor returns the stable anchor for ruleID in the rule index.
func DocsAnchor(ruleID string) string { return ruledocs.Anchor(ruleID) }

// DocsRuleURL returns the absolute help link for ruleID.
func DocsRuleURL(ruleID string) string { return ruledocs.RuleURL(ruleID) }
