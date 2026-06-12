package config

import (
	"strings"

	"github.com/devr-tools/codeguard/internal/codeguard/core"
)

func ApplyDefaults(cfg *core.Config) {
	def := defaultConfigForProfile(cfg.Profile)

	applyRootDefaults(cfg, def)
	applyCheckDefaults(cfg, def)
	applyRulePackDefaults(cfg)
}

func defaultConfigForProfile(profile string) core.Config {
	def := baseExampleConfig()
	normalized := normalizeProfile(profile)
	if spec, ok := profileCatalog[normalized]; ok {
		spec.apply(&def)
		def.Profile = normalized
	}
	return def
}

func applyRootDefaults(cfg *core.Config, def core.Config) {
	if cfg.Name == "" {
		cfg.Name = def.Name
	}
	if cfg.Profile == "" {
		cfg.Profile = def.Profile
	} else {
		cfg.Profile = normalizeProfile(cfg.Profile)
	}
	if cfg.Output.Format == "" {
		cfg.Output.Format = def.Output.Format
	}
	if cfg.Cache.Enabled == nil {
		cfg.Cache.Enabled = boolPtr(true)
	}
	if cfg.Cache.Path == "" {
		cfg.Cache.Path = def.Cache.Path
	}
	if cfg.AI.Enabled == nil {
		cfg.AI.Enabled = boolPtr(false)
	}
	if cfg.AI.Cache.Path == "" {
		cfg.AI.Cache.Path = def.AI.Cache.Path
	}
}

func applyCheckDefaults(cfg *core.Config, def core.Config) {
	applyQualityDefaults(&cfg.Checks.QualityRules, def.Checks.QualityRules)
	applyDesignDefaults(&cfg.Checks.DesignRules, def.Checks.DesignRules)
	applyPromptDefaults(&cfg.Checks.PromptRules, def.Checks.PromptRules)
	applyCIDefaults(&cfg.Checks.CIRules, def.Checks.CIRules)
	applySecurityDefaults(&cfg.Checks.SecurityRules, def.Checks.SecurityRules)
	applyAIDefaults(&cfg.AI, def.AI)
}

func applyRulePackDefaults(cfg *core.Config) {
	for packIdx := range cfg.RulePacks {
		for ruleIdx := range cfg.RulePacks[packIdx].Rules {
			rule := &cfg.RulePacks[packIdx].Rules[ruleIdx]
			if strings.TrimSpace(rule.Section) == "" {
				rule.Section = "Custom Rules"
			}
			if strings.TrimSpace(rule.Severity) == "" {
				rule.Severity = "warn"
			}
		}
	}
}
