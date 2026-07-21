package config

import (
	"errors"
	"fmt"
	"path/filepath"
	"strings"
	"time"

	"github.com/devr-tools/codeguard/internal/codeguard/core"
)

func Validate(cfg core.Config) error {
	return firstError(
		validateNameAndProfile(cfg),
		validateTargets(cfg.Targets),
		validateOutput(cfg.Output),
		validateWaivers(cfg.Waivers),
		validateCommandChecks(cfg),
		validateAIConfig(cfg.AI),
		validateAIProvenance(cfg.Checks.QualityRules.AIProvenance),
		validateAIChangeRisk(cfg.Checks.QualityRules.AIChangeRisk),
		validateRiskScoring(cfg.Checks.QualityRules.RiskScoring),
		validateAIChecks(cfg.Checks.QualityRules.AIChecks),
		validateSupplyChainRules(cfg.Checks.SupplyChainRules),
		validateContractRules(cfg.Checks.ContractRules),
		validateContextRules(cfg.Checks.ContextRules),
		validateCoverageDelta(cfg.Checks.QualityRules.CoverageDelta),
		validateCPPTooling(cfg.Checks.QualityRules.CPPTooling),
		validateGraphThresholds(cfg.Checks.DesignRules),
		validateDesignArchitectureRules(cfg.Checks.DesignRules),
		validatePerformanceRules(cfg.Checks.PerformanceRules),
		validateSecretsRules(cfg.Checks.SecurityRules.Secrets),
		validateParsers(cfg.Parsers),
		validateRulePacks(cfg.RulePacks),
		validateExternalReports(cfg.ExternalReports),
	)
}

func validateExternalReports(reports []core.ExternalReportConfig) error {
	for i, report := range reports {
		field := fmt.Sprintf("external_reports[%d]", i)
		path := strings.TrimSpace(report.Path)
		if path == "" {
			return fmt.Errorf("%s.path is required", field)
		}
		clean := filepath.Clean(filepath.FromSlash(path))
		if !filepath.IsAbs(clean) && (clean == ".." || strings.HasPrefix(clean, ".."+string(filepath.Separator))) {
			return fmt.Errorf("%s.path must not escape the repository", field)
		}
		switch strings.ToLower(strings.TrimSpace(report.Format)) {
		case "sarif", "gitleaks", "trivy":
		default:
			return fmt.Errorf("%s.format must be sarif, gitleaks, or trivy", field)
		}
	}
	return nil
}

func validateCPPTooling(cfg core.CPPToolingConfig) error {
	for _, setting := range []struct{ field, mode string }{
		{"quality_rules.cpp_tooling.clang_format_mode", cfg.ClangFormatMode},
		{"quality_rules.cpp_tooling.compiler_mode", cfg.CompilerMode},
	} {
		switch strings.ToLower(strings.TrimSpace(setting.mode)) {
		case "", core.ExternalToolModeOff, core.ExternalToolModeAuto, core.ExternalToolModeRequired:
		default:
			return fmt.Errorf("%s must be off, auto, or required", setting.field)
		}
	}
	compileCommands := filepath.Clean(filepath.FromSlash(strings.TrimSpace(cfg.CompileCommands)))
	if filepath.IsAbs(compileCommands) || compileCommands == ".." || strings.HasPrefix(compileCommands, ".."+string(filepath.Separator)) {
		return errors.New("quality_rules.cpp_tooling.compile_commands must be relative to the target")
	}
	return nil
}

func validateParsers(parsers core.ParsersConfig) error {
	switch strings.TrimSpace(strings.ToLower(parsers.TreeSitter)) {
	case "", core.TreeSitterModeOff, core.TreeSitterModeAuto:
		return nil
	default:
		return fmt.Errorf("parsers.treesitter must be %q or %q", core.TreeSitterModeOff, core.TreeSitterModeAuto)
	}
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
	case "", "text", "json", "sarif", "github", "cyclonedx", "cyclonedx-json":
		return nil
	default:
		return errors.New("output format must be one of text, json, sarif, github, cyclonedx")
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
	if err := validateLanguageCommandMap("design_rules.language_diff_commands", cfg.Checks.DesignRules.LanguageDiffCommands); err != nil {
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
