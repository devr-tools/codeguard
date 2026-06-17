package support

import (
	"regexp"
	"strings"
)

func packageManagerName(raw string) string {
	raw = strings.TrimSpace(raw)
	switch {
	case strings.HasPrefix(raw, "pnpm@"):
		return "pnpm"
	case strings.HasPrefix(raw, "yarn@"):
		return "yarn"
	case strings.HasPrefix(raw, "bun@"):
		return "bun"
	case strings.HasPrefix(raw, "npm@"):
		return "npm"
	default:
		return ""
	}
}

func parseJSONLicense(value any) string {
	switch typed := value.(type) {
	case string:
		return strings.TrimSpace(typed)
	case map[string]any:
		for _, key := range []string{"type", "text", "name"} {
			if raw, ok := typed[key]; ok {
				if text, ok := raw.(string); ok {
					return strings.TrimSpace(text)
				}
			}
		}
	}
	return ""
}

func parseTOMLLicenseValue(line string) string {
	if idx := strings.Index(line, "="); idx >= 0 {
		value := strings.TrimSpace(line[idx+1:])
		if match := quotedStringPattern.FindAllStringSubmatch(value, -1); len(match) > 0 {
			return strings.TrimSpace(match[0][1])
		}
		if textMatch := regexp.MustCompile(`(?:^|[,{\s])text\s*=\s*["']([^"']+)["']`).FindStringSubmatch(value); textMatch != nil {
			return strings.TrimSpace(textMatch[1])
		}
	}
	return ""
}

func findJSONKeyLine(data []byte, key string) int {
	pattern := `"` + regexp.QuoteMeta(strings.TrimSpace(key)) + `"\s*:`
	re := regexp.MustCompile(pattern)
	lines := strings.Split(string(data), "\n")
	for idx, line := range lines {
		if re.MatchString(line) {
			return idx + 1
		}
	}
	return 0
}

func firstQuotedValue(line string) string {
	matches := quotedStringPattern.FindAllStringSubmatch(line, -1)
	if len(matches) == 0 {
		return ""
	}
	return strings.TrimSpace(matches[0][1])
}

func isGoVersionPinned(version string) bool {
	version = strings.TrimSpace(version)
	return version != "" && !strings.ContainsAny(version, "<>=~^*")
}

func isNodeVersionPinned(version string) bool {
	version = strings.TrimSpace(version)
	if version == "" {
		return false
	}
	lowered := strings.ToLower(version)
	if strings.HasPrefix(lowered, "^") || strings.HasPrefix(lowered, "~") || strings.HasPrefix(lowered, ">") || strings.HasPrefix(lowered, "<") {
		return false
	}
	if strings.Contains(lowered, "||") || strings.ContainsAny(lowered, "*x") {
		return false
	}
	if strings.HasPrefix(lowered, "workspace:") || strings.HasPrefix(lowered, "file:") || strings.HasPrefix(lowered, "link:") || strings.HasPrefix(lowered, "git") || strings.HasPrefix(lowered, "http:") || strings.HasPrefix(lowered, "https:") {
		return false
	}
	return lowered != "latest"
}

func isPythonRequirementPinned(req string) bool {
	req = strings.TrimSpace(req)
	return strings.Contains(req, "===") || strings.Contains(req, "==") || strings.Contains(req, " @ ")
}

func isCargoVersionPinned(version string) bool {
	version = strings.TrimSpace(version)
	return strings.HasPrefix(version, "=") && len(version) > 1
}
