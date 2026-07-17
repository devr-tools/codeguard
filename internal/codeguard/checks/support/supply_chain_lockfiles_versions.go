package support

import (
	"strings"

	"github.com/devr-tools/codeguard/internal/codeguard/core"
)

func exactLockedVersion(manifest core.SupplyChainManifest, dep core.SupplyChainDependency) string {
	switch manifest.Ecosystem {
	case "go":
		return strings.TrimSpace(dep.Version)
	case "npm":
		if dep.Pinned && isExactSemver(dep.Version) {
			return strings.TrimSpace(dep.Version)
		}
	case "cargo":
		if dep.Pinned {
			return strings.TrimSpace(dep.Version)
		}
	case "conan":
		if dep.Pinned {
			return strings.TrimSpace(dep.Version)
		}
	case "python":
		if strings.HasPrefix(strings.TrimSpace(dep.Requirement), "==") || strings.Contains(strings.TrimSpace(dep.Requirement), "==") {
			return trimPythonVersion(dep.Version)
		}
	}
	return ""
}

func trimPythonVersion(version string) string {
	version = strings.TrimSpace(version)
	if idx := strings.Index(version, ";"); idx >= 0 {
		version = strings.TrimSpace(version[:idx])
	}
	return version
}

func isExactSemver(version string) bool {
	version = strings.TrimSpace(version)
	if version == "" {
		return false
	}
	return !strings.ContainsAny(version, "^~<>=*| ")
}
