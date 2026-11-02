package rules

import "strings"

// ruleAliases maps retired rule ids that described the same behaviour as a
// live rule to that rule. Unlike RetiredPerformanceRuleIDs, these ids are
// resolved at runtime: a waiver or `codeguard explain` naming the alias acts
// on the canonical rule, so configs written against the old id keep working.
var ruleAliases = map[string]string{
	// quality.mixed-abstraction-levels was catalogued but never emitted; the
	// mixed-abstraction check has always reported function.mixed-abstraction-level.
	"quality.mixed-abstraction-levels": "function.mixed-abstraction-level",
}

// RuleAliases returns a copy of the retired-id -> canonical-id alias table.
func RuleAliases() map[string]string {
	out := make(map[string]string, len(ruleAliases))
	for alias, target := range ruleAliases {
		out[alias] = target
	}
	return out
}

// ResolveRuleAlias returns the canonical rule id for id, or id (trimmed)
// unchanged when it is not an alias.
func ResolveRuleAlias(id string) string {
	trimmed := strings.TrimSpace(id)
	if target, ok := ruleAliases[trimmed]; ok {
		return target
	}
	return trimmed
}
