package config

import (
	"errors"
	"strings"

	"github.com/devr-tools/codeguard/internal/codeguard/core"
)

func validateContractRules(rules core.ContractRulesConfig) error {
	for _, path := range rules.MigrationPaths {
		if strings.TrimSpace(path) == "" {
			return errors.New("contract_rules.migration_paths must not contain empty entries")
		}
	}
	return nil
}
