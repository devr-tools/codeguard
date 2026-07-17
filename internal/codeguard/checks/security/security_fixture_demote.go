package security

import (
	"path/filepath"
	"strings"

	"github.com/devr-tools/codeguard/internal/codeguard/core"
)

// fixtureDirSegments are path segments that mark conventional test-fixture
// locations; fixtureFileSuffixes are test-file naming conventions. A secret
// hit in either is overwhelmingly synthetic test data, the top false-positive
// source for the secret rules.
var (
	fixtureFileSuffixes = []string{"_test.go", ".test.ts", "_test.py", ".spec.ts"}
	fixtureDirSet       = map[string]struct{}{
		"testdata":     {},
		"fixtures":     {},
		"__fixtures__": {},
	}
)

// demotableFixtureRules are the secret heuristics subject to fixture-path
// demotion. security.private-key is deliberately excluded: real key material
// is dangerous wherever it lives.
var demotableFixtureRules = map[string]struct{}{
	hardcodedSecretRule:     {},
	hardcodedCredentialRule: {},
	highEntropyRule:         {},
}

// fixtureDemotionEnabled resolves checks.security_rules.demote_fixture_findings,
// which defaults to true when unset.
func fixtureDemotionEnabled(rules core.SecurityRulesConfig) bool {
	return rules.DemoteFixtureFindings == nil || *rules.DemoteFixtureFindings
}

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

// demoteFixtureMatch downgrades a demotable secret match found in a fixture
// path: fail becomes warn (fixture credentials are still worth a warn, never
// silent), confidence drops to low, and the message is suffixed so report
// readers can see why the finding was demoted.
func demoteFixtureMatch(match Match) Match {
	if _, ok := demotableFixtureRules[match.RuleID]; !ok {
		return match
	}
	if match.Level == "fail" {
		match.Level = "warn"
	}
	match.Confidence = core.ConfidenceLow
	match.Message += " (fixture path)"
	return match
}
