package config

import (
	"fmt"
	"strings"

	"github.com/devr-tools/codeguard/codeguard/core"
)

func Validate(cfg core.Config) error {
	if strings.TrimSpace(cfg.Name) == "" {
		return fmt.Errorf("name is required")
	}
	if err := validateTargets(cfg.Targets); err != nil {
		return err
	}
	return runValidators(
		func() error { return validateOutput(cfg.Output) },
		func() error { return validateQualityRules(cfg.Checks.QualityRules) },
		func() error { return validateDesignRules(cfg.Checks.DesignRules) },
		func() error { return validatePromptRules(cfg.Checks.PromptRules) },
		func() error { return validateCIRules(cfg.Checks.CIRules) },
		func() error { return validateSecurityRules(cfg.Checks.SecurityRules) },
	)
}

func validateTargets(targets []core.TargetConfig) error {
	if len(targets) == 0 {
		return fmt.Errorf("at least one target is required")
	}

	for i, target := range targets {
		if strings.TrimSpace(target.Path) == "" {
			return fmt.Errorf("targets[%d].path is required", i)
		}
		if strings.TrimSpace(target.Language) == "" {
			return fmt.Errorf("targets[%d].language is required", i)
		}
	}
	return nil
}

func runValidators(validators ...func() error) error {
	for _, validate := range validators {
		if err := validate(); err != nil {
			return err
		}
	}
	return nil
}

func validateOutput(output core.OutputConfig) error {
	format := strings.TrimSpace(output.Format)
	if format == "" {
		return fmt.Errorf("output.format is required")
	}
	if format != "text" && format != "json" {
		return fmt.Errorf("output.format must be text or json")
	}
	return nil
}

func validateQualityRules(rules core.QualityRulesConfig) error {
	if rules.MaxFileLines < 0 {
		return fmt.Errorf("checks.quality_rules.max_file_lines must be >= 0")
	}
	if rules.MaxFunctionLines < 0 {
		return fmt.Errorf("checks.quality_rules.max_function_lines must be >= 0")
	}
	if rules.MaxParameters < 0 {
		return fmt.Errorf("checks.quality_rules.max_parameters must be >= 0")
	}
	if rules.MaxCyclomaticComplexity < 0 {
		return fmt.Errorf("checks.quality_rules.max_cyclomatic_complexity must be >= 0")
	}
	return nil
}

func validateDesignRules(rules core.DesignRulesConfig) error {
	if rules.MaxDeclsPerFile < 0 {
		return fmt.Errorf("checks.design_rules.max_decls_per_file must be >= 0")
	}
	if rules.MaxMethodsPerType < 0 {
		return fmt.Errorf("checks.design_rules.max_methods_per_type must be >= 0")
	}
	if rules.MaxInterfaceMethods < 0 {
		return fmt.Errorf("checks.design_rules.max_interface_methods must be >= 0")
	}
	return validateTrimmedValues("checks.design_rules.forbidden_package_names", rules.ForbiddenPackageNames)
}

func validatePromptRules(rules core.PromptRulesConfig) error {
	if err := validateTrimmedValues("checks.prompt_rules.file_extensions", rules.FileExtensions); err != nil {
		return err
	}
	return validateTrimmedValues("checks.prompt_rules.path_contains", rules.PathContains)
}

func validateCIRules(rules core.CIRulesConfig) error {
	if err := validateTrimmedValues("checks.ci_rules.required_workflow_files", rules.RequiredWorkflowFiles); err != nil {
		return err
	}
	for i, rule := range rules.WorkflowContentRules {
		if strings.TrimSpace(rule.Path) == "" {
			return fmt.Errorf("checks.ci_rules.workflow_content_rules[%d].path must not be empty", i)
		}
		if err := validateTrimmedValues(
			fmt.Sprintf("checks.ci_rules.workflow_content_rules[%d].required_contains", i),
			rule.RequiredContains,
		); err != nil {
			return err
		}
	}
	if err := validateTrimmedValues("checks.ci_rules.required_release_files", rules.RequiredReleaseFiles); err != nil {
		return err
	}
	return validateTrimmedValues("checks.ci_rules.required_automation_paths", rules.RequiredAutomationPaths)
}

func validateSecurityRules(rules core.SecurityRulesConfig) error {
	mode := strings.TrimSpace(rules.GovulncheckMode)
	if mode != "" && mode != "off" && mode != "auto" && mode != "required" {
		return fmt.Errorf("checks.security_rules.govulncheck_mode must be off, auto, or required")
	}
	return nil
}

func validateTrimmedValues(field string, values []string) error {
	for i, value := range values {
		if strings.TrimSpace(value) == "" {
			return fmt.Errorf("%s[%d] must not be empty", field, i)
		}
	}
	return nil
}
