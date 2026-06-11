package config

import "github.com/devr-tools/codeguard/internal/codeguard/core"

func cloneCommandCheckMap(src map[string][]core.CommandCheckConfig) map[string][]core.CommandCheckConfig {
	if len(src) == 0 {
		return nil
	}
	dst := make(map[string][]core.CommandCheckConfig, len(src))
	for language, checks := range src {
		dst[language] = append([]core.CommandCheckConfig(nil), checks...)
	}
	return dst
}

func applyDefaultBoolPtrs(values ...**bool) {
	for _, value := range values {
		if *value == nil {
			*value = boolPtr(true)
		}
	}
}
