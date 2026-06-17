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
)

func IsSupplyChainManifest(rel string) bool {
	normalized := filepath.ToSlash(rel)
	if isInstalledDependencyMetadataPath(normalized) {
		return false
	}
	base := strings.ToLower(path.Base(filepath.ToSlash(rel)))
	switch {
	case base == "go.mod":
		return true
	case base == "package.json":
		return true
	case base == "pyproject.toml":
		return true
	case base == "cargo.toml":
		return true
	case strings.HasPrefix(base, "requirements") && strings.HasSuffix(base, ".txt"):
		return true
	default:
		return false
	}
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
	default:
		if strings.HasPrefix(strings.ToLower(path.Base(rel)), "requirements") && strings.HasSuffix(strings.ToLower(path.Base(rel)), ".txt") {
			return parseRequirementsManifest(root, rel, data), true
		}
	}
	return core.SupplyChainManifest{}, false
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
