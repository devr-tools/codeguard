package design

import (
	"path"
	"strings"
)

func resolveTypeScriptModuleImport(resolver *typeScriptImportResolver, fromModule string, specifier string) string {
	if strings.HasPrefix(specifier, "./") || strings.HasPrefix(specifier, "../") || specifier == "." || specifier == ".." {
		return resolveRelativeTypeScriptImport(resolver, fromModule, specifier)
	}
	if resolved := resolveTypeScriptPackageImport(resolver, fromModule, specifier); resolved != "" {
		return resolved
	}
	if resolved := resolveTypeScriptConfigAlias(resolver, fromModule, specifier); resolved != "" {
		return resolved
	}
	return resolveTypeScriptWorkspacePackage(resolver, specifier)
}

func resolveRelativeTypeScriptImport(resolver *typeScriptImportResolver, fromModule string, specifier string) string {
	return resolveTypeScriptModulePath(resolver, path.Clean(path.Join(path.Dir(fromModule), specifier)))
}

func resolveTypeScriptPackageImport(resolver *typeScriptImportResolver, fromModule string, specifier string) string {
	if !strings.HasPrefix(specifier, "#") {
		return ""
	}
	pkg, ok := typeScriptPackageForModule(resolver, fromModule)
	if !ok {
		return ""
	}
	for _, candidate := range matchTypeScriptMapping(pkg.imports, specifier) {
		if resolved := resolveTypeScriptModulePath(resolver, path.Join(pkg.dir, candidate)); resolved != "" {
			return resolved
		}
	}
	return ""
}

func resolveTypeScriptConfigAlias(resolver *typeScriptImportResolver, fromModule string, specifier string) string {
	cfg := typeScriptConfigForModule(resolver, fromModule)
	if cfg == nil {
		return ""
	}
	for _, alias := range cfg.paths {
		wildcard, ok := matchTypeScriptAlias(alias.pattern, specifier)
		if !ok {
			continue
		}
		for _, target := range alias.targets {
			applied := applyTypeScriptAliasTarget(target, wildcard)
			if resolved := resolveTypeScriptModulePath(resolver, path.Join(cfg.baseDir, applied)); resolved != "" {
				return resolved
			}
		}
	}
	if cfg.baseDir == "" {
		return ""
	}
	return resolveTypeScriptModulePath(resolver, path.Join(cfg.baseDir, specifier))
}

func resolveTypeScriptWorkspacePackage(resolver *typeScriptImportResolver, specifier string) string {
	root := typeScriptPackageRoot(specifier)
	pkg, ok := resolver.packages[root]
	if !ok {
		return ""
	}
	if specifier == root {
		return resolveTypeScriptWorkspacePackageEntrypoint(resolver, pkg)
	}
	subpath := strings.TrimPrefix(specifier, root+"/")
	if subpath == specifier || subpath == "" {
		return ""
	}
	for _, candidate := range matchTypeScriptMapping(pkg.exports, "./"+subpath) {
		if resolved := resolveTypeScriptModulePath(resolver, path.Join(pkg.dir, candidate)); resolved != "" {
			return resolved
		}
	}
	for _, candidate := range []string{
		path.Join(pkg.dir, subpath),
		path.Join(pkg.dir, "src", subpath),
	} {
		if resolved := resolveTypeScriptModulePath(resolver, candidate); resolved != "" {
			return resolved
		}
	}
	return ""
}

func resolveTypeScriptWorkspacePackageEntrypoint(resolver *typeScriptImportResolver, pkg typeScriptWorkspacePackage) string {
	for _, candidate := range matchTypeScriptMapping(pkg.exports, ".") {
		if resolved := resolveTypeScriptModulePath(resolver, path.Join(pkg.dir, candidate)); resolved != "" {
			return resolved
		}
	}
	for _, candidate := range []string{pkg.types, pkg.source, pkg.module, pkg.main} {
		if strings.TrimSpace(candidate) == "" {
			continue
		}
		if resolved := resolveTypeScriptModulePath(resolver, path.Join(pkg.dir, candidate)); resolved != "" {
			return resolved
		}
	}
	for _, candidate := range []string{
		path.Join(pkg.dir, "index"),
		path.Join(pkg.dir, "src", "index"),
	} {
		if resolved := resolveTypeScriptModulePath(resolver, candidate); resolved != "" {
			return resolved
		}
	}
	return ""
}

func typeScriptPackageForModule(resolver *typeScriptImportResolver, module string) (typeScriptWorkspacePackage, bool) {
	node := resolver.graph.modules[module]
	if node == nil {
		return typeScriptWorkspacePackage{}, false
	}
	file := normalizeTypeScriptPath(node.file)
	best := typeScriptWorkspacePackage{}
	bestLen := -1
	for _, pkg := range resolver.packages {
		if !typeScriptPathContains(pkg.dir, file) {
			continue
		}
		if len(pkg.dir) > bestLen {
			best = pkg
			bestLen = len(pkg.dir)
		}
	}
	if bestLen < 0 {
		return typeScriptWorkspacePackage{}, false
	}
	return best, true
}

func resolveTypeScriptModulePath(resolver *typeScriptImportResolver, rel string) string {
	rel = typeScriptModuleKey(normalizeTypeScriptPath(rel))
	for _, candidate := range []string{rel, rel + "/index"} {
		if _, ok := resolver.graph.modules[candidate]; ok {
			return candidate
		}
	}
	return ""
}

func typeScriptConfigForModule(resolver *typeScriptImportResolver, module string) *typeScriptGraphConfig {
	node := resolver.graph.modules[module]
	if node == nil {
		return nil
	}
	dir := normalizeTypeScriptPath(path.Dir(node.file))
	for idx := range resolver.configs {
		cfg := &resolver.configs[idx]
		if typeScriptPathContains(cfg.dir, dir) {
			return cfg
		}
	}
	return nil
}
