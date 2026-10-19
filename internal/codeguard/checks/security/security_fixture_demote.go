package security

import (
	"path/filepath"
	"strings"
)

// pathScope records why a file is a candidate for credential demotion.
//
// A path signal is only ever ONE piece of evidence. It can move a
// credential-shaped finding down to the review tier, but on its own it never
// suppresses one: a real AWS key committed to a _test.go file is still a
// leaked key, and an attacker who wants to hide a credential must not be able
// to do it by choosing a file name. Suppression always requires positive
// evidence about the value itself (see security_fixture_classification.go).
type pathScope string

const (
	scopeProduction    pathScope = "non_fixture"
	scopeFixture       pathScope = "fixture"
	scopeDocumentation pathScope = "documentation"
)

var (
	// fixtureFileSuffixes are lowercase file-name suffixes for the
	// snake/dot test-file conventions (Go, pytest, RSpec, Rust, C++, Jest,
	// Vitest, Jasmine, Storybook).
	fixtureFileSuffixes = []string{
		"_test.go",
		"_test.py", "_tests.py",
		"_test.rb", "_spec.rb",
		"_test.rs",
		"_test.cc", "_test.cpp", "_test.cxx", "_test.c",
		"_test.php", "_testcase.php",
		".test.ts", ".test.tsx", ".test.js", ".test.jsx", ".test.mjs", ".test.cjs", ".test.mts",
		".spec.ts", ".spec.tsx", ".spec.js", ".spec.jsx", ".spec.mjs", ".spec.cjs", ".spec.mts",
		".stories.ts", ".stories.tsx", ".stories.js", ".stories.jsx",
		".fixture.ts", ".fixture.js", ".fixtures.json", ".golden",
	}

	// fixtureFilePrefixes are lowercase base-name prefixes. They exist because
	// the dominant pytest convention (`test_*.py`) marks a test file by PREFIX,
	// which a suffix-only check cannot express; Go's `example_` files and
	// pytest's `conftest.py` follow the same shape.
	fixtureFilePrefixes = []string{"test_", "tests_", "spec_", "specs_", "conftest.", "mock_", "mocks_", "fake_", "fixture_", "fixtures_"}

	// fixturePascalBaseSuffixes are CASE-SENSITIVE base-name suffixes for the
	// JVM/.NET/Swift conventions (FooTest.java, UserTests.cs, ApiSpec.scala).
	// Matching case-sensitively is what keeps `Latest.java` — whose lowercased
	// name ends in "test.java" — out of the fixture scope.
	fixturePascalBaseSuffixes = []string{"Test", "Tests", "TestCase", "TestCases", "Spec", "Specs", "IT"}
	fixturePascalExtensions   = map[string]struct{}{
		".java": {}, ".kt": {}, ".kts": {}, ".cs": {}, ".scala": {}, ".groovy": {}, ".swift": {},
	}

	// fixtureDirSet is segment-matched at any depth, so nested layouts such as
	// packages/api/src/__tests__/ are covered without anchored globs.
	fixtureDirSet = map[string]struct{}{
		"testdata":     {},
		"test":         {},
		"tests":        {},
		"__tests__":    {},
		"spec":         {},
		"specs":        {},
		"fixture":      {},
		"fixtures":     {},
		"__fixtures__": {},
		"mocks":        {},
		"__mocks__":    {},
		"e2e":          {},
		"cypress":      {},
		"stories":      {},
		"testutil":     {},
		"testutils":    {},
	}

	// documentationDirSet and documentationExtensions mark prose and sample
	// trees. They are a separate scope from fixtures because the demotion
	// message differs and because documentation carries a different risk
	// profile (see classifyFixtureCandidate).
	documentationDirSet = map[string]struct{}{
		"docs":          {},
		"doc":           {},
		"documentation": {},
		"example":       {},
		"examples":      {},
		"sample":        {},
		"samples":       {},
	}
	documentationExtensions = map[string]struct{}{
		".md": {}, ".mdx": {}, ".markdown": {}, ".rst": {}, ".adoc": {}, ".asciidoc": {}, ".textile": {},
	}
)

// classifyPathScope reports which demotion scope a file belongs to. Fixture
// evidence wins over documentation evidence: a fixture under docs/ is still
// primarily a fixture.
func classifyPathScope(path string) pathScope {
	normalized := strings.ToLower(filepath.ToSlash(path))
	segments := strings.Split(normalized, "/")
	base := segments[len(segments)-1]

	for _, segment := range segments[:len(segments)-1] {
		if _, ok := fixtureDirSet[segment]; ok {
			return scopeFixture
		}
	}
	if isFixtureBaseName(base, filepath.Base(filepath.ToSlash(path))) {
		return scopeFixture
	}
	for _, segment := range segments[:len(segments)-1] {
		if _, ok := documentationDirSet[segment]; ok {
			return scopeDocumentation
		}
	}
	if _, ok := documentationExtensions[strings.ToLower(filepath.Ext(base))]; ok {
		return scopeDocumentation
	}
	return scopeProduction
}

// isFixtureBaseName applies the suffix, prefix, and PascalCase conventions.
// lowered is the lowercased base name; original preserves case for the
// case-sensitive JVM/.NET check.
func isFixtureBaseName(lowered string, original string) bool {
	for _, suffix := range fixtureFileSuffixes {
		if strings.HasSuffix(lowered, suffix) {
			return true
		}
	}
	for _, prefix := range fixtureFilePrefixes {
		if strings.HasPrefix(lowered, prefix) {
			return true
		}
	}
	ext := strings.ToLower(filepath.Ext(original))
	if _, ok := fixturePascalExtensions[ext]; !ok {
		return false
	}
	stem := strings.TrimSuffix(original, filepath.Ext(original))
	for _, suffix := range fixturePascalBaseSuffixes {
		if len(stem) > len(suffix) && strings.HasSuffix(stem, suffix) {
			return true
		}
	}
	return false
}
