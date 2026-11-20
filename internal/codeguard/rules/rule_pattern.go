package rules

import "strings"

// RulePatternKind classifies the rule selector of a waiver or an inline
// codeguard:ignore directive.
type RulePatternKind int

const (
	// RulePatternExact names one rule id (or a retired alias of one).
	RulePatternExact RulePatternKind = iota
	// RulePatternAll is the bare "*" wildcard, matching every rule.
	RulePatternAll
	// RulePatternPrefix is "<prefix>.*", matching every rule id that starts
	// with "<prefix>." (for example "security.*" or "security.taint.*").
	RulePatternPrefix
	// RulePatternInvalid uses "*" anywhere else ("security*", "*.go",
	// "security.*.go"). It matches nothing, and callers report it so the
	// author learns the selector did nothing.
	RulePatternInvalid
)

// ClassifyRulePattern returns the kind of pattern and, for a prefix pattern,
// the prefix including its trailing dot ("security." for "security.*").
func ClassifyRulePattern(pattern string) (RulePatternKind, string) {
	pattern = strings.TrimSpace(pattern)
	if pattern == "*" {
		return RulePatternAll, ""
	}
	if !strings.Contains(pattern, "*") {
		return RulePatternExact, ""
	}
	prefix, ok := strings.CutSuffix(pattern, ".*")
	if !ok || prefix == "" || strings.Contains(prefix, "*") || strings.HasPrefix(prefix, ".") || strings.HasSuffix(prefix, ".") || strings.Contains(prefix, "..") {
		return RulePatternInvalid, ""
	}
	return RulePatternPrefix, prefix + "."
}

// MatchRulePattern reports whether pattern selects ruleID. An exact pattern
// is resolved through the retired-alias table; a prefix pattern matches the
// canonical id by dotted prefix; an invalid pattern matches nothing.
func MatchRulePattern(pattern string, ruleID string) bool {
	kind, prefix := ClassifyRulePattern(pattern)
	switch kind {
	case RulePatternAll:
		return true
	case RulePatternPrefix:
		return strings.HasPrefix(ruleID, prefix)
	case RulePatternExact:
		return ResolveRuleAlias(pattern) == ruleID
	default:
		return false
	}
}
