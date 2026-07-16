package config

import (
	"errors"
	"fmt"
	"strings"

	"github.com/devr-tools/codeguard/internal/codeguard/core"
	rulespkg "github.com/devr-tools/codeguard/internal/codeguard/rules"
)

func validateRulePacks(packs []core.RulePackConfig) error {
	seenRules := builtInRuleIDs()
	for _, pack := range packs {
		if strings.TrimSpace(pack.Name) == "" {
			return errors.New("rule pack name is required")
		}
		if err := validateCustomRules(pack, seenRules); err != nil {
			return err
		}
	}
	return nil
}

func builtInRuleIDs() map[string]struct{} {
	seen := map[string]struct{}{}
	for id := range rulespkg.Catalog() {
		seen[id] = struct{}{}
	}
	return seen
}

func validateCustomRules(pack core.RulePackConfig, seenRules map[string]struct{}) error {
	for _, rule := range pack.Rules {
		if err := validateCustomRule(pack.Name, rule, seenRules); err != nil {
			return err
		}
		seenRules[rule.ID] = struct{}{}
	}
	return nil
}

func validateCustomRule(packName string, rule core.CustomRuleConfig, seenRules map[string]struct{}) error {
	if strings.TrimSpace(rule.ID) == "" {
		return fmt.Errorf("rule pack %q contains a rule with an empty id", packName)
	}
	if _, exists := seenRules[rule.ID]; exists {
		return fmt.Errorf("duplicate rule id %q", rule.ID)
	}
	if strings.TrimSpace(rule.Title) == "" {
		return fmt.Errorf("custom rule %q title is required", rule.ID)
	}
	if strings.TrimSpace(rule.Message) == "" {
		return fmt.Errorf("custom rule %q message is required", rule.ID)
	}
	if err := validateRuleSeverity(rule); err != nil {
		return err
	}
	if err := validateRuleMatchers(rule); err != nil {
		return err
	}
	return validateRuleRegexes(rule)
}
