package codeguard

import "github.com/devr-tools/codeguard/internal/codeguard/config"

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
