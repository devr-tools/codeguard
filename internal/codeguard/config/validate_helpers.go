package config

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/devr-tools/codeguard/internal/codeguard/core"
)

func validateGraphThresholds(rules core.DesignRulesConfig) error {
	if rules.GodModuleThreshold < 0 {
		return fmt.Errorf("design_rules.god_module_threshold must not be negative, got %d", rules.GodModuleThreshold)
	}
	if rules.HighImpactChangeThreshold < 0 {
		return fmt.Errorf("design_rules.high_impact_change_threshold must not be negative, got %d", rules.HighImpactChangeThreshold)
	}
	return nil
}

func validateRuleSeverity(rule core.CustomRuleConfig) error {
	switch strings.TrimSpace(strings.ToLower(rule.Severity)) {
	case "", "warn", "fail":
		return nil
	default:
		return fmt.Errorf("custom rule %q severity must be warn or fail", rule.ID)
	}
}

func validateRuleMatchers(rule core.CustomRuleConfig) error {
	if len(rule.Paths) > 0 || strings.TrimSpace(rule.PathRegex) != "" || strings.TrimSpace(rule.ContentRegex) != "" || len(rule.FileExtensions) > 0 {
		return nil
	}
	return fmt.Errorf("custom rule %q must define at least one matcher", rule.ID)
}

func validateRuleRegexes(rule core.CustomRuleConfig) error {
	if err := validateOptionalRegex(rule.ID, "path_regex", rule.PathRegex); err != nil {
		return err
	}
	return validateOptionalRegex(rule.ID, "content_regex", rule.ContentRegex)
}

func validateOptionalRegex(ruleID string, field string, pattern string) error {
	if strings.TrimSpace(pattern) == "" {
		return nil
	}
	if _, err := regexp.Compile(pattern); err != nil {
		return fmt.Errorf("custom rule %q invalid %s: %w", ruleID, field, err)
	}
	return nil
}
