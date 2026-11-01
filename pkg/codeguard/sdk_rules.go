package codeguard

import (
	"github.com/devr-tools/codeguard/internal/codeguard/config"
	"github.com/devr-tools/codeguard/internal/codeguard/rules"
)

func Rules() []RuleMetadata {
	return config.RuleList()
}

func RulesForConfig(cfg Config) []RuleMetadata {
	return config.RuleListForConfig(cfg)
}

func ExplainRule(ruleID string) (RuleMetadata, bool) {
	return config.ExplainRule(ruleID)
}

func ExplainRuleForConfig(cfg Config, ruleID string) (RuleMetadata, bool) {
	return config.ExplainRuleForConfig(cfg, ruleID)
}

// RuleSectionKey returns the section id a rule reports under (quality,
// security, supply_chain, ...): the id checks.<section> toggles and
// checks.disabled use.
func RuleSectionKey(rule RuleMetadata) string {
	return config.RuleSectionKey(rule)
}

// RuleEnabled reports whether rule runs under cfg (defaults applied) in a full
// scan: every enable key in rule.ConfigKeys resolves to true.
func RuleEnabled(cfg Config, rule RuleMetadata) bool {
	return config.RuleEnabled(cfg, rule)
}

// RuleProfileNames lists the built-in profiles, plus "default" for no
// profile, under which rule runs on top of the recommended sections.
func RuleProfileNames(rule RuleMetadata) []string {
	return config.RuleProfileNames(rule)
}

// RuleAliases lists the retired rule ids that resolve to ruleID.
func RuleAliases(ruleID string) []string {
	return rules.AliasesFor(ruleID)
}

// RuleDocsPath returns the repository-relative docs link for a rule:
// docs/checks.md#rule-<id with dots replaced by dashes>.
func RuleDocsPath(ruleID string) string {
	return rules.RuleDocsPath(ruleID)
}

// KnownConfigKey reports whether a dotted yaml path (the `scan -set` syntax,
// without the value) names a config field.
func KnownConfigKey(key string) bool {
	return config.KnownConfigKey(key)
}

// LookupConfigKey returns the value at a dotted yaml path in cfg; an unset
// pointer field yields (nil, true).
func LookupConfigKey(cfg Config, key string) (any, bool) {
	return config.LookupConfigKey(cfg, key)
}
