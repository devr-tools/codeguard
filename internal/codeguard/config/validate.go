package config

import (
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/devr-tools/codeguard/internal/codeguard/core"
	rulespkg "github.com/devr-tools/codeguard/internal/codeguard/rules"
)

func Validate(cfg core.Config) error {
	if err := validateNameAndProfile(cfg); err != nil {
		return err
	}
	if err := validateTargets(cfg.Targets); err != nil {
		return err
	}
	if err := validateOutput(cfg.Output); err != nil {
		return err
	}
	if err := validateWaivers(cfg.Waivers); err != nil {
		return err
	}
	if err := validateCommandChecks(cfg); err != nil {
		return err
	}
	if err := validateCoverageDelta(cfg.Checks.QualityRules.CoverageDelta); err != nil {
		return err
	}
	return validateRulePacks(cfg.RulePacks)
}

func validateNameAndProfile(cfg core.Config) error {
	if strings.TrimSpace(cfg.Name) == "" {
		return errors.New("config name is required")
	}

	profile := normalizeProfile(cfg.Profile)
	if profile == "" {
		return nil
	}
	if _, ok := profileCatalog[profile]; !ok {
		return fmt.Errorf("unknown profile %q", cfg.Profile)
	}
	return nil
}

func validateTargets(targets []core.TargetConfig) error {
	if len(targets) == 0 {
		return errors.New("at least one target is required")
	}
	for _, target := range targets {
		if strings.TrimSpace(target.Path) == "" {
			return errors.New("target path is required")
		}
	}
	return nil
}

func validateOutput(output core.OutputConfig) error {
	switch strings.TrimSpace(strings.ToLower(output.Format)) {
	case "", "text", "json", "sarif", "github":
		return nil
	default:
		return errors.New("output format must be one of text, json, sarif, github")
	}
}

func validateWaivers(waivers []core.WaiverConfig) error {
	for _, waiver := range waivers {
		if strings.TrimSpace(waiver.Rule) == "" {
			return errors.New("waiver rule is required")
		}
		if waiver.ExpiresOn == "" {
			continue
		}
		if _, err := time.Parse("2006-01-02", waiver.ExpiresOn); err != nil {
			return errors.New("waiver expires_on must use YYYY-MM-DD")
		}
	}
	return nil
}

func validateCommandChecks(cfg core.Config) error {
	if err := validateLanguageCommandMap("quality_rules.language_commands", cfg.Checks.QualityRules.LanguageCommands); err != nil {
		return err
	}
	if err := validateLanguageCommandMap("design_rules.language_commands", cfg.Checks.DesignRules.LanguageCommands); err != nil {
		return err
	}
	return validateLanguageCommandMap("security_rules.language_commands", cfg.Checks.SecurityRules.LanguageCommands)
}

func validateLanguageCommandMap(field string, languageCommands map[string][]core.CommandCheckConfig) error {
	for language, checks := range languageCommands {
		if strings.TrimSpace(language) == "" {
			return fmt.Errorf("%s contains an empty language key", field)
		}
		for idx, check := range checks {
			if strings.TrimSpace(check.Name) == "" {
				return fmt.Errorf("%s[%q][%d].name is required", field, language, idx)
			}
			if strings.TrimSpace(check.Command) == "" {
				return fmt.Errorf("%s[%q][%d].command is required", field, language, idx)
			}
		}
	}
	return nil
}

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
