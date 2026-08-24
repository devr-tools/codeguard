package config

import (
	"fmt"
	"path/filepath"
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

func validateQualityDeadCode(cfg core.QualityDeadCodeConfig) error {
	switch strings.TrimSpace(strings.ToLower(cfg.Mode)) {
	case "", "off", "toolchain":
	default:
		return fmt.Errorf("quality_rules.dead_code.mode must be off or toolchain, got %q", cfg.Mode)
	}
	switch strings.TrimSpace(strings.ToLower(cfg.Level)) {
	case "", "warn", "fail":
	default:
		return fmt.Errorf("quality_rules.dead_code.level must be warn or fail, got %q", cfg.Level)
	}
	return firstError(
		validateGoDeadCode("quality_rules.dead_code.go", cfg.Go),
		validateRustDeadCode("quality_rules.dead_code.rust", cfg.Rust),
		validateCPPDeadCode("quality_rules.dead_code.cpp", cfg.CPP),
		validatePythonDeadCode("quality_rules.dead_code.python", cfg.Python),
		validateScriptDeadCode("quality_rules.dead_code.typescript", cfg.TypeScript),
		validateScriptDeadCode("quality_rules.dead_code.javascript", cfg.JavaScript),
	)
}

func validateGoDeadCode(field string, cfg core.GoDeadCodeToolchainConfig) error {
	return firstError(
		validateRelativePatternList(field+".packages", cfg.Packages),
		validateRelativePatternList(field+".entrypoints", cfg.Entrypoints),
		validateRelativePatternList(field+".ignore_paths", cfg.IgnorePaths),
	)
}

func validateRustDeadCode(field string, cfg core.RustDeadCodeConfig) error {
	return firstError(
		validateRelativePatternList(field+".crates", cfg.Crates),
		validateNameList(field+".packages", cfg.Packages),
		validateRelativePatternList(field+".entrypoints", cfg.Entrypoints),
		validateRelativePatternList(field+".reports", cfg.Reports),
		validateRelativePatternList(field+".ignore_paths", cfg.IgnorePaths),
	)
}

func validateCPPDeadCode(field string, cfg core.CPPDeadCodeConfig) error {
	return firstError(
		validateOptionalRelativePattern(field+".compile_commands", cfg.CompileCommands),
		validateRelativePatternList(field+".entrypoints", cfg.Entrypoints),
		validateRelativePatternList(field+".reports", cfg.Reports),
		validateRelativePatternList(field+".ignore_paths", cfg.IgnorePaths),
	)
}

func validatePythonDeadCode(field string, cfg core.PythonDeadCodeConfig) error {
	return firstError(
		validateRelativePatternList(field+".modules", cfg.Modules),
		validateRelativePatternList(field+".entrypoints", cfg.Entrypoints),
		validateRelativePatternList(field+".reports", cfg.Reports),
		validateRelativePatternList(field+".ignore_paths", cfg.IgnorePaths),
	)
}

func validateRelativePatternList(field string, patterns []string) error {
	for idx, pattern := range patterns {
		if err := validateRelativePattern(fmt.Sprintf("%s[%d]", field, idx), pattern); err != nil {
			return err
		}
	}
	return nil
}

func validateNameList(field string, names []string) error {
	for idx, name := range names {
		trimmed := strings.TrimSpace(name)
		if trimmed == "" {
			return fmt.Errorf("%s[%d] must not be blank", field, idx)
		}
		if strings.HasPrefix(trimmed, "-") {
			return fmt.Errorf("%s[%d] must be a name, not a flag", field, idx)
		}
	}
	return nil
}

func validateScriptDeadCode(field string, cfg core.ScriptDeadCodeConfig) error {
	if err := validateRelativePatternList(field+".projects", cfg.Projects); err != nil {
		return err
	}
	if err := validateRelativePatternList(field+".entrypoints", cfg.Entrypoints); err != nil {
		return err
	}
	if err := validateRelativePatternList(field+".reports", cfg.Reports); err != nil {
		return err
	}
	return validateRelativePatternList(field+".ignore_paths", cfg.IgnorePaths)
}

func validateRelativePattern(field string, pattern string) error {
	trimmed := strings.TrimSpace(pattern)
	if trimmed == "" {
		return fmt.Errorf("%s must not be blank", field)
	}
	clean := filepath.Clean(filepath.FromSlash(trimmed))
	if filepath.IsAbs(clean) || clean == ".." || strings.HasPrefix(clean, ".."+string(filepath.Separator)) {
		return fmt.Errorf("%s must be relative to the target", field)
	}
	if strings.HasPrefix(trimmed, "-") {
		return fmt.Errorf("%s must be a path or glob, not a flag", field)
	}
	return nil
}

func validateOptionalRelativePattern(field string, pattern string) error {
	if strings.TrimSpace(pattern) == "" {
		return nil
	}
	return validateRelativePattern(field, pattern)
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

// firstError returns the first non-nil error, mirroring sequential validation
// while keeping each rule group independently testable.
func firstError(errs ...error) error {
	for _, err := range errs {
		if err != nil {
			return err
		}
	}
	return nil
}
