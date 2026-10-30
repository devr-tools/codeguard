package quality

import (
	"path"
	"path/filepath"
	"strings"

	"github.com/devr-tools/codeguard/internal/codeguard/checks/support"
	runnersupport "github.com/devr-tools/codeguard/internal/codeguard/runner/support"
)

// internalPackageOwner returns the import-path prefix that owns an internal
// package tree, i.e. the directory Go's internal-visibility rule scopes the
// tree to. For "example.com/m/internal/x" it returns "example.com/m"; for a
// path with no "internal" segment it returns ok=false.
//
// Only the FIRST internal segment matters: once a path crosses into an
// internal tree, every deeper "internal" is already inside it.
func internalPackageOwner(importPath string) (string, bool) {
	cleaned := strings.Trim(strings.TrimSpace(importPath), "/")
	if cleaned == "" {
		return "", false
	}
	segments := strings.Split(cleaned, "/")
	for idx, segment := range segments {
		if segment != "internal" {
			continue
		}
		return strings.Join(segments[:idx], "/"), true
	}
	return "", false
}

// goPackagePathForFile resolves the Go import path of the package containing
// file (a target-root-relative path). It returns "" when the owning module
// cannot be determined, which callers must treat as "unknown", never as a
// violation.
func goPackagePathForFile(modules *goModuleResolver, file string) string {
	if modules == nil {
		return ""
	}
	metadata := modules.metadataForFile(file)
	if metadata.modulePath == "" {
		return ""
	}
	dir := path.Dir(filepath.ToSlash(file))
	if moduleDir := metadata.moduleDir; moduleDir != "" && moduleDir != "." {
		trimmed, ok := trimPathPrefix(dir, moduleDir)
		if !ok {
			return ""
		}
		dir = trimmed
	}
	if dir == "" || dir == "." {
		return metadata.modulePath
	}
	return metadata.modulePath + "/" + dir
}

func trimPathPrefix(value string, prefix string) (string, bool) {
	if value == prefix {
		return "", true
	}
	if strings.HasPrefix(value, prefix+"/") {
		return strings.TrimPrefix(value, prefix+"/"), true
	}
	return "", false
}

// goInternalImportVisible reports whether Go's internal-visibility rule permits
// importer (a full package import path) to import an internal tree owned by
// owner. The rule is purely lexical: an internal tree is importable only by the
// package rooted at its parent directory and that package's descendants.
func goInternalImportVisible(importer string, owner string) bool {
	if importer == "" {
		// Unknown importing package: stay silent rather than guess.
		return true
	}
	if owner == "" {
		// A module-root "internal" tree (import path begins with "internal/")
		// is visible to the whole module; without a module path there is
		// nothing to compare against.
		return true
	}
	return importer == owner || strings.HasPrefix(importer, owner+"/")
}

// testSourceFile reports whether file is test code, so that rules about
// production layering do not fire on it. Test naming conventions are the
// primary signal because they are language-defined and repository-independent;
// ci_rules.allowed_test_paths is consulted as the configured test-path policy
// for test support files that carry no test-naming marker.
func testSourceFile(env support.Context, file string) bool {
	if testNamedSourceFile(file) {
		return true
	}
	return configuredTestPathFile(env, file)
}

// testNamedSourceFile recognises the per-language test-file naming
// conventions. It deliberately requires a separator or a camel-case boundary
// before the marker so that production files such as latest.go, request.py, or
// manifest.ts are not mistaken for tests.
func testNamedSourceFile(file string) bool {
	base := path.Base(filepath.ToSlash(file))
	stem := strings.TrimSuffix(base, path.Ext(base))
	lowered := strings.ToLower(stem)
	// Go, Python, Rust, Ruby, PHP, Elixir: foo_test.go, test_foo.py,
	// foo_spec.rb, conftest.py.
	if strings.HasSuffix(lowered, "_test") || strings.HasSuffix(lowered, "_tests") ||
		strings.HasSuffix(lowered, "_spec") || strings.HasPrefix(lowered, "test_") ||
		strings.HasPrefix(lowered, "spec_") ||
		lowered == "test" || lowered == "tests" || lowered == "conftest" {
		return true
	}
	// JS/TS: foo.test.ts, foo.spec.tsx, foo.e2e.ts.
	if strings.HasSuffix(lowered, ".test") || strings.HasSuffix(lowered, ".spec") ||
		strings.HasSuffix(lowered, ".e2e") {
		return true
	}
	// Java/C#/Kotlin/Swift: FooTest.java, FooTests.cs, FooSpec.kt. The marker
	// must start a camel-case word, so "Latest" and "Manifest" do not match.
	for _, marker := range []string{"Test", "Tests", "Spec", "Specs"} {
		if trimmed, ok := trimCamelSuffix(stem, marker); ok && trimmed != "" {
			return true
		}
	}
	return false
}

// trimCamelSuffix strips marker from stem when it forms its own camel-case
// word, i.e. the character before it is not an upper-case letter.
func trimCamelSuffix(stem string, marker string) (string, bool) {
	if !strings.HasSuffix(stem, marker) {
		return "", false
	}
	trimmed := strings.TrimSuffix(stem, marker)
	if trimmed == "" {
		return "", false
	}
	last := trimmed[len(trimmed)-1]
	if last >= 'A' && last <= 'Z' {
		return "", false
	}
	return trimmed, true
}

// configuredTestPathFile matches file against the repository's configured
// test-path policy (ci_rules.allowed_test_paths). That key is the existing
// source of truth for "where tests live"; this reuses it rather than adding a
// second one.
func configuredTestPathFile(env support.Context, file string) bool {
	normalized := filepath.ToSlash(file)
	for _, pattern := range env.Config.Checks.CIRules.AllowedTestPaths {
		pattern = strings.TrimSpace(pattern)
		if pattern == "" {
			continue
		}
		if runnersupport.MatchPattern(pattern, normalized) {
			return true
		}
		if prefix := strings.TrimSuffix(filepath.ToSlash(pattern), "**"); prefix != pattern &&
			prefix != "" && strings.HasPrefix(normalized, prefix) {
			return true
		}
	}
	return false
}
