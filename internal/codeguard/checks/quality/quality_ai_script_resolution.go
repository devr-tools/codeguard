package quality

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"

	"github.com/devr-tools/codeguard/internal/codeguard/checks/support"
	"github.com/devr-tools/codeguard/internal/codeguard/core"
)

// scriptTSConfigPathMap is deliberately limited to resolution-relevant
// tsconfig fields. The check only needs to establish whether an import is
// plausible, not type-check the program.
type scriptTSConfigPathMap struct {
	dir     string
	baseURL string
	paths   map[string][]string
}

type scriptTSConfigDocument struct {
	CompilerOptions struct {
		BaseURL string              `json:"baseUrl"`
		Paths   map[string][]string `json:"paths"`
	} `json:"compilerOptions"`
}

func newScriptImportCatalog(env support.Context, target core.TargetConfig, rootManifest packageManifest) scriptImportCatalog {
	catalog := scriptImportCatalog{
		deps:             packageManifestDeps(rootManifest),
		workspacePackage: map[string]struct{}{},
		packageManifests: map[string]packageManifest{},
		lockPackages:     readPNPMLockPackages(target.Path),
	}
	if _, ok := readPackageManifest(target.Path); ok {
		catalog.hasManifest = true
		catalog.packageManifests["."] = rootManifest
	}
	for _, rel := range listAITargetFiles(env, target, func(rel string) bool {
		base := filepath.Base(rel)
		return base == "package.json" || strings.HasPrefix(base, "tsconfig") && strings.HasSuffix(base, ".json")
	}) {
		data, err := readAITargetFile(env, target, rel)
		if err != nil {
			continue
		}
		switch filepath.Base(rel) {
		case "package.json":
			manifest, ok := parsePackageManifest(data)
			if !ok {
				continue
			}
			catalog.hasManifest = true
			dir := filepath.Clean(filepath.Dir(rel))
			catalog.packageManifests[dir] = manifest
			if name := strings.TrimSpace(manifest.Name); name != "" {
				catalog.workspacePackage[name] = struct{}{}
			}
		default:
			var config scriptTSConfigDocument
			if json.Unmarshal(stripScriptJSONC(data), &config) != nil || len(config.CompilerOptions.Paths) == 0 {
				continue
			}
			dir := filepath.Clean(filepath.Dir(rel))
			baseURL := config.CompilerOptions.BaseURL
			if baseURL == "" {
				baseURL = "."
			}
			catalog.tsconfigPathMaps = append(catalog.tsconfigPathMaps, scriptTSConfigPathMap{dir: dir, baseURL: baseURL, paths: config.CompilerOptions.Paths})
		}
	}
	return catalog
}

// readPNPMLockPackages extracts package roots from pnpm's lockfile without
// depending on a YAML parser. Both the current name@version format and the
// older /name/version format are supported. A lockfile is installation
// evidence, so it is a useful fallback when node_modules is absent in CI.
func readPNPMLockPackages(root string) map[string]struct{} {
	data, err := os.ReadFile(filepath.Join(root, "pnpm-lock.yaml")) //nolint:gosec // fixed filename under scan target
	if err != nil {
		return map[string]struct{}{}
	}
	packages := map[string]struct{}{}
	inPackages := false
	for _, line := range strings.Split(string(data), "\n") {
		if strings.TrimSpace(line) == "" {
			continue
		}
		if !strings.HasPrefix(line, " ") {
			inPackages = strings.TrimSpace(line) == "packages:"
			continue
		}
		if !inPackages || !strings.HasPrefix(line, "  ") || strings.HasPrefix(line, "    ") {
			continue
		}
		key := strings.Trim(strings.TrimSuffix(strings.TrimSpace(line), ":"), "'\"")
		if name := pnpmLockPackageName(key); name != "" {
			packages[name] = struct{}{}
		}
	}
	return packages
}

func pnpmLockPackageName(key string) string {
	key = strings.TrimPrefix(key, "/")
	if key == "" || strings.HasPrefix(key, "#") {
		return ""
	}
	if strings.HasPrefix(key, "@") {
		if slash := strings.Index(key, "/"); slash > 1 {
			if at := strings.Index(key[slash+1:], "@"); at >= 0 {
				return key[:slash+1+at]
			}
			parts := strings.Split(key, "/")
			if len(parts) >= 2 {
				return parts[0] + "/" + parts[1]
			}
		}
		return ""
	}
	if at := strings.Index(key, "@"); at > 0 {
		return key[:at]
	}
	if slash := strings.Index(key, "/"); slash > 0 {
		return key[:slash]
	}
	return ""
}

func (catalog scriptImportCatalog) declaredDependenciesFor(file string) map[string]struct{} {
	dir := filepath.Clean(filepath.Dir(file))
	for {
		if manifest, ok := catalog.packageManifests[dir]; ok {
			return packageManifestDeps(manifest)
		}
		if dir == "." || dir == string(filepath.Separator) {
			break
		}
		next := filepath.Dir(dir)
		if next == dir {
			break
		}
		dir = next
	}
	return nil
}

func (catalog scriptImportCatalog) hasInstalledPackage(root, file, packageName string) bool {
	dir := filepath.Join(root, filepath.Dir(file))
	root = filepath.Clean(root)
	for {
		if packageExists(filepath.Join(dir, "node_modules", filepath.FromSlash(packageName))) ||
			pnpmPackageExists(filepath.Join(dir, "node_modules", ".pnpm"), packageName) {
			return true
		}
		if filepath.Clean(dir) == root {
			return false
		}
		next := filepath.Dir(dir)
		if next == dir {
			return false
		}
		dir = next
	}
}

func packageExists(path string) bool {
	info, err := os.Stat(path) //nolint:gosec // path is constructed under scan target
	return err == nil && info.IsDir()
}

func pnpmPackageExists(store, packageName string) bool {
	entries, err := os.ReadDir(store) //nolint:gosec // path is constructed under scan target
	if err != nil {
		return false
	}
	want := filepath.FromSlash(filepath.Join("node_modules", packageName))
	for _, entry := range entries {
		if packageExists(filepath.Join(store, entry.Name(), want)) {
			return true
		}
	}
	return false
}

func (catalog scriptImportCatalog) matchesTSConfigPath(root, file, specifier string) bool {
	fileDir := filepath.Clean(filepath.Dir(file))
	for _, config := range catalog.tsconfigPathMaps {
		if !pathContains(config.dir, fileDir) {
			continue
		}
		for pattern, targets := range config.paths {
			wildcard, ok := scriptPathPatternMatch(pattern, specifier)
			if !ok {
				continue
			}
			for _, target := range targets {
				candidate := strings.ReplaceAll(target, "*", wildcard)
				if resolveRelativeScriptImport(root, filepath.Join(config.dir, config.baseURL), candidate) {
					return true
				}
			}
		}
	}
	return false
}

func pathContains(parent, child string) bool {
	parent, child = filepath.Clean(parent), filepath.Clean(child)
	return parent == "." || child == parent || strings.HasPrefix(child, parent+string(filepath.Separator))
}

func scriptPathPatternMatch(pattern, specifier string) (string, bool) {
	if !strings.Contains(pattern, "*") {
		return "", pattern == specifier
	}
	parts := strings.SplitN(pattern, "*", 2)
	if !strings.HasPrefix(specifier, parts[0]) || !strings.HasSuffix(specifier, parts[1]) {
		return "", false
	}
	return strings.TrimSuffix(strings.TrimPrefix(specifier, parts[0]), parts[1]), true
}

// stripScriptJSONC preserves string contents while removing the comments and
// trailing commas commonly accepted by tsconfig.json.
func stripScriptJSONC(data []byte) []byte {
	source := string(data)
	var b strings.Builder
	inString, escaped := false, false
	for i := 0; i < len(source); i++ {
		ch := source[i]
		if inString {
			b.WriteByte(ch)
			switch {
			case escaped:
				escaped = false
			case ch == '\\':
				escaped = true
			case ch == '"':
				inString = false
			}
			continue
		}
		if ch == '"' {
			inString = true
			b.WriteByte(ch)
			continue
		}
		if ch == '/' && i+1 < len(source) && source[i+1] == '/' {
			for i+1 < len(source) && source[i+1] != '\n' {
				i++
			}
			continue
		}
		if ch == '/' && i+1 < len(source) && source[i+1] == '*' {
			i += 2
			for i < len(source) && (i+1 >= len(source) || source[i] != '*' || source[i+1] != '/') {
				if source[i] == '\n' {
					b.WriteByte('\n')
				}
				i++
			}
			i++
			continue
		}
		b.WriteByte(ch)
	}
	return []byte(strings.ReplaceAll(strings.ReplaceAll(b.String(), ",\n}", "\n}"), ",\n]", "\n]"))
}
