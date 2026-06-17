package support

import (
	"encoding/json"
	"sort"
	"strings"

	"github.com/devr-tools/codeguard/internal/codeguard/core"
)

func parseGoModManifest(root string, rel string, data []byte) core.SupplyChainManifest {
	manifest := core.SupplyChainManifest{
		Ecosystem: "go",
		Path:      rel,
		Lockfiles: presentLockfiles(root, rel, []string{"go.sum"}),
	}
	inRequireBlock := false
	for idx, rawLine := range strings.Split(string(data), "\n") {
		line := strings.TrimSpace(rawLine)
		switch {
		case strings.HasPrefix(line, "module "):
			manifest.Name = strings.TrimSpace(strings.TrimPrefix(line, "module "))
		case strings.HasPrefix(line, "require ("):
			inRequireBlock = true
		case inRequireBlock && line == ")":
			inRequireBlock = false
		case strings.HasPrefix(line, "require "):
			if dep, ok := parseGoRequireLine(strings.TrimSpace(strings.TrimPrefix(line, "require ")), idx+1); ok {
				manifest.Dependencies = append(manifest.Dependencies, dep)
			}
		case inRequireBlock && line != "":
			if dep, ok := parseGoRequireLine(line, idx+1); ok {
				manifest.Dependencies = append(manifest.Dependencies, dep)
			}
		}
	}
	sortDependencies(manifest.Dependencies)
	return manifest
}

func parseGoRequireLine(line string, lineNo int) (core.SupplyChainDependency, bool) {
	if line == "" || strings.HasPrefix(line, "//") {
		return core.SupplyChainDependency{}, false
	}
	indirect := strings.Contains(line, "// indirect")
	line = strings.TrimSpace(strings.TrimSuffix(line, "// indirect"))
	fields := strings.Fields(line)
	if len(fields) < 2 {
		return core.SupplyChainDependency{}, false
	}
	return core.SupplyChainDependency{
		Name:        fields[0],
		Requirement: fields[1],
		Version:     fields[1],
		Scope:       "runtime",
		Indirect:    indirect,
		Pinned:      isGoVersionPinned(fields[1]),
		Line:        lineNo,
	}, true
}

func parsePackageJSONManifest(root string, rel string, data []byte) (core.SupplyChainManifest, bool) {
	var manifestData struct {
		Name                 string            `json:"name"`
		License              any               `json:"license"`
		PackageManager       string            `json:"packageManager"`
		Dependencies         map[string]string `json:"dependencies"`
		DevDependencies      map[string]string `json:"devDependencies"`
		PeerDependencies     map[string]string `json:"peerDependencies"`
		OptionalDependencies map[string]string `json:"optionalDependencies"`
	}
	if err := json.Unmarshal(data, &manifestData); err != nil {
		return core.SupplyChainManifest{}, false
	}
	manifest := core.SupplyChainManifest{
		Ecosystem:      "npm",
		Path:           rel,
		Name:           strings.TrimSpace(manifestData.Name),
		License:        parseJSONLicense(manifestData.License),
		LicenseLine:    findJSONKeyLine(data, "license"),
		PackageManager: packageManagerName(manifestData.PackageManager),
		Lockfiles:      presentLockfiles(root, rel, []string{"package-lock.json", "npm-shrinkwrap.json", "pnpm-lock.yaml", "yarn.lock", "bun.lock", "bun.lockb"}),
	}
	appendPackageJSONDeps(data, &manifest.Dependencies, manifestData.Dependencies, "runtime")
	appendPackageJSONDeps(data, &manifest.Dependencies, manifestData.DevDependencies, "dev")
	appendPackageJSONDeps(data, &manifest.Dependencies, manifestData.PeerDependencies, "peer")
	appendPackageJSONDeps(data, &manifest.Dependencies, manifestData.OptionalDependencies, "optional")
	sortDependencies(manifest.Dependencies)
	return manifest, true
}

func appendPackageJSONDeps(data []byte, dst *[]core.SupplyChainDependency, deps map[string]string, scope string) {
	if len(deps) == 0 {
		return
	}
	names := make([]string, 0, len(deps))
	for name := range deps {
		names = append(names, name)
	}
	sort.Strings(names)
	for _, name := range names {
		req := strings.TrimSpace(deps[name])
		*dst = append(*dst, core.SupplyChainDependency{
			Name:        name,
			Requirement: req,
			Version:     req,
			Scope:       scope,
			Pinned:      isNodeVersionPinned(req),
			Line:        findJSONKeyLine(data, name),
		})
	}
}
