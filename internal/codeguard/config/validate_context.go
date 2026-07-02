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
	return nil
}
