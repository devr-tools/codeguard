package support

import (
	"encoding/json"
	"os"
	"path"
	"path/filepath"
	"slices"
	"strings"

	"github.com/devr-tools/codeguard/internal/codeguard/core"
)

func resolveNodeDependencyLicense(root string, manifest core.SupplyChainManifest, dep core.SupplyChainDependency) (string, string) {
	for _, searchRoot := range manifestSearchRoots(root, manifest.Path) {
		manifestPath := filepath.Join(searchRoot, filepath.FromSlash(path.Join("node_modules", dep.Name, "package.json")))
		data, err := os.ReadFile(manifestPath)
		if err != nil {
			continue
		}
		var pkg struct {
			License  any `json:"license"`
			Licenses any `json:"licenses"`
		}
		if err := json.Unmarshal(data, &pkg); err != nil {
			continue
		}
		if license := parseJSONLicense(pkg.License); license != "" {
			return license, "node_modules"
		}
		if license := parseJSONLicenses(pkg.Licenses); license != "" {
			return license, "node_modules"
		}
	}
	return "", ""
}

func resolveCargoDependencyLicense(root string, manifest core.SupplyChainManifest, dep core.SupplyChainDependency) (string, string) {
	for _, searchRoot := range manifestSearchRoots(root, manifest.Path) {
		candidate := filepath.Join(searchRoot, "vendor", filepath.FromSlash(dep.Name), "Cargo.toml")
		data, err := os.ReadFile(candidate)
		if err != nil {
			continue
		}
		if license := parseCargoTOMLLicense(data); license != "" {
			return license, "cargo-vendor"
		}
	}
	return "", ""
}

func resolvePythonDependencyLicense(root string, manifest core.SupplyChainManifest, dep core.SupplyChainDependency) (string, string) {
	for _, searchRoot := range manifestSearchRoots(root, manifest.Path) {
		for _, pattern := range pythonMetadataPatterns(searchRoot, dep.Name) {
			matches, _ := filepath.Glob(pattern)
			for _, match := range matches {
				data, err := os.ReadFile(match)
				if err != nil {
					continue
				}
				if license := parsePythonMetadataLicense(data); license != "" {
					return license, pythonMetadataSource(match)
				}
			}
		}
	}
	return "", ""
}

func manifestRelativeDir(manifestPath string) string {
	dir := path.Dir(filepath.ToSlash(manifestPath))
	if dir == "." || dir == "/" {
		return ""
	}
	return dir
}

func manifestSearchRoots(root string, manifestPath string) []string {
	relDir := manifestRelativeDir(manifestPath)
	dirs := []string{filepath.Clean(root)}
	if relDir == "" {
		return dirs
	}
	dirs = []string{filepath.Join(root, filepath.FromSlash(relDir))}
	for current := relDir; current != "" && current != "." && current != "/"; {
		parent := path.Dir(current)
		if parent == "." || parent == "/" {
			break
		}
		dirs = append(dirs, filepath.Join(root, filepath.FromSlash(parent)))
		current = parent
	}
	dirs = append(dirs, filepath.Clean(root))
	return slices.Compact(dirs)
}

func parseJSONLicenses(value any) string {
	switch typed := value.(type) {
	case []any:
		licenses := make([]string, 0, len(typed))
		for _, item := range typed {
			if text := parseJSONLicense(item); text != "" {
				licenses = append(licenses, text)
			}
		}
		return strings.Join(licenses, " OR ")
	case map[string]any:
		return parseJSONLicense(typed)
	default:
		return ""
	}
}

func parseCargoTOMLLicense(data []byte) string {
	for _, rawLine := range strings.Split(string(data), "\n") {
		line := strings.TrimSpace(rawLine)
		if strings.HasPrefix(line, "license") && strings.Contains(line, "=") {
			return parseTOMLLicenseValue(line)
		}
	}
	return ""
}

func normalizePythonDistName(name string) string {
	lowered := strings.ToLower(strings.TrimSpace(name))
	return strings.NewReplacer("-", "_", ".", "_").Replace(lowered)
}

func parsePythonMetadataLicense(data []byte) string {
	lines := strings.Split(string(data), "\n")
	var classifierLicenses []string
	for _, rawLine := range lines {
		line := strings.TrimSpace(rawLine)
		switch {
		case strings.HasPrefix(line, "License:"):
			value := strings.TrimSpace(strings.TrimPrefix(line, "License:"))
			if value != "" && value != "UNKNOWN" {
				return value
			}
		case strings.HasPrefix(line, "Classifier: License ::"):
			classifierLicenses = append(classifierLicenses, strings.TrimSpace(strings.TrimPrefix(line, "Classifier: License ::")))
		}
	}
	if len(classifierLicenses) > 0 {
		return strings.Join(classifierLicenses, " OR ")
	}
	return ""
}

func pythonMetadataPatterns(searchRoot string, depName string) []string {
	dist := normalizePythonDistName(depName) + "-*.dist-info"
	return []string{
		filepath.Join(searchRoot, ".venv", "lib", "*", "site-packages", dist, "METADATA"),
		filepath.Join(searchRoot, ".venv", "Lib", "site-packages", dist, "METADATA"),
		filepath.Join(searchRoot, "site-packages", dist, "METADATA"),
	}
}

func pythonMetadataSource(match string) string {
	if strings.Contains(match, ".venv") {
		return ".venv dist-info"
	}
	return "python-dist-info"
}
