package quality

import "github.com/devr-tools/codeguard/codeguard/core"

type qualityRules struct {
	maxFileLines            int
	maxFunctionLines        int
	maxParameters           int
	maxCyclomaticComplexity int
}

func resolveQualityRules(cfg core.QualityRulesConfig) qualityRules {
	rules := qualityRules{
		maxFileLines:            400,
		maxFunctionLines:        80,
		maxParameters:           5,
		maxCyclomaticComplexity: 10,
	}
	if cfg.MaxFileLines > 0 {
		rules.maxFileLines = cfg.MaxFileLines
	}
	if cfg.MaxFunctionLines > 0 {
		rules.maxFunctionLines = cfg.MaxFunctionLines
	}
	if cfg.MaxParameters > 0 {
		rules.maxParameters = cfg.MaxParameters
	}
	if cfg.MaxCyclomaticComplexity > 0 {
		rules.maxCyclomaticComplexity = cfg.MaxCyclomaticComplexity
	}
	return rules
}
