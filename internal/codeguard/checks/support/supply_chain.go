package support

import (
	"os"
	"path"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"github.com/devr-tools/codeguard/internal/codeguard/core"
)

var (
	requirementNamePattern    = regexp.MustCompile(`^([A-Za-z0-9][A-Za-z0-9._-]*)`)
	tomlSectionPattern        = regexp.MustCompile(`^\s*\[([^\]]+)\]\s*$`)
	tomlKeyPattern            = regexp.MustCompile(`^\s*(?:"([^"]+)"|([A-Za-z0-9._-]+))\s*=`)
	quotedStringPattern       = regexp.MustCompile(`["']([^"']+)["']`)
	cargoInlineVersionPattern = regexp.MustCompile(`(?:^|[,{\s])version\s*=\s*["']([^"']+)["']`)
	knownManifestNames        = map[string]bool{
		"go.mod": true, "package.json": true, "pyproject.toml": true,
		"cargo.toml": true, "vcpkg.json": true, "conanfile.txt": true,
		"conanfile.py": true,
	}
)

func IsSupplyChainManifest(rel string) bool {
	normalized := filepath.ToSlash(rel)
	if isInstalledDependencyMetadataPath(normalized) {
		return false
	}
	base := strings.ToLower(path.Base(normalized))
	return knownManifestNames[base] || isCMakeManifestName(base) || isRequirementsManifestName(base)
}

func CollectSupplyChainManifests(env Context, target core.TargetConfig) []core.SupplyChainManifest {
	type manifestFile struct {
		rel  string
		data []byte
	}

	files := make([]manifestFile, 0)
	env.VisitTargetFiles(target, IsSupplyChainManifest, func(rel string, data []byte) {
		files = append(files, manifestFile{
			rel:  filepath.ToSlash(rel),
			data: append([]byte(nil), data...),
		})
	})
	if len(files) == 0 {
		return nil
	}

	sort.Slice(files, func(i, j int) bool { return files[i].rel < files[j].rel })
	manifests := make([]core.SupplyChainManifest, 0, len(files))
	for _, file := range files {
		manifest, ok := parseSupplyChainManifest(target.Path, file.rel, file.data)
		if !ok {
			continue
		}
		manifests = append(manifests, manifest)
	}
	sort.Slice(manifests, func(i, j int) bool { return manifests[i].Path < manifests[j].Path })
	return manifests
}

func parseSupplyChainManifest(root string, rel string, data []byte) (core.SupplyChainManifest, bool) {
	switch strings.ToLower(path.Base(rel)) {
	case "go.mod":
		return parseGoModManifest(root, rel, data), true
	case "package.json":
		return parsePackageJSONManifest(root, rel, data)
	case "pyproject.toml":
		return parsePyprojectManifest(root, rel, data), true
	case "cargo.toml":
		return parseCargoManifest(root, rel, data), true
	case "vcpkg.json":
		return parseVCPKGManifest(root, rel, data)
	case "conanfile.txt":
		return parseConanTextManifest(root, rel, data), true
	case "conanfile.py":
		return parseConanPythonManifest(root, rel, data), true
	default:
		return parseOtherSupplyChainManifest(root, rel, data)
	}
}

func parseOtherSupplyChainManifest(root, rel string, data []byte) (core.SupplyChainManifest, bool) {
	base := strings.ToLower(path.Base(rel))
	if isCMakeManifestName(base) {
		manifest := parseCMakeManifest(root, rel, data)
		complete := base == "cmakelists.txt" || len(manifest.Dependencies) > 0 || len(manifest.AnalysisLimitations) > 0
		return manifest, complete
	}
	if isRequirementsManifestName(base) {
		return parseRequirementsManifest(root, rel, data), true
	}
	return core.SupplyChainManifest{}, false
}

func isCMakeManifestName(base string) bool {
	return base == "cmakelists.txt" || strings.HasSuffix(base, ".cmake")
}

func isRequirementsManifestName(base string) bool {
	return strings.HasPrefix(base, "requirements") && strings.HasSuffix(base, ".txt")
}

func isInstalledDependencyMetadataPath(rel string) bool {
	parts := strings.Split(filepath.ToSlash(rel), "/")
	for _, part := range parts {
		switch part {
		case "node_modules", "vendor", ".venv", "site-packages":
			return true
		}
	}
	return false
}

func presentLockfiles(root string, manifestPath string, candidates []string) []string {
	dir := path.Dir(manifestPath)
	if dir == "." {
		dir = ""
	}
	lockfiles := make([]string, 0, len(candidates))
	for _, candidate := range candidates {
		rel := candidate
		if dir != "" {
			rel = path.Join(dir, candidate)
		}
		info, err := os.Stat(filepath.Join(root, filepath.FromSlash(rel)))
		if err == nil && !info.IsDir() {
			lockfiles = append(lockfiles, rel)
		}
	}
	sort.Strings(lockfiles)
	return lockfiles
}

func sortDependencies(deps []core.SupplyChainDependency) {
	sort.Slice(deps, func(i, j int) bool {
		if deps[i].Name == deps[j].Name {
			if deps[i].Scope == deps[j].Scope {
				return deps[i].Requirement < deps[j].Requirement
			}
			return deps[i].Scope < deps[j].Scope
		}
		return deps[i].Name < deps[j].Name
	})
}
