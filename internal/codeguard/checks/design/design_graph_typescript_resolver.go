package design

import (
	"encoding/json"
	"path"
	"sort"
	"strings"
)

type typeScriptImportResolver struct {
	graph           *moduleGraph
	configs         []typeScriptGraphConfig
	packages        map[string]typeScriptWorkspacePackage
	tsconfigs       map[string]typeScriptConfigDocument
	tsconfigPrimary map[string]string
}

type typeScriptGraphConfig struct {
	dir     string
	baseDir string
	paths   []typeScriptPathAlias
}

type typeScriptPathAlias struct {
	pattern string
	targets []string
}

type typeScriptWorkspacePackage struct {
	name    string
	dir     string
	main    string
	module  string
	source  string
	types   string
	exports map[string][]string
	imports map[string][]string
}

type typeScriptPackageManifest struct {
	Name    string          `json:"name"`
	Main    string          `json:"main"`
	Module  string          `json:"module"`
	Source  string          `json:"source"`
	Types   string          `json:"types"`
	Exports json.RawMessage `json:"exports"`
	Imports json.RawMessage `json:"imports"`
}

type typeScriptConfigDocument struct {
	Extends         string                    `json:"extends"`
	CompilerOptions typeScriptCompilerOptions `json:"compilerOptions"`
}

type typeScriptCompilerOptions struct {
	BaseURL string              `json:"baseUrl"`
	Paths   map[string][]string `json:"paths"`
}

func newTypeScriptImportResolver(graph *moduleGraph) *typeScriptImportResolver {
	return &typeScriptImportResolver{
		graph:           graph,
		packages:        make(map[string]typeScriptWorkspacePackage),
		tsconfigs:       make(map[string]typeScriptConfigDocument),
		tsconfigPrimary: make(map[string]string),
	}
}

func isTypeScriptResolverMetadataFile(rel string) bool {
	base := path.Base(rel)
	if base == "package.json" {
		return true
	}
	return strings.HasSuffix(base, ".json")
}

func (resolver *typeScriptImportResolver) indexMetadata(rel string, data []byte) {
	base := path.Base(rel)
	switch {
	case base == "package.json":
		resolver.indexPackageManifest(path.Dir(rel), data)
	case strings.HasSuffix(base, ".json"):
		resolver.indexTSConfig(rel, data)
	}
}

func (resolver *typeScriptImportResolver) indexPackageManifest(dir string, data []byte) {
	var manifest typeScriptPackageManifest
	if err := json.Unmarshal(data, &manifest); err != nil {
		return
	}
	name := strings.TrimSpace(manifest.Name)
	if name == "" {
		return
	}
	pkg := typeScriptWorkspacePackage{
		name:    name,
		dir:     normalizeTypeScriptPath(dir),
		main:    strings.TrimSpace(manifest.Main),
		module:  strings.TrimSpace(manifest.Module),
		source:  strings.TrimSpace(manifest.Source),
		types:   strings.TrimSpace(manifest.Types),
		exports: parseTypeScriptPackageExports(manifest.Exports),
		imports: parseTypeScriptPackageImports(manifest.Imports),
	}
	resolver.packages[name] = pkg
}

func (resolver *typeScriptImportResolver) indexTSConfig(rel string, data []byte) {
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

func (resolver *typeScriptImportResolver) finalizeConfigs() {
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
		cfg, ok := resolver.effectiveTSConfig(resolver.tsconfigPrimary[dir], cache, make(map[string]bool))
		if !ok {
			continue
		}
		resolver.configs = append(resolver.configs, cfg)
	}
}

func (resolver *typeScriptImportResolver) effectiveTSConfig(rel string, cache map[string]typeScriptGraphConfig, seen map[string]bool) (typeScriptGraphConfig, bool) {
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
	if parentRel, ok := resolver.resolveTSConfigExtends(dir, doc.Extends); ok {
		if parent, ok := resolver.effectiveTSConfig(parentRel, cache, seen); ok {
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

func (resolver *typeScriptImportResolver) resolveTSConfigExtends(dir string, value string) (string, bool) {
	value = strings.TrimSpace(value)
	if value == "" {
		return "", false
	}
	candidates := make([]string, 0, 3)
	if strings.HasPrefix(value, ".") || strings.HasPrefix(value, "/") {
		candidates = append(candidates, normalizeTypeScriptPath(path.Join(dir, value)))
	} else {
		candidates = append(candidates, normalizeTypeScriptPath(path.Join(dir, value)))
		candidates = append(candidates, resolver.workspaceTSConfigCandidates(value)...)
	}
	if !strings.HasSuffix(value, ".json") {
		candidates = append(candidates, normalizeTypeScriptPath(path.Join(dir, value+".json")))
		if !strings.HasPrefix(value, ".") && !strings.HasPrefix(value, "/") {
			for _, candidate := range resolver.workspaceTSConfigCandidates(value + ".json") {
				candidates = append(candidates, candidate)
			}
		}
	}
	candidates = append(candidates, normalizeTypeScriptPath(path.Join(dir, value, "tsconfig.json")))
	if !strings.HasPrefix(value, ".") && !strings.HasPrefix(value, "/") {
		for _, candidate := range resolver.workspaceTSConfigCandidates(path.Join(value, "tsconfig.json")) {
			candidates = append(candidates, candidate)
		}
	}
	for _, candidate := range candidates {
		if _, ok := resolver.tsconfigs[candidate]; ok {
			return candidate, true
		}
	}
	return "", false
}

func (resolver *typeScriptImportResolver) workspaceTSConfigCandidates(specifier string) []string {
	root := typeScriptPackageRoot(specifier)
	pkg, ok := resolver.packages[root]
	if !ok {
		return nil
	}
	subpath := strings.TrimPrefix(specifier, root)
	subpath = strings.TrimPrefix(subpath, "/")
	candidates := make([]string, 0, 3)
	if subpath == "" {
		candidates = append(candidates, normalizeTypeScriptPath(path.Join(pkg.dir, "tsconfig.json")))
	} else {
		candidates = append(candidates, normalizeTypeScriptPath(path.Join(pkg.dir, subpath)))
	}
	return candidates
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

func (resolver *typeScriptImportResolver) resolve(fromModule string, specifier string) string {
	if strings.HasPrefix(specifier, "./") || strings.HasPrefix(specifier, "../") || specifier == "." || specifier == ".." {
		return resolver.resolveRelative(fromModule, specifier)
	}
	if resolved := resolver.resolvePackageImport(fromModule, specifier); resolved != "" {
		return resolved
	}
	if resolved := resolver.resolveTSConfigAlias(fromModule, specifier); resolved != "" {
		return resolved
	}
	if resolved := resolver.resolveWorkspacePackage(specifier); resolved != "" {
		return resolved
	}
	return ""
}

func (resolver *typeScriptImportResolver) resolveRelative(fromModule string, specifier string) string {
	joined := path.Clean(path.Join(path.Dir(fromModule), specifier))
	return resolver.resolveModulePath(joined)
}

func (resolver *typeScriptImportResolver) resolvePackageImport(fromModule string, specifier string) string {
	if !strings.HasPrefix(specifier, "#") {
		return ""
	}
	pkg, ok := resolver.packageForModule(fromModule)
	if !ok {
		return ""
	}
	for _, candidate := range matchTypeScriptMapping(pkg.imports, specifier) {
		if resolved := resolver.resolveModulePath(path.Join(pkg.dir, candidate)); resolved != "" {
			return resolved
		}
	}
	return ""
}

func (resolver *typeScriptImportResolver) resolveTSConfigAlias(fromModule string, specifier string) string {
	cfg := resolver.configForModule(fromModule)
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
			if resolved := resolver.resolveModulePath(path.Join(cfg.baseDir, applied)); resolved != "" {
				return resolved
			}
		}
	}
	if cfg.baseDir == "" {
		return ""
	}
	return resolver.resolveModulePath(path.Join(cfg.baseDir, specifier))
}

func (resolver *typeScriptImportResolver) resolveWorkspacePackage(specifier string) string {
	root := typeScriptPackageRoot(specifier)
	pkg, ok := resolver.packages[root]
	if !ok {
		return ""
	}
	if specifier == root {
		return resolver.resolveWorkspacePackageEntrypoint(pkg)
	}
	subpath := strings.TrimPrefix(specifier, root+"/")
	if subpath == specifier || subpath == "" {
		return ""
	}
	for _, candidate := range matchTypeScriptMapping(pkg.exports, "./"+subpath) {
		if resolved := resolver.resolveModulePath(path.Join(pkg.dir, candidate)); resolved != "" {
			return resolved
		}
	}
	for _, candidate := range []string{
		path.Join(pkg.dir, subpath),
		path.Join(pkg.dir, "src", subpath),
	} {
		if resolved := resolver.resolveModulePath(candidate); resolved != "" {
			return resolved
		}
	}
	return ""
}

func (resolver *typeScriptImportResolver) resolveWorkspacePackageEntrypoint(pkg typeScriptWorkspacePackage) string {
	for _, candidate := range matchTypeScriptMapping(pkg.exports, ".") {
		if resolved := resolver.resolveModulePath(path.Join(pkg.dir, candidate)); resolved != "" {
			return resolved
		}
	}
	for _, candidate := range []string{pkg.types, pkg.source, pkg.module, pkg.main} {
		if strings.TrimSpace(candidate) == "" {
			continue
		}
		if resolved := resolver.resolveModulePath(path.Join(pkg.dir, candidate)); resolved != "" {
			return resolved
		}
	}
	for _, candidate := range []string{
		path.Join(pkg.dir, "index"),
		path.Join(pkg.dir, "src", "index"),
	} {
		if resolved := resolver.resolveModulePath(candidate); resolved != "" {
			return resolved
		}
	}
	return ""
}

func (resolver *typeScriptImportResolver) packageForModule(module string) (typeScriptWorkspacePackage, bool) {
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

func (resolver *typeScriptImportResolver) resolveModulePath(rel string) string {
	rel = normalizeTypeScriptPath(rel)
	rel = typeScriptModuleKey(rel)
	for _, candidate := range []string{rel, rel + "/index"} {
		if _, ok := resolver.graph.modules[candidate]; ok {
			return candidate
		}
	}
	return ""
}

func (resolver *typeScriptImportResolver) configForModule(module string) *typeScriptGraphConfig {
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

func typeScriptPathContains(parent string, child string) bool {
	parent = normalizeTypeScriptPath(parent)
	child = normalizeTypeScriptPath(child)
	if parent == "." {
		return true
	}
	return child == parent || strings.HasPrefix(child, parent+"/")
}

func normalizeTypeScriptPath(value string) string {
	value = strings.TrimSpace(strings.ReplaceAll(value, "\\", "/"))
	if value == "" {
		return "."
	}
	value = path.Clean(value)
	value = strings.TrimPrefix(value, "./")
	if value == "" {
		return "."
	}
	return value
}

func typeScriptPackageRoot(specifier string) string {
	if strings.HasPrefix(specifier, "@") {
		parts := strings.Split(specifier, "/")
		if len(parts) >= 2 {
			return parts[0] + "/" + parts[1]
		}
	}
	return firstTypeScriptSegment(specifier)
}

func firstTypeScriptSegment(specifier string) string {
	specifier = strings.TrimSpace(specifier)
	if specifier == "" {
		return ""
	}
	if cut := strings.IndexByte(specifier, '/'); cut >= 0 {
		return specifier[:cut]
	}
	return specifier
}

func matchTypeScriptAlias(pattern string, specifier string) (string, bool) {
	if !strings.Contains(pattern, "*") {
		return "", pattern == specifier
	}
	parts := strings.SplitN(pattern, "*", 2)
	if !strings.HasPrefix(specifier, parts[0]) || !strings.HasSuffix(specifier, parts[1]) {
		return "", false
	}
	return specifier[len(parts[0]) : len(specifier)-len(parts[1])], true
}

func applyTypeScriptAliasTarget(target string, wildcard string) string {
	if !strings.Contains(target, "*") {
		return target
	}
	return strings.Replace(target, "*", wildcard, 1)
}

func matchTypeScriptMapping(mappings map[string][]string, specifier string) []string {
	if len(mappings) == 0 {
		return nil
	}
	patterns := make([]string, 0, len(mappings))
	for pattern := range mappings {
		patterns = append(patterns, pattern)
	}
	sort.Slice(patterns, func(i, j int) bool {
		if len(patterns[i]) != len(patterns[j]) {
			return len(patterns[i]) > len(patterns[j])
		}
		return patterns[i] < patterns[j]
	})
	for _, pattern := range patterns {
		wildcard, ok := matchTypeScriptAlias(pattern, specifier)
		if !ok {
			continue
		}
		values := make([]string, 0, len(mappings[pattern]))
		for _, target := range mappings[pattern] {
			values = append(values, applyTypeScriptAliasTarget(target, wildcard))
		}
		return values
	}
	return nil
}

func parseTypeScriptPackageExports(raw json.RawMessage) map[string][]string {
	return parseTypeScriptPackageMappings(raw, true)
}

func parseTypeScriptPackageImports(raw json.RawMessage) map[string][]string {
	return parseTypeScriptPackageMappings(raw, false)
}

func parseTypeScriptPackageMappings(raw json.RawMessage, isExports bool) map[string][]string {
	if len(raw) == 0 {
		return nil
	}
	var node any
	if err := json.Unmarshal(raw, &node); err != nil {
		return nil
	}
	mappings := make(map[string][]string)
	switch value := node.(type) {
	case string:
		mappings["."] = append(mappings["."], value)
	case map[string]any:
		for key, child := range value {
			switch {
			case isExports && key == ".":
				mappings["."] = append(mappings["."], collectTypeScriptExportTargets(child)...)
			case isExports && strings.HasPrefix(key, "./"):
				mappings[key] = append(mappings[key], collectTypeScriptExportTargets(child)...)
			case !isExports && strings.HasPrefix(key, "#"):
				mappings[key] = append(mappings[key], collectTypeScriptExportTargets(child)...)
			default:
				mappings["."] = append(mappings["."], collectTypeScriptExportTargets(child)...)
			}
		}
	}
	for key, values := range mappings {
		mappings[key] = uniqueNonEmptyStrings(values)
		if len(mappings[key]) == 0 {
			delete(mappings, key)
		}
	}
	return mappings
}

func collectTypeScriptExportTargets(node any) []string {
	switch value := node.(type) {
	case string:
		return []string{value}
	case []any:
		out := make([]string, 0, len(value))
		for _, item := range value {
			out = append(out, collectTypeScriptExportTargets(item)...)
		}
		return out
	case map[string]any:
		out := make([]string, 0, len(value))
		for _, key := range orderedTypeScriptConditionKeys(value) {
			child := value[key]
			out = append(out, collectTypeScriptExportTargets(child)...)
		}
		return out
	default:
		return nil
	}
}

func orderedTypeScriptConditionKeys(values map[string]any) []string {
	preferred := []string{
		"types", "source", "import", "module", "browser", "development",
		"production", "node", "default", "require",
	}
	keys := make([]string, 0, len(values))
	seen := make(map[string]struct{}, len(values))
	for _, key := range preferred {
		if _, ok := values[key]; ok {
			keys = append(keys, key)
			seen[key] = struct{}{}
		}
	}
	extra := make([]string, 0, len(values))
	for key := range values {
		if _, ok := seen[key]; ok {
			continue
		}
		extra = append(extra, key)
	}
	sort.Strings(extra)
	return append(keys, extra...)
}

func uniqueNonEmptyStrings(values []string) []string {
	out := make([]string, 0, len(values))
	seen := make(map[string]struct{}, len(values))
	for _, value := range values {
		value = strings.TrimSpace(strings.TrimPrefix(value, "./"))
		if value == "" {
			continue
		}
		if _, ok := seen[value]; ok {
			continue
		}
		seen[value] = struct{}{}
		out = append(out, value)
	}
	return out
}

func normalizeJSONC(data []byte) ([]byte, bool) {
	withoutComments, ok := stripJSONCComments(string(data))
	if !ok {
		return nil, false
	}
	return []byte(stripJSONCTrailingCommas(withoutComments)), true
}

func stripJSONCComments(source string) (string, bool) {
	var b strings.Builder
	b.Grow(len(source))
	inString := false
	escaped := false
	for idx := 0; idx < len(source); idx++ {
		ch := source[idx]
		if inString {
			b.WriteByte(ch)
			if escaped {
				escaped = false
				continue
			}
			switch ch {
			case '\\':
				escaped = true
			case '"':
				inString = false
			}
			continue
		}
		if ch == '"' {
			inString = true
			b.WriteByte(ch)
			continue
		}
		if ch == '/' && idx+1 < len(source) {
			switch source[idx+1] {
			case '/':
				for idx+1 < len(source) && source[idx+1] != '\n' {
					idx++
				}
				continue
			case '*':
				idx += 2
				for idx < len(source) {
					if idx+1 < len(source) && source[idx] == '*' && source[idx+1] == '/' {
						idx++
						break
					}
					if source[idx] == '\n' {
						b.WriteByte('\n')
					}
					idx++
				}
				if idx >= len(source) {
					return "", false
				}
				continue
			}
		}
		b.WriteByte(ch)
	}
	return b.String(), !inString
}

func stripJSONCTrailingCommas(source string) string {
	var b strings.Builder
	b.Grow(len(source))
	inString := false
	escaped := false
	for idx := 0; idx < len(source); idx++ {
		ch := source[idx]
		if inString {
			b.WriteByte(ch)
			if escaped {
				escaped = false
				continue
			}
			switch ch {
			case '\\':
				escaped = true
			case '"':
				inString = false
			}
			continue
		}
		if ch == '"' {
			inString = true
			b.WriteByte(ch)
			continue
		}
		if ch == ',' {
			next := idx + 1
			for next < len(source) && isJSONWhitespace(source[next]) {
				next++
			}
			if next < len(source) && (source[next] == '}' || source[next] == ']') {
				continue
			}
		}
		b.WriteByte(ch)
	}
	return b.String()
}

func isJSONWhitespace(ch byte) bool {
	switch ch {
	case ' ', '\t', '\r', '\n':
		return true
	default:
		return false
	}
}
