package cli

import (
	"strings"

	service "github.com/devr-tools/codeguard/pkg/codeguard"
)

func (s *mcpToolService) resolveExplainRule(configPath string, profile string, ruleID string) (service.RuleMetadata, bool, error) {
	if strings.TrimSpace(configPath) != "" {
		cfg, err := s.loadConfig(configPath, profile)
		if err != nil {
			return service.RuleMetadata{}, false, err
		}
		rule, ok := service.ExplainRuleForConfig(cfg, ruleID)
		return rule, ok, nil
	}

	rule, ok := service.ExplainRule(ruleID)
	if strings.TrimSpace(profile) == "" {
		return rule, ok, nil
	}
	cfg, err := s.loadConfig("", profile)
	if err != nil {
		return service.RuleMetadata{}, false, err
	}
	rule, ok = service.ExplainRuleForConfig(cfg, ruleID)
	return rule, ok, nil
}
