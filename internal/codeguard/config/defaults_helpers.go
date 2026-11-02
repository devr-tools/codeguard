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

// defaultInt fills an int setting with its profile default when unset. A plain
// int cannot tell an explicit 0 from an omitted key, so 0 counts as unset
// here. Settings where 0 can be a real choice track explicit keys and use
// defaultUnlessExplicit; the list is documented in docs/config-reference.md.
func defaultInt(dst *int, def int) {
	if *dst == 0 {
		*dst = def
	}
}

// defaultUnlessExplicit fills an int setting with its default when it is 0
// and was not marked explicit (by a config file key, a -set override, or an
// SDK SetExplicit call). Unlike defaultInt it keeps an explicit 0, so use it
// for settings where 0 is a meaningful choice (a weight that should
// contribute nothing).
func defaultUnlessExplicit(dst *int, explicit bool, def int) {
	if *dst == 0 && !explicit {
		*dst = def
	}
}

// defaultBoolPtr fills an optional bool setting with the given default when unset.
func defaultBoolPtr(dst **bool, value bool) {
	if *dst == nil {
		*dst = boolPtr(value)
	}
}

// boolValueOrTrue reads an optional bool, treating an unset (nil) pointer as true.
func boolValueOrTrue(ptr *bool) bool {
	if ptr == nil {
		return true
	}
	return *ptr
}

// boolValue reads an optional bool, treating an unset (nil) pointer as false.
// Use it when reading a baseline value whose "not stated" answer is off.
func boolValue(ptr *bool) bool {
	return ptr != nil && *ptr
}

// defaultStringSlice fills a string-slice setting with a copy of its default
// when unset.
func defaultStringSlice(dst *[]string, def []string) {
	if *dst != nil {
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
