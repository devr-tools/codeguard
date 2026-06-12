package config

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/devr-tools/codeguard/internal/codeguard/core"
)

func validateRuleSeverity(rule core.CustomRuleConfig) error {
	switch strings.TrimSpace(strings.ToLower(rule.Severity)) {
	case "", "warn", "fail":
		return nil
	default:
		return fmt.Errorf("custom rule %q severity must be warn or fail", rule.ID)
	}
}

func validateRuleMatchers(rule core.CustomRuleConfig) error {
	if len(rule.Paths) > 0 || strings.TrimSpace(rule.PathRegex) != "" || strings.TrimSpace(rule.ContentRegex) != "" || strings.TrimSpace(rule.NaturalLanguage) != "" || len(rule.FileExtensions) > 0 {
		return nil
	}
	return fmt.Errorf("custom rule %q must define at least one matcher", rule.ID)
}

func validateRuleRegexes(rule core.CustomRuleConfig) error {
	if strings.TrimSpace(rule.NaturalLanguage) != "" && strings.TrimSpace(rule.ContentRegex) != "" {
		return fmt.Errorf("custom rule %q cannot define both natural_language and content_regex", rule.ID)
	}
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
