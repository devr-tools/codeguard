package design

import (
	"encoding/json"
	"path"
	"sort"
	"strings"
)

func indexTypeScriptResolverMetadata(resolver *typeScriptImportResolver, rel string, data []byte) {
	base := path.Base(rel)
	switch {
	case base == "package.json":
		indexTypeScriptPackageManifest(resolver, path.Dir(rel), data)
	case strings.HasSuffix(base, ".json"):
		indexTypeScriptTSConfig(resolver, rel, data)
	}
}

func indexTypeScriptPackageManifest(resolver *typeScriptImportResolver, dir string, data []byte) {
	var manifest typeScriptPackageManifest
	if err := json.Unmarshal(data, &manifest); err != nil {
		return
	}
	name := strings.TrimSpace(manifest.Name)
	if name == "" {
		return
	}
	resolver.packages[name] = typeScriptWorkspacePackage{
		name:    name,
		dir:     normalizeTypeScriptPath(dir),
		main:    strings.TrimSpace(manifest.Main),
		module:  strings.TrimSpace(manifest.Module),
		source:  strings.TrimSpace(manifest.Source),
		types:   strings.TrimSpace(manifest.Types),
		exports: parseTypeScriptPackageExports(manifest.Exports),
		imports: parseTypeScriptPackageImports(manifest.Imports),
	}
}

func indexTypeScriptTSConfig(resolver *typeScriptImportResolver, rel string, data []byte) {
	var doc typeScriptConfigDocument
	normalized, ok := normalizeJSONC(data)
	if !ok || json.Unmarshal(normalized, &doc) != nil {
		return
	}
	rel = normalizeTypeScriptPath(rel)
	resolver.tsconfigs[rel] = doc
	dir := normalizeTypeScriptPath(path.Dir(rel))
	primary, ok := resolver.tsconfigPrimary[dir]
	if !ok || path.Base(rel) == "tsconfig.json" || path.Base(primary) != "tsconfig.json" {
		resolver.tsconfigPrimary[dir] = rel
	}
}

func finalizeTypeScriptResolverConfigs(resolver *typeScriptImportResolver) {
	resolver.configs = resolver.configs[:0]
	dirs := make([]string, 0, len(resolver.tsconfigPrimary))
	for dir := range resolver.tsconfigPrimary {
		dirs = append(dirs, dir)
	}
	sort.Slice(dirs, func(i, j int) bool {
		return len(dirs[i]) > len(dirs[j])
	})
	cache := make(map[string]typeScriptGraphConfig, len(dirs))
	for _, dir := range dirs {
		cfg, ok := effectiveTypeScriptConfig(resolver, resolver.tsconfigPrimary[dir], cache, make(map[string]bool))
		if !ok {
			continue
		}
		resolver.configs = append(resolver.configs, cfg)
	}
}
