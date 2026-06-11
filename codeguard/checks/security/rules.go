package security

import (
	"strings"

	"github.com/devr-tools/codeguard/codeguard/core"
)

type securityRules struct {
	govulncheckMode    string
	govulncheckCommand string
}

func resolveSecurityRules(cfg core.SecurityRulesConfig) securityRules {
	rules := securityRules{
		govulncheckMode:    "auto",
		govulncheckCommand: "govulncheck",
	}
	if strings.TrimSpace(cfg.GovulncheckMode) != "" {
		rules.govulncheckMode = strings.TrimSpace(cfg.GovulncheckMode)
	}
	if strings.TrimSpace(cfg.GovulncheckCommand) != "" {
		rules.govulncheckCommand = strings.TrimSpace(cfg.GovulncheckCommand)
	}
	return rules
}
