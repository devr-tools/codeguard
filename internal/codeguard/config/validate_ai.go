package config

import (
	"errors"
	"fmt"
	"strings"

	"github.com/devr-tools/codeguard/internal/codeguard/core"
)

func validateAIProvenance(cfg core.AIProvenanceConfig) error {
	if cfg.SlopScoreWarnThreshold < 0 {
		return errors.New("quality_rules.ai_provenance.slop_score_warn_threshold must be non-negative")
	}
	if cfg.SlopScoreFailThreshold < 0 {
		return errors.New("quality_rules.ai_provenance.slop_score_fail_threshold must be non-negative")
	}
	if cfg.SlopScoreFailThreshold > 0 && cfg.SlopScoreWarnThreshold > 0 && cfg.SlopScoreFailThreshold < cfg.SlopScoreWarnThreshold {
		return errors.New("quality_rules.ai_provenance.slop_score_fail_threshold must be greater than or equal to slop_score_warn_threshold")
	}
	for _, key := range cfg.EnvVars {
		if strings.TrimSpace(key) == "" {
			return errors.New("quality_rules.ai_provenance.env_vars must not contain empty values")
		}
	}
	for _, trailer := range cfg.CommitTrailers {
		if strings.TrimSpace(trailer) == "" {
			return errors.New("quality_rules.ai_provenance.commit_trailers must not contain empty values")
		}
	}
	return nil
}

func validateAIChangeRisk(cfg core.AIChangeRiskConfig) error {
	if cfg.WarnThreshold < 0 {
		return errors.New("quality_rules.ai_change_risk.warn_threshold must be non-negative")
	}
	if cfg.FailThreshold < 0 {
		return errors.New("quality_rules.ai_change_risk.fail_threshold must be non-negative")
	}
	if cfg.FailThreshold > 0 && cfg.WarnThreshold > 0 && cfg.FailThreshold < cfg.WarnThreshold {
		return errors.New("quality_rules.ai_change_risk.fail_threshold must be greater than or equal to warn_threshold")
	}
	return nil
}

func validateAIChecks(cfg core.AIChecksConfig) error {
	if cfg.SlopHistoryLimit < 0 {
		return errors.New("quality_rules.ai_checks.slop_history_limit must be non-negative")
	}
	return nil
}

func validateAIConfig(cfg core.AIConfig) error {
	if err := validateAIProvider(cfg.Provider); err != nil {
		return err
	}
	if cfg.AutoFix.MaxFixes < 0 {
		return errors.New("ai.autofix.max_fixes must be non-negative")
	}
	for idx, check := range cfg.AutoFix.TestCommands {
		if strings.TrimSpace(check.Name) == "" {
			return fmt.Errorf("ai.autofix.test_commands[%d].name is required", idx)
		}
		if strings.TrimSpace(check.Command) == "" {
			return fmt.Errorf("ai.autofix.test_commands[%d].command is required", idx)
		}
	}
	return nil
}

func validateAIProvider(cfg core.AIProviderConfig) error {
	providerType := strings.TrimSpace(strings.ToLower(cfg.Type))
	switch providerType {
	case "", "openai", "command":
	default:
		return fmt.Errorf("ai.provider.type must be openai or command")
	}
	if providerType == "command" && strings.TrimSpace(cfg.Command) == "" {
		return errors.New("ai.provider.command is required when ai.provider.type=command")
	}
	return nil
}
