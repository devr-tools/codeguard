package config

import (
	"fmt"
	"strings"

	"github.com/devr-tools/codeguard/internal/codeguard/core"
)

var recognizedDisabledChecks = map[string]struct{}{
	"quality": {}, "performance": {}, "design": {}, "security": {}, "prompts": {},
	"ci": {}, "supply_chain": {}, "context": {}, "contracts": {},
}

func validateDisabledChecks(disabled []string) error {
	seen := make(map[string]struct{}, len(disabled))
	for idx, section := range disabled {
		if strings.TrimSpace(section) == "" {
			return fmt.Errorf("checks.disabled[%d] must not be blank", idx)
		}
		if _, exists := recognizedDisabledChecks[section]; !exists {
			return fmt.Errorf("checks.disabled contains unknown section %q", section)
		}
		if _, exists := seen[section]; exists {
			return fmt.Errorf("checks.disabled contains duplicate section %q", section)
		}
		seen[section] = struct{}{}
	}
	return nil
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
