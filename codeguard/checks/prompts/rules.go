package prompts

import (
	"strings"

	"github.com/devr-tools/codeguard/codeguard/core"
)

type promptRules struct {
	fileExtensions            []string
	pathContains              []string
	forbidSecretInterpolation bool
	forbidUnsafeInstructions  bool
}

func resolvePromptRules(cfg core.PromptRulesConfig) promptRules {
	rules := promptRules{
		fileExtensions:            []string{".prompt", ".md", ".txt", ".tmpl", ".yaml", ".yml", ".json"},
		pathContains:              []string{"prompt", "system", "instruction", "template"},
		forbidSecretInterpolation: true,
		forbidUnsafeInstructions:  true,
	}
	if len(cfg.FileExtensions) > 0 {
		rules.fileExtensions = normalizeList(cfg.FileExtensions)
	}
	if len(cfg.PathContains) > 0 {
		rules.pathContains = normalizeList(cfg.PathContains)
	}
	rules.forbidSecretInterpolation = boolValue(cfg.ForbidSecretInterpolation, rules.forbidSecretInterpolation)
	rules.forbidUnsafeInstructions = boolValue(cfg.ForbidUnsafeInstructions, rules.forbidUnsafeInstructions)
	return rules
}

func normalizeList(values []string) []string {
	out := make([]string, 0, len(values))
	for _, value := range values {
		trimmed := strings.ToLower(strings.TrimSpace(value))
		if trimmed != "" {
			out = append(out, trimmed)
		}
	}
	return out
}

func boolValue(value *bool, fallback bool) bool {
	if value == nil {
		return fallback
	}
	return *value
}
