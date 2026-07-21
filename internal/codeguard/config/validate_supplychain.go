package config

import (
	"errors"
	"strings"

	"github.com/devr-tools/codeguard/internal/codeguard/core"
)

func validateSupplyChainRules(cfg core.SupplyChainRulesConfig) error {
	if cfg.DetectVulnerabilities != nil && *cfg.DetectVulnerabilities && strings.TrimSpace(cfg.AdvisoryCachePath) == "" {
		return errors.New("supply_chain_rules.advisory_cache_path is required when detect_vulnerabilities is enabled")
	}
	if err := validateNonEmptyStrings("supply_chain_rules.allowed_licenses", cfg.AllowedLicenses); err != nil {
		return err
	}
	if err := validateNonEmptyStrings("supply_chain_rules.denied_licenses", cfg.DeniedLicenses); err != nil {
		return err
	}
	if err := validateSingleCommandMap("supply_chain_rules.license_commands", cfg.LicenseCommands); err != nil {
		return err
	}

	allowed := make(map[string]struct{}, len(cfg.AllowedLicenses))
	for _, license := range cfg.AllowedLicenses {
		allowed[strings.ToLower(strings.TrimSpace(license))] = struct{}{}
	}
	for _, license := range cfg.DeniedLicenses {
		if _, ok := allowed[strings.ToLower(strings.TrimSpace(license))]; ok {
			return errors.New("supply_chain_rules.allowed_licenses and denied_licenses must not overlap")
		}
	}
	return nil
}

func validateSingleCommandMap(field string, commands map[string]core.CommandCheckConfig) error {
	for key, check := range commands {
		if strings.TrimSpace(key) == "" {
			return errors.New(field + " contains an empty ecosystem key")
		}
		if strings.TrimSpace(check.Name) == "" {
			return errors.New(field + "[" + key + "].name is required")
		}
		if strings.TrimSpace(check.Command) == "" {
			return errors.New(field + "[" + key + "].command is required")
		}
	}
	return nil
}

func validateNonEmptyStrings(field string, values []string) error {
	for _, value := range values {
		if strings.TrimSpace(value) == "" {
			return errors.New(field + " must not contain empty entries")
		}
	}
	return nil
}
