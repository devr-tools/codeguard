package config

import (
	"fmt"
	"strings"

	"github.com/devr-tools/codeguard/internal/codeguard/core"
)

func validateCoverageDelta(cfg core.CoverageDeltaConfig) error {
	if err := validateCoverageThreshold("quality_rules.coverage_delta.min_changed_line_coverage", cfg.MinChangedLineCoverage); err != nil {
		return err
	}
	if err := validateCoverageThreshold("quality_rules.coverage_delta.fail_under", cfg.FailUnder); err != nil {
		return err
	}
	return validateCoverageCommands(cfg.LanguageCommands)
}

func validateCoverageThreshold(field string, value *int) error {
	if value == nil {
		return nil
	}
	if *value < 0 || *value > 100 {
		return fmt.Errorf("%s must be between 0 and 100", field)
	}
	return nil
}

func validateCoverageCommands(commands map[string]core.CoverageCommandConfig) error {
	for language, command := range commands {
		field := fmt.Sprintf("quality_rules.coverage_delta.language_commands[%q]", language)
		if strings.TrimSpace(language) == "" {
			return fmt.Errorf("quality_rules.coverage_delta.language_commands contains an empty language key")
		}
		if strings.TrimSpace(command.Command) == "" {
			return fmt.Errorf("%s.command is required", field)
		}
		if strings.TrimSpace(command.ReportPath) == "" {
			return fmt.Errorf("%s.report_path is required", field)
		}
		switch strings.TrimSpace(strings.ToLower(command.Format)) {
		case "", "lcov":
		default:
			return fmt.Errorf("%s.format must be lcov", field)
		}
	}
	return nil
}
