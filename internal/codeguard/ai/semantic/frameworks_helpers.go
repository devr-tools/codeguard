package semantic

import (
	"sort"
	"strings"
)

func containsAny(content string, needles ...string) bool {
	for _, needle := range needles {
		if strings.Contains(content, needle) {
			return true
		}
	}
	return false
}

func hasAnySuffix(value string, suffixes ...string) bool {
	for _, suffix := range suffixes {
		if strings.HasSuffix(value, suffix) {
			return true
		}
	}
	return false
}

func containsComponentExport(content string) bool {
	lines := strings.Split(content, "\n")
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		switch {
		case strings.HasPrefix(trimmed, "export default function ") && hasPascalCaseName(trimmed, "export default function "):
			return true
		case strings.HasPrefix(trimmed, "export function ") && hasPascalCaseName(trimmed, "export function "):
			return true
		case strings.HasPrefix(trimmed, "const ") && strings.Contains(trimmed, " = (") && strings.Contains(trimmed, "=>"):
			name := strings.TrimSpace(strings.TrimPrefix(strings.SplitN(trimmed, "=", 2)[0], "const "))
			if startsUpper(name) {
				return true
			}
		}
	}
	return false
}

func hasPascalCaseName(line string, prefix string) bool {
	name := strings.TrimSpace(strings.TrimPrefix(line, prefix))
	if name == "" {
		return false
	}
	for i, r := range name {
		if r == '(' || r == ' ' || r == '<' {
			return i > 0 && startsUpper(name[:i])
		}
	}
	return startsUpper(name)
}

func startsUpper(value string) bool {
	if value == "" {
		return false
	}
	r := rune(value[0])
	return r >= 'A' && r <= 'Z'
}

func uniqueSortedStrings(values []string) []string {
	if len(values) == 0 {
		return nil
	}
	seen := make(map[string]struct{}, len(values))
	out := make([]string, 0, len(values))
	for _, value := range values {
		if value == "" {
			continue
		}
		if _, ok := seen[value]; ok {
			continue
		}
		seen[value] = struct{}{}
		out = append(out, value)
	}
	sort.Strings(out)
	return out
}
