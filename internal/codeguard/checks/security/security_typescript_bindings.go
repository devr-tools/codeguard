package security

import (
	"path/filepath"
	"regexp"
	"strings"

	"github.com/devr-tools/codeguard/internal/codeguard/checks/support"
)

var (
	tsNamedImportPattern      = regexp.MustCompile(`(?m)^\s*import\s*{\s*([^}]+)\s*}\s*from\s*["'](?:node:)?%s["']`)
	tsNamespaceImportPattern  = regexp.MustCompile(`(?m)^\s*import\s+\*\s+as\s+([A-Za-z_$][\w$]*)\s*from\s*["'](?:node:)?%s["']`)
	tsDefaultImportPattern    = regexp.MustCompile(`(?m)^\s*import\s+([A-Za-z_$][\w$]*)\s*from\s*["'](?:node:)?%s["']`)
	tsNamedRequirePattern     = regexp.MustCompile(`(?m)^\s*(?:const|let|var)\s+{\s*([^}]+)\s*}\s*=\s*require\(\s*["'](?:node:)?%s["']\s*\)`)
	tsNamespaceRequirePattern = regexp.MustCompile(`(?m)^\s*(?:const|let|var)\s+([A-Za-z_$][\w$]*)\s*=\s*require\(\s*["'](?:node:)?%s["']\s*\)`)
)

func collectTypeScriptNamedModuleBindings(source string, module string, allowed []string) map[string]string {
	allowedSet := make(map[string]struct{}, len(allowed))
	for _, name := range allowed {
		allowedSet[name] = struct{}{}
	}
	aliases := make(map[string]string)
	for _, spec := range collectTypeScriptBindingSpecs(source, module, tsNamedImportPattern, tsNamedRequirePattern) {
		original, alias := parseTypeScriptBindingSpec(spec)
		if _, ok := allowedSet[original]; ok {
			aliases[alias] = original
		}
	}
	return aliases
}

func collectTypeScriptNamespaceBindings(source string, module string) map[string]struct{} {
	namespaces := make(map[string]struct{})
	for _, pattern := range []*regexp.Regexp{tsNamespaceImportPattern, tsDefaultImportPattern, tsNamespaceRequirePattern} {
		re := regexp.MustCompile(strings.ReplaceAll(pattern.String(), "%s", regexp.QuoteMeta(module)))
		for _, match := range re.FindAllStringSubmatch(source, -1) {
			if len(match) > 1 {
				namespaces[match[1]] = struct{}{}
			}
		}
	}
	return namespaces
}

func collectTypeScriptBindingSpecs(source string, module string, patterns ...*regexp.Regexp) []string {
	specs := make([]string, 0)
	for _, pattern := range patterns {
		re := regexp.MustCompile(strings.ReplaceAll(pattern.String(), "%s", regexp.QuoteMeta(module)))
		for _, match := range re.FindAllStringSubmatch(source, -1) {
			if len(match) > 1 {
				specs = append(specs, splitTypeScriptBindingSpecs(match[1])...)
			}
		}
	}
	return specs
}

func splitTypeScriptBindingSpecs(source string) []string {
	parts := strings.Split(source, ",")
	out := make([]string, 0, len(parts))
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part != "" {
			out = append(out, part)
		}
	}
	return out
}

func parseTypeScriptBindingSpec(spec string) (string, string) {
	if before, after, ok := strings.Cut(spec, " as "); ok {
		return strings.TrimSpace(before), strings.TrimSpace(after)
	}
	if before, after, ok := strings.Cut(spec, ":"); ok {
		return strings.TrimSpace(before), strings.TrimSpace(after)
	}
	spec = strings.TrimSpace(spec)
	return spec, spec
}

func typeScriptCallLinesWithShellOption(code string, alias string, namespaced bool) []int {
	lines := make([]int, 0)
	seen := make(map[int]struct{})
	patternText := `\b` + regexp.QuoteMeta(alias)
	if namespaced {
		patternText += `\s*\.\s*(?:spawn|spawnSync)\s*\(`
	} else {
		patternText += `\s*\(`
	}
	pattern := regexp.MustCompile(patternText)
	for _, match := range pattern.FindAllStringIndex(code, -1) {
		if !hasShellTrueNearOffset(code, match[0]) {
			continue
		}
		line := support.LineNumberForOffset(code, match[0])
		if _, exists := seen[line]; exists {
			continue
		}
		seen[line] = struct{}{}
		lines = append(lines, line)
	}
	return lines
}

func hasShellTrueNearOffset(code string, offset int) bool {
	limit := offset + 240
	if limit > len(code) {
		limit = len(code)
	}
	window := code[offset:limit]
	return strings.Contains(window, "shell") && regexp.MustCompile(`shell\s*:\s*true`).MatchString(window)
}

func isTypeScriptFile(path string) bool {
	switch strings.ToLower(filepath.Ext(path)) {
	case ".ts", ".tsx", ".js", ".jsx", ".mjs", ".cjs", ".mts", ".cts":
		return true
	default:
		return false
	}
}
