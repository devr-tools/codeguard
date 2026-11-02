package support

import (
	"bufio"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"

	"github.com/devr-tools/codeguard/internal/codeguard/core"
)

// SDKFacade is the resolved identity of a repository's published API surface:
// the package(s) under pkg/ that exist to wrap the module's internals.
//
// It used to be the literal prefix "pkg/codeguard/", which exempted this
// repository from two fail-level layering rules that every other repository
// was held to - and made design.cmd-through-internal-cli a guaranteed no-op
// everywhere else. Both halves are now a property of the scanned repository:
// either stated in design_rules.sdk_facade_paths or discovered from its layout.
type SDKFacade struct {
	// prefixes are target-root-relative directory prefixes in slash form, each
	// with a trailing "/" so a prefix test cannot match a sibling directory
	// whose name merely starts the same way.
	prefixes []string
	// modulePath is the go.mod module path of the target, used to recognise an
	// import of the facade. Empty when the target has no readable go.mod.
	modulePath string
}

// MatchesFile reports whether a target-root-relative file lives inside the
// facade.
func (facade SDKFacade) MatchesFile(rel string) bool {
	if len(facade.prefixes) == 0 {
		return false
	}
	normalized := strings.TrimPrefix(filepath.ToSlash(strings.TrimSpace(rel)), "./")
	for _, prefix := range facade.prefixes {
		if strings.HasPrefix(normalized, prefix) {
			return true
		}
	}
	return false
}

// MatchesImport reports whether a Go import path refers to the facade.
//
// When the module path is known the match is module-qualified, so a third
// party's similarly named package cannot trigger it. Without a go.mod the
// check degrades to a path-segment match, which is what the hardcoded
// predicate did.
func (facade SDKFacade) MatchesImport(importPath string) bool {
	candidate := strings.TrimSuffix(filepath.ToSlash(strings.TrimSpace(importPath)), "/")
	if candidate == "" {
		return false
	}
	for _, prefix := range facade.prefixes {
		dir := strings.TrimSuffix(prefix, "/")
		if facade.modulePath != "" {
			qualified := facade.modulePath + "/" + dir
			if candidate == qualified || strings.HasPrefix(candidate, qualified+"/") {
				return true
			}
			continue
		}
		if strings.HasSuffix(candidate, "/"+dir) || strings.Contains(candidate, "/"+dir+"/") {
			return true
		}
	}
	return false
}

// CacheKey fingerprints everything the facade matches on, for per-file cache
// entries whose findings depend on it.
func (facade SDKFacade) CacheKey() string {
	return "facade|" + facade.modulePath + "|" + strings.Join(facade.prefixes, ",")
}

// SDKFacadeResolver resolves one SDKFacade per scan target. Resolution touches
// the filesystem (go.mod plus a single directory listing), so results are
// memoized for the life of a scan; the lock is needed because check sections
// run concurrently.
type SDKFacadeResolver struct {
	configured []string
	mu         sync.Mutex
	cache      map[string]SDKFacade
}

// NewSDKFacadeResolver binds a resolver to one scan's configuration.
func NewSDKFacadeResolver(cfg core.Config) *SDKFacadeResolver {
	return &SDKFacadeResolver{
		configured: NormalizeSDKFacadePaths(cfg.Checks.DesignRules.SDKFacadePaths),
		cache:      map[string]SDKFacade{},
	}
}

// For returns the facade of target.
func (resolver *SDKFacadeResolver) For(target core.TargetConfig) SDKFacade {
	if resolver == nil {
		return SDKFacade{}
	}
	root := filepath.Clean(strings.TrimSpace(target.Path))
	if root == "" || root == "." {
		root = "."
	}
	resolver.mu.Lock()
	defer resolver.mu.Unlock()
	if facade, ok := resolver.cache[root]; ok {
		return facade
	}
	facade := SDKFacade{modulePath: goModuleDirective(root)}
	if len(resolver.configured) > 0 {
		facade.prefixes = resolver.configured
	} else {
		facade.prefixes = discoverPublicPackageDirs(root)
	}
	resolver.cache[root] = facade
	return facade
}

// NormalizeSDKFacadePaths turns configured entries into comparable directory
// prefixes. Entries are directories, but a user who writes a glob tail
// ("pkg/sdk/**") means the same directory, so those tails are trimmed rather
// than rejected.
func NormalizeSDKFacadePaths(values []string) []string {
	seen := map[string]struct{}{}
	out := make([]string, 0, len(values))
	for _, value := range values {
		cleaned := strings.Trim(filepath.ToSlash(strings.TrimSpace(value)), "/")
		cleaned = strings.TrimPrefix(cleaned, "./")
		for _, tail := range []string{"/**", "/*"} {
			cleaned = strings.TrimSuffix(cleaned, tail)
		}
		cleaned = strings.Trim(cleaned, "/")
		if cleaned == "" || cleaned == "." {
			continue
		}
		prefix := cleaned + "/"
		if _, duplicate := seen[prefix]; duplicate {
			continue
		}
		seen[prefix] = struct{}{}
		out = append(out, prefix)
	}
	sort.Strings(out)
	return out
}

// discoverPublicPackageDirs is the zero-config heuristic: every directory
// directly under pkg/ is this module's public surface.
func discoverPublicPackageDirs(root string) []string {
	entries, err := os.ReadDir(filepath.Join(root, "pkg"))
	if err != nil {
		return nil
	}
	out := make([]string, 0, len(entries))
	for _, entry := range entries {
		name := entry.Name()
		if !entry.IsDir() || strings.HasPrefix(name, ".") || strings.HasPrefix(name, "_") {
			continue
		}
		out = append(out, "pkg/"+name+"/")
	}
	sort.Strings(out)
	return out
}

// goModuleDirective returns the module path declared by dir/go.mod, or "".
func goModuleDirective(dir string) string {
	file, err := os.Open(filepath.Join(dir, "go.mod")) //nolint:gosec // fixed filename joined under the configured scan root
	if err != nil {
		return ""
	}
	defer func() { _ = file.Close() }()
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.TrimSpace(strings.SplitN(scanner.Text(), "//", 2)[0])
		if rest, ok := strings.CutPrefix(line, "module "); ok {
			return strings.Trim(strings.TrimSpace(rest), `"`)
		}
	}
	return ""
}
