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
	cfg.Parsers.TreeSitter = strings.ToLower(strings.TrimSpace(cfg.Parsers.TreeSitter))
	if cfg.Parsers.TreeSitter == "" {
		cfg.Parsers.TreeSitter = core.TreeSitterModeOff
	}
}

func applyCheckDefaults(cfg *core.Config, def core.Config) {
	if cfg.Checks.Contracts == nil {
		cfg.Checks.Contracts = def.Checks.Contracts
	}
	if cfg.Checks.Context == nil {
		cfg.Checks.Context = def.Checks.Context
	}
	if cfg.Checks.Reliability == nil {
		cfg.Checks.Reliability = def.Checks.Reliability
	}
	if cfg.Checks.Data == nil {
		cfg.Checks.Data = def.Checks.Data
	}
	if cfg.Checks.Observability == nil {
		cfg.Checks.Observability = def.Checks.Observability
	}
	if cfg.Checks.Operations == nil {
		cfg.Checks.Operations = def.Checks.Operations
	}
	if cfg.Checks.Change == nil {
		cfg.Checks.Change = def.Checks.Change
	}
	applyQualityDefaults(&cfg.Checks.QualityRules, def.Checks.QualityRules)
	applyPerformanceDefaults(&cfg.Checks.PerformanceRules)
	applyDesignDefaults(&cfg.Checks.DesignRules, def.Checks.DesignRules)
	applyPromptDefaults(&cfg.Checks.PromptRules, def.Checks.PromptRules)
	applyCIDefaults(&cfg.Checks.CIRules, def.Checks.CIRules)
	applySecurityDefaults(&cfg.Checks.SecurityRules, def.Checks.SecurityRules)
	applySupplyChainDefaults(&cfg.Checks.SupplyChainRules, def.Checks.SupplyChainRules)
	applyReliabilityDefaults(&cfg.Checks.ReliabilityRules, def.Checks.ReliabilityRules)
	applyDataDefaults(&cfg.Checks.DataRules, def.Checks.DataRules)
	applyObservabilityDefaults(&cfg.Checks.ObservabilityRules, def.Checks.ObservabilityRules)
	applyOperationsDefaults(&cfg.Checks.OperationsRules, def.Checks.OperationsRules)
	applyChangeDefaults(&cfg.Checks.ChangeRules, def.Checks.ChangeRules)
	applyContextDefaults(&cfg.Checks.ContextRules, def.Checks.ContextRules)
	applyContractDefaults(&cfg.Checks.ContractRules, def.Checks.ContractRules)
	applyProductionRiskDefaults(&cfg.Checks.ProductionRisk, def.Checks.ProductionRisk)
	applyAIDefaults(&cfg.AI, def.AI)
	applyCheckActivationDefaults(&cfg.Checks)
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
