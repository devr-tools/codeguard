package support

import (
	"path/filepath"
	"strings"
)

// IsNestJSBoundaryPath recognizes Nest request-boundary modules where
// decorators and framework conventions define command/orchestration behavior.
func IsNestJSBoundaryPath(file string) bool {
	normalized := strings.ToLower(strings.ReplaceAll(file, "\\", "/"))
	base := filepath.Base(normalized)
	if strings.Contains(normalized, "/controllers/") ||
		strings.Contains(normalized, "/resolvers/") ||
		strings.Contains(normalized, "/gateways/") {
		return true
	}
	for _, suffix := range []string{
		".controller.ts",
		".controller.tsx",
		".controller.js",
		".controller.jsx",
		".resolver.ts",
		".resolver.tsx",
		".resolver.js",
		".resolver.jsx",
		".gateway.ts",
		".gateway.tsx",
		".gateway.js",
		".gateway.jsx",
	} {
		if strings.HasSuffix(base, suffix) {
			return true
		}
	}
	return false
}
