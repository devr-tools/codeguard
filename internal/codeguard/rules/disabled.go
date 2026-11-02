package rules

import "strings"

// DisabledRuleWildcardSuffix ends a checks.disabled_rules entry that names a
// rule family: "security.taint.*" disables every rule whose id starts with
// "security.taint.".
const DisabledRuleWildcardSuffix = ".*"

// MatchDisabledRule reports whether ruleID is turned off by one of the
// checks.disabled_rules entries, returning the entry that matched. An entry
// is an exact rule id (a retired alias resolves to its canonical rule) or a
// dotted prefix ending in ".*".
func MatchDisabledRule(patterns []string, ruleID string) (string, bool) {
	if ruleID == "" {
		return "", false
	}
	for _, pattern := range patterns {
		trimmed := strings.TrimSpace(pattern)
		if trimmed == "" {
			continue
		}
		if prefix, ok := strings.CutSuffix(trimmed, DisabledRuleWildcardSuffix); ok {
			if prefix != "" && strings.HasPrefix(ruleID, prefix+".") {
				return trimmed, true
			}
			continue
		}
		if trimmed == ruleID || ResolveRuleAlias(trimmed) == ruleID {
			return trimmed, true
		}
	}
	return "", false
}
