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

// defaultInt fills an int setting with its profile default when unset.
func defaultInt(dst *int, def int) {
	if *dst == 0 {
		*dst = def
	}
}

// defaultBoolPtr fills an optional bool setting with the given default when unset.
func defaultBoolPtr(dst **bool, value bool) {
	if *dst == nil {
		*dst = boolPtr(value)
	}
}

func valueOrDefault(ptr *bool, def bool) bool {
	if ptr == nil {
		return def
	}
	return *ptr
}

// defaultStringSlice fills a string-slice setting with a copy of its default
// when unset. requireNonEmpty skips defaults that are empty.
func defaultStringSlice(dst *[]string, def []string, requireNonEmpty bool) {
	if *dst != nil || (requireNonEmpty && len(def) == 0) {
		return
	}
	*dst = append([]string(nil), def...)
}

// defaultCommandMap fills a per-language command map with a cloned default
// when unset.
func defaultCommandMap(dst *map[string][]core.CommandCheckConfig, def map[string][]core.CommandCheckConfig) {
	if *dst == nil && len(def) > 0 {
		*dst = cloneCommandCheckMap(def)
	}
}

func defaultSingleCommandMap(dst *map[string]core.CommandCheckConfig, def map[string]core.CommandCheckConfig) {
	if *dst == nil && len(def) > 0 {
		cloned := make(map[string]core.CommandCheckConfig, len(def))
		for key, value := range def {
			cloned[key] = value
		}
		*dst = cloned
	}
}
