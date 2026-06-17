package support

import (
	"strings"

	"github.com/devr-tools/codeguard/internal/codeguard/core"
)

type pyprojectParseState struct {
	section              string
	inDependenciesArray  bool
	currentOptionalGroup string
}

func parseRequirementsManifest(root string, rel string, data []byte) core.SupplyChainManifest {
	manifest := core.SupplyChainManifest{
		Ecosystem: "python",
		Path:      rel,
		Lockfiles: presentLockfiles(root, rel, []string{"poetry.lock", "uv.lock"}),
	}
	for idx, rawLine := range strings.Split(string(data), "\n") {
		dep, ok := parsePythonRequirementLine(rawLine, "runtime", idx+1)
		if ok {
			manifest.Dependencies = append(manifest.Dependencies, dep)
		}
	}
	sortDependencies(manifest.Dependencies)
	return manifest
}

func parsePyprojectManifest(root string, rel string, data []byte) core.SupplyChainManifest {
	manifest := core.SupplyChainManifest{
		Ecosystem: "python",
		Path:      rel,
		Lockfiles: presentLockfiles(root, rel, []string{"poetry.lock", "uv.lock"}),
	}
	state := pyprojectParseState{}
	for idx, rawLine := range strings.Split(string(data), "\n") {
		if next, ok := nextPyprojectSection(rawLine); ok {
			state = next
			continue
		}
		applyPyprojectLine(&manifest, &state, rawLine, idx+1)
	}
	sortDependencies(manifest.Dependencies)
	return manifest
}

func nextPyprojectSection(rawLine string) (pyprojectParseState, bool) {
	match := tomlSectionPattern.FindStringSubmatch(rawLine)
	if match == nil {
		return pyprojectParseState{}, false
	}
	section := strings.TrimSpace(match[1])
	state := pyprojectParseState{section: section}
	switch {
	case strings.HasPrefix(section, "project.optional-dependencies."):
		state.currentOptionalGroup = strings.TrimPrefix(section, "project.optional-dependencies.")
	case strings.HasPrefix(section, "dependency-groups."):
		state.currentOptionalGroup = strings.TrimPrefix(section, "dependency-groups.")
	}
	return state, true
}

func applyPyprojectLine(manifest *core.SupplyChainManifest, state *pyprojectParseState, rawLine string, lineNo int) {
	line := strings.TrimSpace(rawLine)
	switch {
	case state.section == "project":
		applyPyprojectProjectLine(manifest, state, rawLine, line, lineNo)
	case strings.HasPrefix(state.section, "project.optional-dependencies"):
		appendPyprojectQuotedDependencies(manifest, rawLine, "optional", state.currentOptionalGroup, lineNo)
	case strings.HasPrefix(state.section, "dependency-groups"):
		appendPyprojectQuotedDependencies(manifest, rawLine, "dev", state.currentOptionalGroup, lineNo)
	case isPoetryDependencySection(state.section):
		applyPoetryDependencyLine(manifest, state.section, line, lineNo)
	case state.section == "tool.poetry":
		applyPoetryMetadataLine(manifest, line, lineNo)
	case state.section == "tool.uv":
		if manifest.PackageManager == "" {
			manifest.PackageManager = "uv"
		}
	}
}

func applyPyprojectProjectLine(manifest *core.SupplyChainManifest, state *pyprojectParseState, rawLine string, line string, lineNo int) {
	applyManifestIdentityLine(manifest, line, lineNo)
	if strings.HasPrefix(line, "dependencies") && strings.Contains(line, "=") {
		state.inDependenciesArray = true
	}
	if !state.inDependenciesArray {
		return
	}
	appendPyprojectQuotedDependencies(manifest, rawLine, "runtime", "", lineNo)
	if strings.Contains(line, "]") {
		state.inDependenciesArray = false
	}
}

func appendPyprojectQuotedDependencies(manifest *core.SupplyChainManifest, rawLine string, scope string, group string, lineNo int) {
	for _, match := range quotedStringPattern.FindAllStringSubmatch(rawLine, -1) {
		dep, ok := parsePythonRequirementLine(match[1], scope, lineNo)
		if !ok {
			continue
		}
		if group != "" {
			dep.Groups = []string{group}
		}
		manifest.Dependencies = append(manifest.Dependencies, dep)
	}
}

func parsePythonRequirementLine(line string, scope string, lineNo int) (core.SupplyChainDependency, bool) {
	line = strings.TrimSpace(line)
	if line == "" || strings.HasPrefix(line, "#") || strings.HasPrefix(line, "-") {
		return core.SupplyChainDependency{}, false
	}
	if idx := strings.Index(line, "#"); idx >= 0 {
		line = strings.TrimSpace(line[:idx])
	}
	match := requirementNamePattern.FindStringSubmatch(line)
	if match == nil {
		return core.SupplyChainDependency{}, false
	}
	name := match[1]
	return core.SupplyChainDependency{
		Name:        name,
		Requirement: line,
		Version:     pythonRequirementVersion(line),
		Scope:       scope,
		Pinned:      isPythonRequirementPinned(line),
		Line:        lineNo,
	}, true
}

func pythonRequirementVersion(line string) string {
	operators := []string{"===", "==", "~=", ">=", "<=", "!=", ">", "<"}
	for _, op := range operators {
		if idx := strings.Index(line, op); idx >= 0 {
			return strings.TrimSpace(line[idx+len(op):])
		}
	}
	if idx := strings.Index(line, "@"); idx >= 0 {
		return strings.TrimSpace(line[idx+1:])
	}
	return ""
}
