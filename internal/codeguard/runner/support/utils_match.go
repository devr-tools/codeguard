package support

import (
	"path/filepath"
	"regexp"
	"strings"
	"sync"
)

var patternCache sync.Map // map[string]*regexp.Regexp

func IsInternalOrCmdFile(path string) bool {
	return isInternalFile(path) || IsCmdFile(path)
}

func isInternalFile(path string) bool {
	return strings.HasPrefix(filepath.ToSlash(path), "internal/")
}

func IsCmdFile(path string) bool {
	return strings.HasPrefix(filepath.ToSlash(path), "cmd/")
}

func IsPublicPackageFile(path string) bool {
	normalized := filepath.ToSlash(path)
	return strings.HasPrefix(normalized, "pkg/")
}

func IsSDKFacadeFile(path string) bool {
	normalized := filepath.ToSlash(path)
	return strings.HasPrefix(normalized, "pkg/codeguard/")
}

// ShouldExclude reports whether a target-root-relative path is skipped by the
// built-in defaults or by the user's exclude list.
func ShouldExclude(rel string, excludes []string) bool {
	return shouldExclude(rel, excludes, FileWalkOptions{})
}

func shouldExclude(rel string, excludes []string, opts FileWalkOptions) bool {
	if matchesReinclude(rel, excludes) {
		return false
	}
	if isDefaultDependencyOrBuildPath(rel, opts.ScanVendoredSource) {
		return true
	}
	return matchesUserExclude(rel, excludes)
}

// matchesUserExclude reports whether rel matches any (non-negated) configured
// exclude glob. It deliberately ignores the built-in defaults so that the walk
// can apply the two sets at different scopes: the defaults are meaningful only
// relative to the directory actually being scanned, whereas user globs are
// written against both the target-relative and the repository-relative form of
// a path. See walkFilesBounded.
func matchesUserExclude(rel string, excludes []string) bool {
	for _, pattern := range excludes {
		if strings.HasPrefix(strings.TrimSpace(pattern), "!") {
			continue
		}
		if MatchPattern(pattern, rel) {
			return true
		}
	}
	return false
}

// matchesReinclude reports whether rel matches a negated exclude entry
// ("!some/glob/**").
//
// This is the escape hatch for the built-in default exclusions below. Those
// defaults are not user-editable, so a repository whose real source happens to
// live under one of the excluded names would otherwise have no way to get it
// scanned. A re-include wins over everything, defaults included: it is an
// explicit statement by the repository owner, which is the only signal strong
// enough to override a name-based heuristic.
func matchesReinclude(rel string, excludes []string) bool {
	for _, pattern := range excludes {
		trimmed := strings.TrimSpace(pattern)
		if !strings.HasPrefix(trimmed, "!") {
			continue
		}
		if MatchPattern(strings.TrimPrefix(trimmed, "!"), rel) {
			return true
		}
	}
	return false
}

// reincludeMayCoverSubtree reports whether any "!" exclude entry could match a
// path beneath dir.
//
// The walk prunes an excluded directory wholesale, which is what keeps the
// exclusions cheap, but that would make a re-include of
// "!packages/ui/dist/**" unreachable: packages/ui/dist is skipped before the
// walk ever descends. This is the same problem gitignore negation has. The test
// is intentionally loose - it only has to decide whether descending is
// worthwhile; each file inside is still matched individually.
func reincludeMayCoverSubtree(dir string, excludes []string) bool {
	prefix := filepath.ToSlash(strings.TrimSpace(dir))
	if prefix == "" || prefix == "." {
		return false
	}
	prefix += "/"
	for _, pattern := range excludes {
		trimmed := strings.TrimSpace(pattern)
		if !strings.HasPrefix(trimmed, "!") {
			continue
		}
		glob := filepath.ToSlash(strings.TrimPrefix(trimmed, "!"))
		// The pattern reaches into this directory ...
		if strings.HasPrefix(glob, prefix) {
			return true
		}
		// ... or its literal prefix stops at or above this directory, in which
		// case its wildcards may still match inside.
		if static := staticGlobPrefix(glob); static == "" || strings.HasPrefix(prefix, static) {
			return true
		}
	}
	return false
}

// staticGlobPrefix returns the leading directory portion of glob that contains
// no wildcard, or "" when the glob starts with one.
func staticGlobPrefix(glob string) string {
	wildcard := strings.IndexAny(glob, "*?[")
	if wildcard < 0 {
		return glob
	}
	if cut := strings.LastIndex(glob[:wildcard], "/"); cut >= 0 {
		return glob[:cut+1]
	}
	return ""
}

// Built-in scan exclusions are deliberately split by how certain the name is,
// because over-excluding is as harmful as under-excluding: a directory skipped
// by name disappears from the scan with no signal to the user, so a real
// vulnerability in it is reported as a pass.
//
//  1. unambiguousGeneratedDirs - directory names that are never hand-written
//     source in any ecosystem we know of (dependency trees, tool caches,
//     framework build output). Matched by path *segment*, so they are skipped at
//     any depth. Depth independence is the point: these used to be anchored
//     globs like "dist/**", which matched dist/app.js but not
//     services/app/dist/app.js, so every monorepo scanned its own build output.
//  2. generatedFileSuffixes - individual generated files (minified bundles and
//     source maps). They are text, so the binary guard does not catch them, yet
//     they are machine-generated megabyte blobs whose findings are never
//     actionable.
//  3. buildOutputDirMarkers (utils_build_output.go) - names that are build
//     output in most repositories but legitimate source in some: build, out,
//     bin, obj, target, coverage. These are NOT excluded on the name alone.
//     A sibling manifest must corroborate that the directory is that build
//     system's output directory.
//
// "vendor" keeps its existing opt-out (config scan_vendored_source) because
// vendored *source* is real code that some repositories do want reviewed.
var unambiguousGeneratedDirs = map[string]struct{}{
	// Version control metadata and codeguard's own state directory.
	".git":       {},
	".hg":        {},
	".svn":       {},
	".codeguard": {},

	// Dependency trees. Nothing here is written by the repository.
	"node_modules":     {},
	"bower_components": {},
	"site-packages":    {},
	".venv":            {},
	"venv":             {},
	".pub-cache":       {},
	"Pods":             {},
	"Carthage":         {},

	// JS/TS framework, bundler and deploy output. ".next" alone accounted for
	// 97% of the findings on one real Next.js monorepo and 78% on another.
	".next":         {},
	".nuxt":         {},
	".svelte-kit":   {},
	".angular":      {},
	".astro":        {},
	".docusaurus":   {},
	".output":       {},
	".vercel":       {},
	".netlify":      {},
	".turbo":        {},
	".parcel-cache": {},
	".vite":         {},
	".webpack":      {},
	".nyc_output":   {},

	// Tool caches. ".cache" is the common name that ".gocache"/".gomodcache"
	// were standing in for: a Go repo with a .build/go-cache tree produced 566
	// security findings against compiled build-cache objects.
	".cache":             {},
	".gocache":           {},
	".gomodcache":        {},
	".mypy_cache":        {},
	".pytest_cache":      {},
	".ruff_cache":        {},
	".tox":               {},
	".nox":               {},
	".gradle":            {},
	".terraform":         {},
	".build":             {}, // SwiftPM, and the go-cache tree noted above
	".dart_tool":         {},
	".sass-cache":        {},
	".eggs":              {},
	".ipynb_checkpoints": {},

	// Report output whose directory name has no other meaning.
	"htmlcov": {},

	// Infrastructure synth output.
	"cdk.out": {},

	// Python bytecode.
	"__pycache__": {},

	// "dist" is treated as unconditional rather than corroborated: it was
	// already a built-in root-anchored default, so the project has already
	// accepted the name as build output, and a *source* directory named dist is
	// essentially unheard of. Making it depth-independent is the single
	// highest-impact fix here. A repository that really does keep source in a
	// dist/ directory re-includes it with exclude: ["!path/to/dist/**"].
	"dist": {},
}

// unambiguousGeneratedDirSuffixes are directory-name suffixes (rather than
// exact names) that identify generated trees.
var unambiguousGeneratedDirSuffixes = []string{
	".egg-info", // setuptools metadata
}

// generatedFileSuffixes are file names produced by a bundler or compiler. Only
// compound suffixes are listed: a bare ".map" could plausibly be a data file,
// whereas ".js.map" is a source map and nothing else.
var generatedFileSuffixes = []string{
	".min.js",
	".min.mjs",
	".min.cjs",
	".min.css",
	".bundle.js",
	".bundle.mjs",
	".bundle.css",
	".js.map",
	".mjs.map",
	".cjs.map",
	".css.map",
	".ts.map",
}

func isDefaultDependencyOrBuildPath(rel string, scanVendoredSource bool) bool {
	normalized := filepath.ToSlash(rel)
	for _, segment := range strings.Split(normalized, "/") {
		if _, generated := unambiguousGeneratedDirs[segment]; generated {
			return true
		}
		for _, suffix := range unambiguousGeneratedDirSuffixes {
			if len(segment) > len(suffix) && strings.HasSuffix(segment, suffix) {
				return true
			}
		}
		if segment == "vendor" && !scanVendoredSource {
			return true
		}
	}
	return isGeneratedFileName(normalized)
}

func isGeneratedFileName(normalized string) bool {
	base := normalized
	if idx := strings.LastIndex(base, "/"); idx >= 0 {
		base = base[idx+1:]
	}
	lower := strings.ToLower(base)
	for _, suffix := range generatedFileSuffixes {
		if len(lower) > len(suffix) && strings.HasSuffix(lower, suffix) {
			return true
		}
	}
	return false
}

func MatchPattern(pattern string, value string) bool {
	pattern = filepath.ToSlash(strings.TrimSpace(pattern))
	value = filepath.ToSlash(strings.TrimSpace(value))
	if pattern == "" {
		return false
	}
	re, ok := compilePattern(pattern)
	if !ok {
		return false
	}
	return re.MatchString(value)
}

func compilePattern(pattern string) (*regexp.Regexp, bool) {
	if cached, ok := patternCache.Load(pattern); ok {
		re, _ := cached.(*regexp.Regexp)
		return re, re != nil
	}
	replacer := strings.NewReplacer(
		`\`, `\\`,
		`.`, `\.`,
		`+`, `\+`,
		`(`, `\(`,
		`)`, `\)`,
		`[`, `\[`,
		`]`, `\]`,
		`{`, `\{`,
		`}`, `\}`,
		`^`, `\^`,
		`$`, `\$`,
	)
	expr := replacer.Replace(pattern)
	expr = strings.ReplaceAll(expr, "**", "§§DOUBLESTAR§§")
	expr = strings.ReplaceAll(expr, "*", `[^/]*`)
	expr = strings.ReplaceAll(expr, "§§DOUBLESTAR§§", `.*`)
	expr = strings.ReplaceAll(expr, "?", `[^/]`)
	re, err := regexp.Compile("^" + expr + "$")
	if err != nil {
		patternCache.Store(pattern, (*regexp.Regexp)(nil))
		return nil, false
	}
	patternCache.Store(pattern, re)
	return re, true
}
