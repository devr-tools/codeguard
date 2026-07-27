package config

import "github.com/devr-tools/codeguard/internal/codeguard/core"

func applyCheckActivationDefaults(checks *core.CheckConfig) {
	if checks.UseRecommendedDefaults {
		enableRecommendedChecks(checks)
	}
	for _, disabled := range checks.Disabled {
		if disable, ok := checkDisablers[disabled]; ok {
			disable(checks)
		}
	}
}

func enableRecommendedChecks(checks *core.CheckConfig) {
	checks.Quality = true
	checks.Design = true
	checks.Security = true
	checks.Prompts = true
	checks.CI = true
}

var checkDisablers = map[string]func(*core.CheckConfig){
	"quality":      func(checks *core.CheckConfig) { checks.Quality = false },
	"performance":  func(checks *core.CheckConfig) { checks.Performance = boolPtr(false) },
	"design":       func(checks *core.CheckConfig) { checks.Design = false },
	"security":     func(checks *core.CheckConfig) { checks.Security = false },
	"prompts":      func(checks *core.CheckConfig) { checks.Prompts = false },
	"ci":           func(checks *core.CheckConfig) { checks.CI = false },
	"supply_chain": func(checks *core.CheckConfig) { checks.SupplyChain = false },
	"reliability":  func(checks *core.CheckConfig) { checks.Reliability = boolPtr(false) },
	"data":         func(checks *core.CheckConfig) { checks.Data = boolPtr(false) },
	"change":       func(checks *core.CheckConfig) { checks.Change = boolPtr(false) },
	"context":      func(checks *core.CheckConfig) { checks.Context = boolPtr(false) },
	"contracts":    func(checks *core.CheckConfig) { checks.Contracts = boolPtr(false) },
}
