package checks_test

import "github.com/devr-tools/codeguard/pkg/codeguard"

func supplyChainTestConfig(dir string, name string) codeguard.Config {
	cfg := codeguard.ExampleConfig()
	cfg.Name = name
	cfg.Targets = []codeguard.TargetConfig{{Name: "repo", Path: dir, Language: "go"}}
	cfg.Checks.Quality = false
	cfg.Checks.Design = false
	cfg.Checks.Security = false
	cfg.Checks.Prompts = false
	cfg.Checks.CI = false
	cfg.Checks.SupplyChain = true
	off := false
	cfg.Checks.Context = &off
	cfg.Cache.Enabled = &off
	return cfg
}

func supplyChainRuleMessages(report codeguard.Report, ruleID string) []string {
	messages := make([]string, 0)
	for _, section := range report.Sections {
		if section.ID != "supply_chain" {
			continue
		}
		for _, finding := range section.Findings {
			if finding.RuleID == ruleID {
				messages = append(messages, finding.Message)
			}
		}
	}
	return messages
}
