package design

import (
	"encoding/json"
	"sort"
	"strings"
)

func parseTypeScriptPackageExports(raw json.RawMessage) map[string][]string {
	return parseTypeScriptPackageMappings(raw, true)
}

func parseTypeScriptPackageImports(raw json.RawMessage) map[string][]string {
	return parseTypeScriptPackageMappings(raw, false)
}

func parseTypeScriptPackageMappings(raw json.RawMessage, isExports bool) map[string][]string {
	if len(raw) == 0 {
		return nil
	}
	var node any
	if err := json.Unmarshal(raw, &node); err != nil {
		return nil
	}
	mappings := make(map[string][]string)
	switch value := node.(type) {
	case string:
		mappings["."] = append(mappings["."], value)
	case map[string]any:
		for key, child := range value {
			appendTypeScriptPackageMapping(mappings, key, child, isExports)
		}
	}
	for key, values := range mappings {
		mappings[key] = uniqueNonEmptyStrings(values)
		if len(mappings[key]) == 0 {
			delete(mappings, key)
		}
	}
	return mappings
}

func appendTypeScriptPackageMapping(mappings map[string][]string, key string, child any, isExports bool) {
	mappingKey := typeScriptPackageMappingKey(key, isExports)
	mappings[mappingKey] = append(mappings[mappingKey], collectTypeScriptExportTargets(child)...)
}

func typeScriptPackageMappingKey(key string, isExports bool) string {
	switch {
	case isExports && (key == "." || strings.HasPrefix(key, "./")):
		return key
	case !isExports && strings.HasPrefix(key, "#"):
		return key
	default:
		return "."
	}
}

func collectTypeScriptExportTargets(node any) []string {
	switch value := node.(type) {
	case string:
		return []string{value}
	case []any:
		out := make([]string, 0, len(value))
		for _, item := range value {
			out = append(out, collectTypeScriptExportTargets(item)...)
		}
		return out
	case map[string]any:
		out := make([]string, 0, len(value))
		for _, key := range orderedTypeScriptConditionKeys(value) {
			out = append(out, collectTypeScriptExportTargets(value[key])...)
		}
		return out
	default:
		return nil
	}
}

func orderedTypeScriptConditionKeys(values map[string]any) []string {
	preferred := []string{
		"types", "source", "import", "module", "browser", "development",
		"production", "node", "default", "require",
	}
	keys := make([]string, 0, len(values))
	seen := make(map[string]struct{}, len(values))
	for _, key := range preferred {
		if _, ok := values[key]; ok {
			keys = append(keys, key)
			seen[key] = struct{}{}
		}
	}
	extra := make([]string, 0, len(values))
	for key := range values {
		if _, ok := seen[key]; ok {
			continue
		}
		extra = append(extra, key)
	}
	sort.Strings(extra)
	return append(keys, extra...)
}

func uniqueNonEmptyStrings(values []string) []string {
	out := make([]string, 0, len(values))
	seen := make(map[string]struct{}, len(values))
	for _, value := range values {
		value = strings.TrimSpace(strings.TrimPrefix(value, "./"))
		if value == "" {
			continue
		}
		if _, ok := seen[value]; ok {
			continue
		}
		seen[value] = struct{}{}
		out = append(out, value)
	}
	return out
}
