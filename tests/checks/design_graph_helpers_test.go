package checks_test

import (
	"testing"

	"github.com/devr-tools/codeguard/pkg/codeguard"
)

func graphTestConfig(name string, dir string, language string) codeguard.Config {
	cfg := codeguard.ExampleConfig()
	cfg.Name = name
	cfg.Targets = []codeguard.TargetConfig{{Name: "repo", Path: dir, Language: language}}
	cfg.Checks.Design = true
	cfg.Checks.Quality = false
	cfg.Checks.Security = false
	cfg.Checks.Prompts = false
	cfg.Checks.CI = false
	return cfg
}

func assertFindingRuleAbsent(t *testing.T, report codeguard.Report, section string, ruleID string) {
	t.Helper()
	for _, result := range report.Sections {
		if result.Name != section {
			continue
		}
		for _, finding := range result.Findings {
			if finding.RuleID == ruleID {
				t.Fatalf("section %q unexpectedly contains rule %q: %s", section, ruleID, finding.Message)
			}
		}
		return
	}
	t.Fatalf("section %q not found", section)
}
