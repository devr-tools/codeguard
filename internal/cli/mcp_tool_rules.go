package cli

import (
	"strings"

	service "github.com/devr-tools/codeguard/pkg/codeguard"
)

// resolveExplainRule backs both the MCP explain tool and the
// codeguard://rules/<id> resource, so they return the same payload as
// `codeguard explain -format json`. A named config must load; otherwise the
// server's default config is used when it loads, then built-in defaults.
func (s *mcpToolService) resolveExplainRule(configPath string, profile string, ruleID string) (service.RuleMetadata, *explainConfig, bool, error) {
	path := strings.TrimSpace(configPath)
	if path == "" {
		path = strings.TrimSpace(s.defaultConfigPath)
	}
	ec, err := resolveExplainConfig(path, profile, s.loadConfig, nil)
	if err != nil {
		return service.RuleMetadata{}, nil, false, err
	}
	rule, ok := service.ExplainRuleForConfig(ec.cfg, ruleID)
	return rule, &ec, ok, nil
}
