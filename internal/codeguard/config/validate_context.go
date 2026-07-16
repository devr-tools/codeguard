package config

import (
	"errors"

	"github.com/devr-tools/codeguard/internal/codeguard/core"
)

func validateContextRules(cfg core.ContextRulesConfig) error {
	if cfg.MaxFileLines < 0 {
		return errors.New("context_rules.max_file_lines must be positive")
	}
	if cfg.AmbiguousSymbolThreshold < 0 || cfg.AmbiguousSymbolThreshold == 1 {
		return errors.New("context_rules.ambiguous_symbol_threshold must be at least 2")
	}
	if cfg.MaxAgentDocLines < 0 {
		return errors.New("context_rules.max_agent_doc_lines must be positive")
	}
	if cfg.LegibilityWarnThreshold < 0 || cfg.LegibilityWarnThreshold > 100 {
		return errors.New("context_rules.legibility_warn_threshold must be between 0 and 100")
	}
	if cfg.LegibilityFailThreshold < 0 || cfg.LegibilityFailThreshold > 100 {
		return errors.New("context_rules.legibility_fail_threshold must be between 0 and 100")
	}
	// Legibility is good-high: the finding fires when the score drops below a
	// threshold, so the fail bar must sit at or below the warn bar (the
	// inverse of the slop-score threshold ordering).
	if cfg.LegibilityFailThreshold > 0 && cfg.LegibilityWarnThreshold > 0 && cfg.LegibilityFailThreshold > cfg.LegibilityWarnThreshold {
		return errors.New("context_rules.legibility_fail_threshold must be less than or equal to legibility_warn_threshold")
	}
	if cfg.LegibilityHistoryLimit < 0 {
		return errors.New("context_rules.legibility_history_limit must be non-negative")
	}
	return nil
}
