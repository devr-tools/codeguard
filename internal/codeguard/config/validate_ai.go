package config

import (
	"errors"
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
