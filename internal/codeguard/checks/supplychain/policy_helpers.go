package supplychain

import (
	"path/filepath"
	"regexp"
	"slices"
	"strings"

	"github.com/devr-tools/codeguard/internal/codeguard/core"
)

func changedFilesSet(paths []string) map[string]struct{} {
	if len(paths) == 0 {
		return nil
	}
	out := make(map[string]struct{}, len(paths))
	for _, path := range paths {
		out[filepath.ToSlash(path)] = struct{}{}
	}
	return out
}

func manifestExpectsLockfile(manifest core.SupplyChainManifest) bool {
	switch manifest.Ecosystem {
	case "go", "npm", "cargo", "conan":
		return true
	case "python":
		return manifest.PackageManager == "poetry" || manifest.PackageManager == "uv"
	default:
		return false
	}
}

func normalizeLicenseList(values []string) []string {
	normalized := make([]string, 0, len(values))
	for _, value := range values {
		if trimmed := strings.ToUpper(strings.TrimSpace(value)); trimmed != "" {
			normalized = append(normalized, trimmed)
		}
	}
	slices.Sort(normalized)
	return slices.Compact(normalized)
}

var licenseTokenPattern = regexp.MustCompile(`[A-Za-z0-9.+-]+`)

func normalizeLicenseExpression(value string) (string, []string) {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return "", nil
	}
	normalized := strings.ToUpper(trimmed)
	tokens := licenseTokenPattern.FindAllString(normalized, -1)
	filtered := make([]string, 0, len(tokens))
	for _, token := range tokens {
		switch token {
		case "AND", "OR", "WITH":
			continue
		default:
			filtered = append(filtered, token)
		}
	}
	slices.Sort(filtered)
	filtered = slices.Compact(filtered)
	return normalized, filtered
}

func licenseDenied(normalized string, tokens []string, denied []string) bool {
	if len(denied) == 0 {
		return false
	}
	for _, deniedLicense := range denied {
		if normalized == deniedLicense {
			return true
		}
		for _, token := range tokens {
			if token == deniedLicense {
				return true
			}
		}
	}
	return false
}

func licenseOutsideAllowed(normalized string, tokens []string, allowed []string) bool {
	if len(allowed) == 0 {
		return false
	}
	if normalized != "" && slices.Contains(allowed, normalized) {
		return false
	}
	if len(tokens) == 0 {
		return true
	}
	for _, token := range tokens {
		if !slices.Contains(allowed, token) {
			return true
		}
	}
	return false
}
