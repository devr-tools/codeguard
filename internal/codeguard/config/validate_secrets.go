package config

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/devr-tools/codeguard/internal/codeguard/core"
)

func validateSecretsRules(secrets *core.SecretsRulesConfig) error {
	if secrets == nil {
		return nil
	}
	for idx, pattern := range secrets.AllowPatterns {
		if _, err := regexp.Compile(pattern); err != nil {
			return fmt.Errorf("security_rules.secrets.allow_patterns[%d] invalid regex: %w", idx, err)
		}
	}
	for _, custom := range secrets.CustomPatterns {
		if err := validateSecretCustomPattern(custom); err != nil {
			return err
		}
	}
	return validateSecretEntropy(secrets.Entropy)
}

func validateSecretCustomPattern(custom core.CustomSecretPattern) error {
	if strings.TrimSpace(custom.ID) == "" {
		return fmt.Errorf("security_rules.secrets.custom_patterns has an entry with an empty id")
	}
	if strings.TrimSpace(custom.Regex) == "" {
		return fmt.Errorf("security_rules.secrets.custom_patterns[%q].regex is required", custom.ID)
	}
	if _, err := regexp.Compile(custom.Regex); err != nil {
		return fmt.Errorf("security_rules.secrets.custom_patterns[%q] invalid regex: %w", custom.ID, err)
	}
	if !validSecretLevel(custom.Level) {
		return fmt.Errorf("security_rules.secrets.custom_patterns[%q].level must be warn or fail", custom.ID)
	}
	return nil
}

func validateSecretEntropy(entropy *core.SecretsEntropyConfig) error {
	if entropy == nil {
		return nil
	}
	if entropy.MinLength < 0 {
		return fmt.Errorf("security_rules.secrets.entropy.min_length must not be negative, got %d", entropy.MinLength)
	}
	if entropy.Threshold < 0 {
		return fmt.Errorf("security_rules.secrets.entropy.threshold must not be negative, got %g", entropy.Threshold)
	}
	if !validSecretLevel(entropy.Level) {
		return fmt.Errorf("security_rules.secrets.entropy.level must be warn or fail")
	}
	return nil
}

func validSecretLevel(level string) bool {
	switch strings.TrimSpace(strings.ToLower(level)) {
	case "", "warn", "fail":
		return true
	default:
		return false
	}
}
