package support

import (
	"strings"

	"github.com/devr-tools/codeguard/internal/codeguard/core"
)

func applyPoetryDependencyLine(manifest *core.SupplyChainManifest, section string, line string, lineNo int) {
	if manifest.PackageManager == "" {
		manifest.PackageManager = "poetry"
	}
	if dep, ok := parsePoetryDependencyLine(line, section, lineNo); ok {
		manifest.Dependencies = append(manifest.Dependencies, dep)
	}
}

func applyPoetryMetadataLine(manifest *core.SupplyChainManifest, line string, lineNo int) {
	if manifest.PackageManager == "" {
		manifest.PackageManager = "poetry"
	}
	if strings.HasPrefix(line, "license") && strings.Contains(line, "=") {
		manifest.License = parseTOMLLicenseValue(line)
		manifest.LicenseLine = lineNo
	}
}

func isPoetryDependencySection(section string) bool {
	if section == "tool.poetry.dependencies" || section == "tool.poetry.dev-dependencies" {
		return true
	}
	return strings.HasPrefix(section, "tool.poetry.group.") && strings.HasSuffix(section, ".dependencies")
}

func parsePoetryDependencyLine(line string, section string, lineNo int) (core.SupplyChainDependency, bool) {
	match := tomlKeyPattern.FindStringSubmatch(line)
	if match == nil {
		return core.SupplyChainDependency{}, false
	}
	name := match[1]
	if name == "" {
		name = match[2]
	}
	if strings.EqualFold(name, "python") {
		return core.SupplyChainDependency{}, false
	}
	req := strings.TrimSpace(line[strings.Index(line, "=")+1:])
	req = strings.Trim(req, `"`)
	scope := "runtime"
	group := ""
	if section == "tool.poetry.dev-dependencies" || strings.HasPrefix(section, "tool.poetry.group.") {
		scope = "dev"
		if strings.HasPrefix(section, "tool.poetry.group.") {
			group = strings.TrimSuffix(strings.TrimPrefix(section, "tool.poetry.group."), ".dependencies")
		}
	}
	dep := core.SupplyChainDependency{
		Name:        name,
		Requirement: req,
		Version:     extractPoetryVersion(req),
		Scope:       scope,
		Pinned:      isPythonRequirementPinned(req),
		Line:        lineNo,
	}
	if group != "" {
		dep.Groups = []string{group}
	}
	return dep, true
}

func extractPoetryVersion(req string) string {
	if match := cargoInlineVersionPattern.FindStringSubmatch(req); match != nil {
		return strings.TrimSpace(match[1])
	}
	return strings.TrimSpace(strings.Trim(req, `"`))
}
