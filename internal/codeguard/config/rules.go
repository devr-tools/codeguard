package config

import (
	"sort"
	"strings"

	"github.com/devr-tools/codeguard/internal/codeguard/core"
	rulespkg "github.com/devr-tools/codeguard/internal/codeguard/rules"
)

func RuleList() []core.RuleMetadata {
	return ruleListFromCatalog(rulespkg.Catalog())
}

func RuleListForConfig(cfg core.Config) []core.RuleMetadata {
	ApplyDefaults(&cfg)
	return ruleListFromCatalog(RuleCatalogForConfig(cfg))
}

func ExplainRule(ruleID string) (core.RuleMetadata, bool) {
	meta, ok := rulespkg.Catalog()[strings.TrimSpace(ruleID)]
	return meta, ok
}

func ExplainRuleForConfig(cfg core.Config, ruleID string) (core.RuleMetadata, bool) {
	ApplyDefaults(&cfg)
	meta, ok := RuleCatalogForConfig(cfg)[strings.TrimSpace(ruleID)]
	return meta, ok
}

func RuleCatalogForConfig(cfg core.Config) map[string]core.RuleMetadata {
	out := map[string]core.RuleMetadata{}
	for id, meta := range rulespkg.Catalog() {
		out[id] = meta
	}
	for _, pack := range cfg.RulePacks {
		for _, rule := range pack.Rules {
			out[rule.ID] = buildCustomRuleMetadata(rule)
		}
	}
	return out
}

func buildCustomRuleMetadata(rule core.CustomRuleConfig) core.RuleMetadata {
	section := strings.TrimSpace(rule.Section)
	if section == "" {
		section = "Custom Rules"
	}

	severity := strings.TrimSpace(strings.ToLower(rule.Severity))
	if severity == "" {
		severity = "warn"
	}

	return core.RuleMetadata{
		ID:           rule.ID,
		Section:      section,
		DefaultLevel: severity,
		Title:        rule.Title,
		Description:  firstNonEmpty(rule.Description, rule.Message),
		HowToFix:     rule.HowToFix,
	}
}

func ruleListFromCatalog(catalog map[string]core.RuleMetadata) []core.RuleMetadata {
	out := make([]core.RuleMetadata, 0, len(catalog))
	for _, meta := range catalog {
		out = append(out, meta)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	return out
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return value
		}
	}
	return ""
}
