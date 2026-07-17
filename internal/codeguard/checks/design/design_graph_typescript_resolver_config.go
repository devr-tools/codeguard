package design

import (
	"path"
	"sort"
	"strings"
)

func effectiveTypeScriptConfig(resolver *typeScriptImportResolver, rel string, cache map[string]typeScriptGraphConfig, seen map[string]bool) (typeScriptGraphConfig, bool) {
	if cfg, ok := cache[rel]; ok {
		return cfg, true
	}
	if seen[rel] {
		return typeScriptGraphConfig{}, false
	}
	doc, ok := resolver.tsconfigs[rel]
	if !ok {
		return typeScriptGraphConfig{}, false
	}
	seen[rel] = true
	dir := normalizeTypeScriptPath(path.Dir(rel))
	effective := typeScriptCompilerOptions{}
	if parentRel, ok := resolveTypeScriptConfigExtends(resolver, dir, doc.Extends); ok {
		if parent, ok := effectiveTypeScriptConfig(resolver, parentRel, cache, seen); ok {
			effective.BaseURL = parent.baseDir
			effective.Paths = aliasesToPathMap(parent.paths)
		}
	}
	effective = mergeTypeScriptCompilerOptions(dir, effective, doc.CompilerOptions)
	cfg := typeScriptGraphConfig{
		dir:     dir,
		baseDir: effective.BaseURL,
		paths:   sortedTypeScriptAliases(effective.Paths),
	}
	if cfg.baseDir == "" {
		cfg.baseDir = dir
	}
	cache[rel] = cfg
	return cfg, true
}

func aliasesToPathMap(aliases []typeScriptPathAlias) map[string][]string {
	if len(aliases) == 0 {
		return nil
	}
	out := make(map[string][]string, len(aliases))
	for _, alias := range aliases {
		out[alias.pattern] = append([]string(nil), alias.targets...)
	}
	return out
}

func mergeTypeScriptCompilerOptions(dir string, base typeScriptCompilerOptions, override typeScriptCompilerOptions) typeScriptCompilerOptions {
	merged := typeScriptCompilerOptions{
		BaseURL: base.BaseURL,
		Paths:   cloneTypeScriptPathMap(base.Paths),
	}
	if strings.TrimSpace(override.BaseURL) != "" {
		merged.BaseURL = normalizeTypeScriptPath(path.Join(dir, override.BaseURL))
	}
	if len(override.Paths) > 0 {
		merged.Paths = cloneTypeScriptPathMap(override.Paths)
	}
	if merged.BaseURL == "" {
		merged.BaseURL = dir
	}
	return merged
}

func cloneTypeScriptPathMap(paths map[string][]string) map[string][]string {
	if len(paths) == 0 {
		return nil
	}
	out := make(map[string][]string, len(paths))
	for key, values := range paths {
		out[key] = append([]string(nil), values...)
	}
	return out
}

func resolveTypeScriptConfigExtends(resolver *typeScriptImportResolver, dir string, value string) (string, bool) {
	value = strings.TrimSpace(value)
	if value == "" {
		return "", false
	}
	candidates := []string{normalizeTypeScriptPath(path.Join(dir, value))}
	if !strings.HasPrefix(value, ".") && !strings.HasPrefix(value, "/") {
		candidates = append(candidates, workspaceTypeScriptConfigCandidates(resolver, value)...)
	}
	if !strings.HasSuffix(value, ".json") {
		candidates = append(candidates, normalizeTypeScriptPath(path.Join(dir, value+".json")))
		if !strings.HasPrefix(value, ".") && !strings.HasPrefix(value, "/") {
			candidates = append(candidates, workspaceTypeScriptConfigCandidates(resolver, value+".json")...)
		}
	}
	candidates = append(candidates, normalizeTypeScriptPath(path.Join(dir, value, "tsconfig.json")))
	if !strings.HasPrefix(value, ".") && !strings.HasPrefix(value, "/") {
		candidates = append(candidates, workspaceTypeScriptConfigCandidates(resolver, path.Join(value, "tsconfig.json"))...)
	}
	for _, candidate := range candidates {
		if _, ok := resolver.tsconfigs[candidate]; ok {
			return candidate, true
		}
	}
	return "", false
}

func workspaceTypeScriptConfigCandidates(resolver *typeScriptImportResolver, specifier string) []string {
	root := typeScriptPackageRoot(specifier)
	pkg, ok := resolver.packages[root]
	if !ok {
		return nil
	}
	subpath := strings.TrimPrefix(strings.TrimPrefix(specifier, root), "/")
	if subpath == "" {
		return []string{normalizeTypeScriptPath(path.Join(pkg.dir, "tsconfig.json"))}
	}
	return []string{normalizeTypeScriptPath(path.Join(pkg.dir, subpath))}
}

func sortedTypeScriptAliases(paths map[string][]string) []typeScriptPathAlias {
	aliases := make([]typeScriptPathAlias, 0, len(paths))
	for pattern, targets := range paths {
		cleanTargets := make([]string, 0, len(targets))
		for _, target := range targets {
			target = strings.TrimSpace(target)
			if target == "" {
				continue
			}
			cleanTargets = append(cleanTargets, target)
		}
		if len(cleanTargets) == 0 {
			continue
		}
		aliases = append(aliases, typeScriptPathAlias{
			pattern: strings.TrimSpace(pattern),
			targets: cleanTargets,
		})
	}
	sort.Slice(aliases, func(i, j int) bool {
		if len(aliases[i].pattern) != len(aliases[j].pattern) {
			return len(aliases[i].pattern) > len(aliases[j].pattern)
		}
		return aliases[i].pattern < aliases[j].pattern
	})
	return aliases
}
