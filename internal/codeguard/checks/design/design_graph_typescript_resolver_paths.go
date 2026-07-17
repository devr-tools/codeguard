package design

import (
	"path"
	"sort"
	"strings"
)

func typeScriptPathContains(parent string, child string) bool {
	parent = normalizeTypeScriptPath(parent)
	child = normalizeTypeScriptPath(child)
	if parent == "." {
		return true
	}
	return child == parent || strings.HasPrefix(child, parent+"/")
}

func normalizeTypeScriptPath(value string) string {
	value = strings.TrimSpace(strings.ReplaceAll(value, "\\", "/"))
	if value == "" {
		return "."
	}
	value = strings.TrimPrefix(path.Clean(value), "./")
	if value == "" {
		return "."
	}
	return value
}

func typeScriptPackageRoot(specifier string) string {
	if strings.HasPrefix(specifier, "@") {
		parts := strings.Split(specifier, "/")
		if len(parts) >= 2 {
			return parts[0] + "/" + parts[1]
		}
	}
	return firstTypeScriptSegment(specifier)
}

func firstTypeScriptSegment(specifier string) string {
	specifier = strings.TrimSpace(specifier)
	if specifier == "" {
		return ""
	}
	if cut := strings.IndexByte(specifier, '/'); cut >= 0 {
		return specifier[:cut]
	}
	return specifier
}

func matchTypeScriptAlias(pattern string, specifier string) (string, bool) {
	if !strings.Contains(pattern, "*") {
		return "", pattern == specifier
	}
	parts := strings.SplitN(pattern, "*", 2)
	if !strings.HasPrefix(specifier, parts[0]) || !strings.HasSuffix(specifier, parts[1]) {
		return "", false
	}
	return specifier[len(parts[0]) : len(specifier)-len(parts[1])], true
}

func applyTypeScriptAliasTarget(target string, wildcard string) string {
	if !strings.Contains(target, "*") {
		return target
	}
	return strings.Replace(target, "*", wildcard, 1)
}

func matchTypeScriptMapping(mappings map[string][]string, specifier string) []string {
	if len(mappings) == 0 {
		return nil
	}
	patterns := make([]string, 0, len(mappings))
	for pattern := range mappings {
		patterns = append(patterns, pattern)
	}
	sort.Slice(patterns, func(i, j int) bool {
		if len(patterns[i]) != len(patterns[j]) {
			return len(patterns[i]) > len(patterns[j])
		}
		return patterns[i] < patterns[j]
	})
	for _, pattern := range patterns {
		wildcard, ok := matchTypeScriptAlias(pattern, specifier)
		if !ok {
			continue
		}
		values := make([]string, 0, len(mappings[pattern]))
		for _, target := range mappings[pattern] {
			values = append(values, applyTypeScriptAliasTarget(target, wildcard))
		}
		return values
	}
	return nil
}
