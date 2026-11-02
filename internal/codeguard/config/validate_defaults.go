package config

import (
	"fmt"
	"strings"

	"github.com/devr-tools/codeguard/internal/codeguard/core"
	rulespkg "github.com/devr-tools/codeguard/internal/codeguard/rules"
	"github.com/devr-tools/codeguard/internal/codeguard/suggest"
)

var recognizedDisabledChecks = map[string]struct{}{
	"quality": {}, "performance": {}, "design": {}, "security": {}, "prompts": {},
	"ci": {}, "supply_chain": {}, "delivery": {}, "reliability": {}, "data": {}, "change": {}, "context": {}, "contracts": {},
}

func validateDisabledChecks(disabled []string) error {
	seen := make(map[string]struct{}, len(disabled))
	for idx, section := range disabled {
		if strings.TrimSpace(section) == "" {
			return fmt.Errorf("checks.disabled[%d] must not be blank", idx)
		}
		if _, exists := recognizedDisabledChecks[section]; !exists {
			known := sortedKeys(recognizedDisabledChecks)
			return fmt.Errorf("checks.disabled contains unknown section %q%s (valid sections: %s)",
				section, suggest.Hint(section, known), strings.Join(known, ", "))
		}
		if _, exists := seen[section]; exists {
			return fmt.Errorf("checks.disabled contains duplicate section %q", section)
		}
		seen[section] = struct{}{}
	}
	return nil
}

// validateDisabledRules checks checks.disabled_rules against the rule catalog
// and the config's rule packs. An entry is an exact rule id (a retired alias
// is accepted) or a "family.*" prefix that matches at least one rule.
func validateDisabledRules(cfg core.Config) error {
	catalog := RuleCatalogForConfig(cfg)
	ids := make([]string, 0, len(catalog))
	for id := range catalog {
		ids = append(ids, id)
	}
	retired := rulespkg.RetiredPerformanceRuleIDs()
	seen := make(map[string]struct{}, len(cfg.Checks.DisabledRules))
	for idx, raw := range cfg.Checks.DisabledRules {
		entry := strings.TrimSpace(raw)
		if entry == "" {
			return fmt.Errorf("checks.disabled_rules[%d] must not be blank", idx)
		}
		if _, dup := seen[entry]; dup {
			return fmt.Errorf("checks.disabled_rules contains duplicate entry %q", entry)
		}
		seen[entry] = struct{}{}
		if prefix, ok := strings.CutSuffix(entry, rulespkg.DisabledRuleWildcardSuffix); ok {
			if !anyRuleHasPrefix(ids, prefix+".") {
				return fmt.Errorf("checks.disabled_rules entry %q matches no rule%s", entry, suggest.Hint(prefix, ids))
			}
			continue
		}
		if _, ok := catalog[entry]; ok {
			continue
		}
		if _, ok := catalog[rulespkg.ResolveRuleAlias(entry)]; ok {
			continue
		}
		if renamed, ok := retired[entry]; ok {
			return fmt.Errorf("checks.disabled_rules contains retired rule %q; it is now %q", entry, renamed)
		}
		return fmt.Errorf("checks.disabled_rules contains unknown rule %q%s (run codeguard rules to list rule ids)", entry, suggest.Hint(entry, ids))
	}
	return nil
}

func anyRuleHasPrefix(ids []string, prefix string) bool {
	if prefix == "." {
		return false
	}
	for _, id := range ids {
		if strings.HasPrefix(id, prefix) {
			return true
		}
	}
	return false
}

func validateBasicThresholds(checks core.CheckConfig) error {
	for _, threshold := range basicThresholds(checks) {
		if threshold.value < 0 {
			return fmt.Errorf("%s must not be negative, got %d", threshold.field, threshold.value)
		}
	}
	return nil
}

func basicThresholds(checks core.CheckConfig) []struct {
	field string
	value int
} {
	return []struct {
		field string
		value int
	}{
		{"quality_rules.max_file_lines", checks.QualityRules.MaxFileLines},
		{"quality_rules.max_function_lines", checks.QualityRules.MaxFunctionLines},
		{"quality_rules.max_parameters", checks.QualityRules.MaxParameters},
		{"quality_rules.max_cyclomatic_complexity", checks.QualityRules.MaxCyclomaticComplexity},
		{"quality_rules.clone_token_threshold", checks.QualityRules.CloneTokenThreshold},
		{"design_rules.max_decls_per_file", checks.DesignRules.MaxDeclsPerFile},
		{"design_rules.max_methods_per_type", checks.DesignRules.MaxMethodsPerType},
		{"design_rules.max_interface_methods", checks.DesignRules.MaxInterfaceMethods},
	}
}
