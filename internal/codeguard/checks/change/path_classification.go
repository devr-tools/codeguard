package change

import (
	"path"
	"path/filepath"
	"strings"

	"github.com/devr-tools/codeguard/internal/codeguard/checks/support"
)

// pathRule assigns category to a normalized, lower-cased path when any of its
// patterns match. Rules are evaluated in table order and the first match wins,
// so more specific categories must come first.
type pathRule struct {
	category     string
	prefixes     []string // path prefixes, e.g. ".github/"
	suffixes     []string // whole-path suffixes, e.g. ".md"
	segments     []string // exact path segments, e.g. "api"
	contains     []string // substrings anywhere in the path, e.g. "/repo"
	baseNames    []string // exact base names, e.g. "go.mod"
	baseContains []string // substrings of the base name, e.g. "lock"
	exts         []string // lower-cased extensions, e.g. ".yaml"
}

// pathFacts holds the derived forms of a path that pathRule patterns test.
type pathFacts struct {
	lower string
	base  string
	ext   string
}

func newPathFacts(lower string) pathFacts {
	return pathFacts{lower: lower, base: path.Base(lower), ext: strings.ToLower(filepath.Ext(lower))}
}

func (r pathRule) matches(facts pathFacts) bool {
	return anyHasPrefix(facts.lower, r.prefixes) ||
		anyHasSuffix(facts.lower, r.suffixes) ||
		anyPathSegment(facts.lower, r.segments) ||
		anyContains(facts.lower, r.contains) ||
		anyEqual(facts.base, r.baseNames) ||
		anyContains(facts.base, r.baseContains) ||
		anyEqual(facts.ext, r.exts)
}

// classifyPath returns the category of the first matching rule.
func classifyPath(facts pathFacts, rules []pathRule) (string, bool) {
	for _, rule := range rules {
		if rule.matches(facts) {
			return rule.category, true
		}
	}
	return "", false
}

var deliveryPrefixes = []string{".github/", ".buildkite/", "ci/"}

// layerRules map paths to architectural layers. Test files are classified
// before these rules run; unmatched paths are "core".
var layerRules = []pathRule{
	{category: "delivery", prefixes: deliveryPrefixes},
	{category: "docs", prefixes: []string{"docs/"}, suffixes: []string{".md"}},
	{category: "presentation", segments: []string{"ui", "view", "frontend"}, contains: []string{"/component"}},
	{category: "application", segments: []string{"api"}, contains: []string{"/handler", "/controller", "/route"}},
	{category: "domain", segments: []string{"domain"}, contains: []string{"/model/", "/service/"}},
	{category: "data", segments: []string{"db", "data"}, contains: []string{"/store/", "/repo"}},
	{category: "infrastructure", segments: []string{"infra", "platform"}, contains: []string{"/adapter/"}},
}

// concernRules map paths to product or engineering concerns. Test files are
// classified before these rules run; unmatched paths fall back to their layer
// and then to their first path segment.
var concernRules = []pathRule{
	{category: "ci", prefixes: deliveryPrefixes},
	{category: "docs", prefixes: []string{"docs/"}, exts: []string{".md", ".mdx", ".rst"}},
	{category: "dependencies", baseNames: []string{"go.mod", "go.sum", "package.json"}, baseContains: []string{"lock"}},
	{category: "config", exts: []string{".yaml", ".yml", ".json", ".toml"}},
	{category: "auth", contains: []string{"auth", "permission"}},
	{category: "billing", segments: []string{"billing"}, contains: []string{"payment", "invoice"}},
	{category: "api", segments: []string{"api"}, baseContains: []string{"openapi", "swagger"}, exts: []string{".proto", ".graphql", ".gql"}},
	{category: "ui", segments: []string{"ui", "frontend", "view"}, contains: []string{"/component"}},
	{category: "data", segments: []string{"db", "data"}, contains: []string{"/store/", "/repo", "migration"}},
	{category: "infra", segments: []string{"infra", "platform", "deploy"}, contains: []string{"docker"}},
}

// publicSurfaceRule matches API, SDK, schema, and package-entry files.
var publicSurfaceRule = pathRule{
	category:     "public",
	segments:     []string{"api", "apis", "public", "pkg", "sdk", "include", "proto", "graphql"},
	baseNames:    []string{"index.ts", "index.tsx", "index.js", "index.jsx"},
	baseContains: []string{"openapi", "swagger"},
	exts:         []string{".proto", ".graphql", ".gql"},
}

func isPublicSurfaceFile(env support.Context, rel string) bool {
	// The SDK facade used to be tested here too; it is a strict subset of
	// IsPublicPackageFile ("pkg/"), so the call only ever duplicated the next
	// one. Dropping it also keeps this predicate target-free.
	if env.IsPublicPackageFile != nil && env.IsPublicPackageFile(rel) {
		return true
	}
	return publicSurfaceRule.matches(newPathFacts(strings.ToLower(normalizePath(rel))))
}

func layerCategory(rel string) string {
	lower := strings.ToLower(normalizePath(rel))
	if isTestFile(lower) {
		return "tests"
	}
	if category, ok := classifyPath(newPathFacts(lower), layerRules); ok {
		return category
	}
	return "core"
}

func concernFamily(rel string) string {
	lower := strings.ToLower(normalizePath(rel))
	if isTestFile(lower) {
		return "tests"
	}
	if category, ok := classifyPath(newPathFacts(lower), concernRules); ok {
		return category
	}
	if layer := layerCategory(lower); layer != "core" {
		return layer
	}
	return firstPathSegment(lower)
}

func anyHasPrefix(value string, prefixes []string) bool {
	for _, prefix := range prefixes {
		if strings.HasPrefix(value, prefix) {
			return true
		}
	}
	return false
}

func anyHasSuffix(value string, suffixes []string) bool {
	for _, suffix := range suffixes {
		if strings.HasSuffix(value, suffix) {
			return true
		}
	}
	return false
}

func anyContains(value string, needles []string) bool {
	for _, needle := range needles {
		if strings.Contains(value, needle) {
			return true
		}
	}
	return false
}

func anyEqual(value string, candidates []string) bool {
	for _, candidate := range candidates {
		if value == candidate {
			return true
		}
	}
	return false
}

func anyPathSegment(rel string, segments []string) bool {
	for _, segment := range segments {
		if hasPathSegment(rel, segment) {
			return true
		}
	}
	return false
}
