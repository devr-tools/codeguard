package support

import (
	"encoding/json"
	"regexp"
	"strings"

	"github.com/devr-tools/codeguard/internal/codeguard/core"
)

var vcpkgBaselinePattern = regexp.MustCompile(`^[0-9a-fA-F]{40}$`)

func parseVCPKGManifest(root string, rel string, data []byte) (core.SupplyChainManifest, bool) {
	var raw struct {
		Name            string            `json:"name"`
		BuiltinBaseline string            `json:"builtin-baseline"`
		Dependencies    []json.RawMessage `json:"dependencies"`
		Overrides       []struct {
			Name    string `json:"name"`
			Version string `json:"version"`
		} `json:"overrides"`
	}
	if err := json.Unmarshal(data, &raw); err != nil {
		return core.SupplyChainManifest{}, false
	}
	overrides := make(map[string]string, len(raw.Overrides))
	for _, override := range raw.Overrides {
		if name, version := strings.TrimSpace(override.Name), strings.TrimSpace(override.Version); name != "" && version != "" {
			overrides[name] = version
		}
	}
	baselinePinned := vcpkgBaselinePattern.MatchString(strings.TrimSpace(raw.BuiltinBaseline))
	manifest := core.SupplyChainManifest{
		Ecosystem:      "vcpkg",
		PackageManager: "vcpkg",
		Path:           rel,
		Name:           strings.TrimSpace(raw.Name),
		Lockfiles:      presentLockfiles(root, rel, []string{"vcpkg-lock.json"}),
	}
	for _, entry := range raw.Dependencies {
		name, constraint, scope, ok := parseVCPKGDependency(entry)
		if !ok {
			continue
		}
		version := strings.TrimSpace(overrides[name])
		requirement := constraint
		pinned := version != "" || baselinePinned
		if version != "" {
			requirement = version
		} else if baselinePinned && requirement == "" {
			requirement = "builtin-baseline@" + strings.TrimSpace(raw.BuiltinBaseline)
		}
		manifest.Dependencies = append(manifest.Dependencies, core.SupplyChainDependency{
			Name:        name,
			Requirement: requirement,
			Version:     version,
			Scope:       scope,
			Pinned:      pinned,
			Line:        findJSONKeyLine(data, name),
		})
	}
	sortDependencies(manifest.Dependencies)
	return manifest, true
}

func parseVCPKGDependency(data json.RawMessage) (name string, constraint string, scope string, ok bool) {
	var simple string
	if err := json.Unmarshal(data, &simple); err == nil {
		name = strings.TrimSpace(simple)
		return name, "", "runtime", name != ""
	}
	var object struct {
		Name       string `json:"name"`
		VersionMin string `json:"version>="`
		Host       bool   `json:"host"`
	}
	if err := json.Unmarshal(data, &object); err != nil {
		return "", "", "", false
	}
	scope = "runtime"
	if object.Host {
		scope = "build"
	}
	name = strings.TrimSpace(object.Name)
	return name, strings.TrimSpace(object.VersionMin), scope, name != ""
}

func parseConanTextManifest(root string, rel string, data []byte) core.SupplyChainManifest {
	manifest := core.SupplyChainManifest{
		Ecosystem:      "conan",
		PackageManager: "conan",
		Path:           rel,
		Lockfiles:      presentLockfiles(root, rel, []string{"conan.lock"}),
	}
	section := ""
	for idx, rawLine := range strings.Split(string(data), "\n") {
		line := strings.TrimSpace(rawLine)
		if strings.HasPrefix(line, "[") && strings.HasSuffix(line, "]") {
			section = strings.ToLower(strings.TrimSpace(strings.TrimSuffix(strings.TrimPrefix(line, "["), "]")))
			continue
		}
		if section != "requires" && section != "tool_requires" {
			continue
		}
		if strings.HasPrefix(line, "#") {
			continue
		}
		if comment := strings.Index(line, ";"); comment >= 0 {
			line = strings.TrimSpace(line[:comment])
		}
		if dep, ok := parseConanReference(line, section, idx+1); ok {
			manifest.Dependencies = append(manifest.Dependencies, dep)
		}
	}
	sortDependencies(manifest.Dependencies)
	return manifest
}

func parseConanReference(reference string, section string, line int) (core.SupplyChainDependency, bool) {
	reference = strings.TrimSpace(reference)
	if reference == "" {
		return core.SupplyChainDependency{}, false
	}
	name, remainder, ok := strings.Cut(reference, "/")
	if !ok || strings.TrimSpace(name) == "" || strings.TrimSpace(remainder) == "" {
		return core.SupplyChainDependency{}, false
	}
	version := remainder
	if idx := strings.IndexAny(version, "@#"); idx >= 0 {
		version = version[:idx]
	}
	version = strings.TrimSpace(version)
	scope := "runtime"
	if section == "tool_requires" {
		scope = "build"
	}
	lowered := strings.ToLower(version)
	pinned := version != "" && !strings.ContainsAny(version, "[]<>=~^*,|") && lowered != "latest"
	return core.SupplyChainDependency{
		Name:        strings.TrimSpace(name),
		Requirement: reference,
		Version:     version,
		Scope:       scope,
		Pinned:      pinned,
		Line:        line,
	}, true
}

func parseConanLockState(data []byte) (lockfileState, bool) {
	var lock struct {
		Requires       []string `json:"requires"`
		BuildRequires  []string `json:"build_requires"`
		PythonRequires []string `json:"python_requires"`
		GraphLock      struct {
			Nodes map[string]struct {
				Ref string `json:"ref"`
			} `json:"nodes"`
		} `json:"graph_lock"`
	}
	if err := json.Unmarshal(data, &lock); err != nil {
		return lockfileState{}, false
	}
	state := newLockfileState()
	refs := append([]string(nil), lock.Requires...)
	refs = append(refs, lock.BuildRequires...)
	refs = append(refs, lock.PythonRequires...)
	for _, node := range lock.GraphLock.Nodes {
		refs = append(refs, node.Ref)
	}
	for _, ref := range refs {
		dep, ok := parseConanReference(ref, "requires", 0)
		if !ok {
			continue
		}
		addLockfilePackage(state, dep.Name, dep.Version)
	}
	return state, len(state.packages) > 0
}
