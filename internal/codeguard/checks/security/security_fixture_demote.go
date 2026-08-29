package security

import (
	"path/filepath"
	"strings"
)

// fixtureDirSegments and fixtureFileSuffixes provide one evidence signal for
// classification; a path match alone never exempts or confirms a credential.
var (
	fixtureFileSuffixes = []string{"_test.go", ".test.ts", "_test.py", ".spec.ts"}
	fixtureDirSet       = map[string]struct{}{
		"testdata":     {},
		"fixtures":     {},
		"__fixtures__": {},
	}
)

// isFixturePath reports whether the file lives in a test/fixture location.
func isFixturePath(path string) bool {
	normalized := strings.ToLower(filepath.ToSlash(path))
	for _, segment := range strings.Split(normalized, "/") {
		if _, ok := fixtureDirSet[segment]; ok {
			return true
		}
	}
	for _, suffix := range fixtureFileSuffixes {
		if strings.HasSuffix(normalized, suffix) {
			return true
		}
	}
	return false
}
