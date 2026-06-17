package support

import (
	"strings"

	"github.com/devr-tools/codeguard/internal/codeguard/core"
)

func parseCargoManifest(root string, rel string, data []byte) core.SupplyChainManifest {
	manifest := core.SupplyChainManifest{
		Ecosystem: "cargo",
		Path:      rel,
		Lockfiles: presentLockfiles(root, rel, []string{"Cargo.lock"}),
	}
	section := ""
	for idx, rawLine := range strings.Split(string(data), "\n") {
		line := strings.TrimSpace(rawLine)
		if match := tomlSectionPattern.FindStringSubmatch(rawLine); match != nil {
			section = strings.TrimSpace(match[1])
			continue
		}
		switch {
		case section == "package":
			applyManifestIdentityLine(&manifest, line, idx+1)
		case isCargoDependencySection(section):
			if dep, ok := parseCargoDependencyLine(line, cargoScope(section), idx+1); ok {
				manifest.Dependencies = append(manifest.Dependencies, dep)
			}
		}
	}
	sortDependencies(manifest.Dependencies)
	return manifest
}

func isCargoDependencySection(section string) bool {
	switch {
	case section == "dependencies", section == "dev-dependencies", section == "build-dependencies":
		return true
	case strings.HasSuffix(section, ".dependencies"), strings.HasSuffix(section, ".dev-dependencies"), strings.HasSuffix(section, ".build-dependencies"):
		return strings.HasPrefix(section, "target.")
	default:
		return false
	}
}

func cargoScope(section string) string {
	switch {
	case strings.Contains(section, "dev-dependencies"):
		return "dev"
	case strings.Contains(section, "build-dependencies"):
		return "build"
	default:
		return "runtime"
	}
}

func parseCargoDependencyLine(line string, scope string, lineNo int) (core.SupplyChainDependency, bool) {
	match := tomlKeyPattern.FindStringSubmatch(line)
	if match == nil {
		return core.SupplyChainDependency{}, false
	}
	name := match[1]
	if name == "" {
		name = match[2]
	}
	value := strings.TrimSpace(line[strings.Index(line, "=")+1:])
	value = strings.TrimSpace(strings.TrimSuffix(strings.TrimPrefix(value, "{"), "}"))
	version := ""
	if strings.HasPrefix(strings.TrimSpace(line[strings.Index(line, "=")+1:]), `"`) {
		version = firstQuotedValue(line)
	} else if match := cargoInlineVersionPattern.FindStringSubmatch(value); match != nil {
		version = strings.TrimSpace(match[1])
	}
	req := version
	if strings.TrimSpace(req) == "" {
		req = strings.TrimSpace(line[strings.Index(line, "=")+1:])
	}
	return core.SupplyChainDependency{
		Name:        name,
		Requirement: req,
		Version:     version,
		Scope:       scope,
		Pinned:      isCargoVersionPinned(version),
		Line:        lineNo,
	}, true
}
