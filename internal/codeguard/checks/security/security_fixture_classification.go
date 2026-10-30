package security

import (
	"regexp"
	"strings"
)

type fixtureClassification string

const (
	fixtureConfirmed       fixtureClassification = "confirmed"
	fixtureAmbiguous       fixtureClassification = "ambiguous_fixture"
	fixtureLikelySynthetic fixtureClassification = "likely_synthetic_fixture"
)

type fixtureAssessment struct {
	Classification fixtureClassification
	Scope          pathScope
	Evidence       []string
}

var syntheticTokens = map[string]struct{}{
	"fixture": {}, "example": {}, "dummy": {}, "fake": {}, "mock": {}, "test": {},
}
var fixtureSymbolTokens = []string{"test", "fake", "mock", "fixture"}
var syntheticComponentSeparator = regexp.MustCompile(`[^a-z0-9]+`)

// vendorDocumentedPlaceholders are credential-shaped literals that a vendor
// publishes in its own public documentation as non-secrets. They are the
// strongest synthetic signal there is: the value is world-readable by design,
// so a match can never be a leak, and using one confers no advantage on an
// attacker trying to smuggle a live key past the scanner. They are therefore
// the only value class recognized regardless of path.
var vendorDocumentedPlaceholders = []string{
	// AWS documentation examples (also allowlisted by GitHub secret scanning).
	"akiaiosfodnn7example",
	"asiaiosfodnn7example",
	"akiai44qh8dhbexample",
	"wjalrxutnfemi/k7mdeng/bpxrficyexamplekey",
	"je7mtgbclwbf/2zp9utk/h3yco8nvbexamplekey",
}

// syntheticValueMarkers are substrings that mark a credential body as filler.
// They only apply inside a fixture or documentation scope: the same words in
// production code are much weaker evidence, and this repo's own
// isPlaceholderSecret already applies the equivalent test to every pattern that
// captures its value.
var syntheticValueMarkers = []string{
	"example", "fake", "dummy", "redacted", "placeholder", "changeme",
	"notreal", "not-real", "sample", "synthetic", "xxxxxxxx", "deadbeef",
}

// unroutableCredentialHosts are reserved (RFC 2606) or loopback hosts. A
// credential aimed at one of them cannot authenticate against anything real,
// so pairing it with a fixture or documentation path is synthetic evidence.
var unroutableCredentialHosts = []string{"example.com", "example.org", "example.net", "example.test", "localhost", "127.0.0.1", "::1"}

// fillerRunPattern matches a run of one repeated character long enough to be a
// stand-in body (an AKIA prefix followed by sixteen zeros, "ghp_" followed by
// a run of x). The examples are spelled out rather than written as literals so
// this production file does not itself carry a provider-shaped credential.
var fillerRunPattern = regexp.MustCompile(`(0{8,}|x{8,}|a{8,}|z{8,}|1{8,}|\*{8,}|\.{8,}|-{8,}|_{8,})`)

// classifyFixtureCandidate grades one secret match into a reporting tier.
//
// The security trade-off, in order of precedence:
//
//  1. PEM private-key material and the entropy heuristic are never
//     path-demoted. Key material is dangerous wherever it is committed, and
//     the entropy rule is already an opt-in low-confidence signal.
//  2. A vendor's own documented placeholder is a non-secret everywhere, so it
//     drops to an informational diagnostic on any path.
//  3. Outside a fixture or documentation path everything else stays confirmed
//     (fail, high confidence) exactly as before.
//  4. Inside those paths a credential-shaped literal is DEMOTED, not
//     suppressed, unless the value itself carries synthetic evidence. A real
//     key in a _test.go file is still a leaked key, so the finding stays
//     visible and baselinable at the review tier instead of failing CI.
func classifyFixtureCandidate(path, line string, match Match) fixtureAssessment {
	if match.SecretType == "private_key" || match.SecretType == "high_entropy" {
		return fixtureAssessment{Classification: fixtureConfirmed, Scope: classifyPathScope(path), Evidence: []string{"credential_structure:" + match.SecretType}}
	}
	lower := strings.ToLower(line)
	scope := classifyPathScope(path)
	if containsAny(lower, vendorDocumentedPlaceholders) {
		return fixtureAssessment{Classification: fixtureLikelySynthetic, Scope: scope, Evidence: []string{"value:vendor_documented_placeholder"}}
	}
	if scope == scopeProduction {
		return fixtureAssessment{Classification: fixtureConfirmed, Scope: scope, Evidence: []string{"path_scope:non_fixture"}}
	}

	evidence := []string{"path_scope:" + string(scope)}
	value := lower
	symbol := false
	if assignment := strings.IndexAny(lower, "=:"); assignment > 0 {
		symbol = containsAny(lower[:assignment], fixtureSymbolTokens)
		value = lower[assignment+1:]
	}
	synthetic := containsSyntheticValueComponent(value)
	unroutable := containsAny(value, unroutableCredentialHosts)
	filler := containsAny(value, syntheticValueMarkers) || fillerRunPattern.MatchString(value)
	if unroutable {
		evidence = append(evidence, "host:unroutable")
	}
	if symbol {
		evidence = append(evidence, "symbol:fixture_convention")
	}
	if synthetic {
		evidence = append(evidence, "value:synthetic_component")
	}
	if filler {
		evidence = append(evidence, "value:synthetic_marker")
	}

	// A provider-format or connection-string match is structurally certain, so
	// the fixture symbol name is not required: the value is the only thing that
	// could be real. Positive evidence about the value suppresses; its absence
	// demotes.
	if match.RuleID == hardcodedCredentialRule {
		if filler || unroutable {
			return fixtureAssessment{Classification: fixtureLikelySynthetic, Scope: scope, Evidence: evidence}
		}
		return fixtureAssessment{Classification: fixtureAmbiguous, Scope: scope, Evidence: evidence}
	}

	// The name-based heuristic is the noisiest tier, so it keeps the stricter
	// combined bar: the symbol must follow a fixture convention AND the value
	// must look synthetic.
	if symbol && synthetic {
		return fixtureAssessment{Classification: fixtureLikelySynthetic, Scope: scope, Evidence: evidence}
	}
	return fixtureAssessment{Classification: fixtureAmbiguous, Scope: scope, Evidence: evidence}
}

func containsSyntheticValueComponent(value string) bool {
	for _, component := range syntheticComponentSeparator.Split(strings.ToLower(value), -1) {
		if _, ok := syntheticTokens[component]; ok {
			return true
		}
	}
	return false
}

func containsAny(value string, tokens []string) bool {
	for _, token := range tokens {
		if strings.Contains(value, token) {
			return true
		}
	}
	return false
}
