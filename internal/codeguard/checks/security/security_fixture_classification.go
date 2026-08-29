package security

import "strings"

type fixtureClassification string

const (
	fixtureConfirmed       fixtureClassification = "confirmed"
	fixtureAmbiguous       fixtureClassification = "ambiguous_fixture"
	fixtureLikelySynthetic fixtureClassification = "likely_synthetic_fixture"
)

type fixtureAssessment struct {
	Classification fixtureClassification
	Evidence       []string
}

var syntheticTokens = []string{"fixture", "example", "dummy", "fake", "mock", "test", "local"}
var fixtureSymbolTokens = []string{"test", "fake", "mock", "fixture"}

func classifyFixtureCandidate(path, line string, match Match) fixtureAssessment {
	if match.SecretType == "private_key" || match.SecretType == "high_entropy" || match.RuleID == hardcodedCredentialRule {
		return fixtureAssessment{Classification: fixtureConfirmed, Evidence: []string{"credential_structure:" + match.SecretType}}
	}
	if !isFixturePath(path) {
		return fixtureAssessment{Classification: fixtureConfirmed, Evidence: []string{"path_scope:non_fixture"}}
	}
	lower := strings.ToLower(line)
	evidence := []string{"path_scope:fixture"}
	value := lower
	symbol := false
	if assignment := strings.IndexAny(lower, "=:"); assignment > 0 {
		symbol = containsAny(lower[:assignment], fixtureSymbolTokens)
		value = lower[assignment+1:]
	}
	synthetic := containsAny(value, syntheticTokens)
	if strings.Contains(value, "example.com") {
		evidence = append(evidence, "host:reserved_example")
	}
	if symbol {
		evidence = append(evidence, "symbol:fixture_convention")
	}
	if synthetic {
		evidence = append(evidence, "value:synthetic_component")
	}
	if symbol && synthetic {
		return fixtureAssessment{Classification: fixtureLikelySynthetic, Evidence: evidence}
	}
	return fixtureAssessment{Classification: fixtureAmbiguous, Evidence: evidence}
}

func containsAny(value string, tokens []string) bool {
	for _, token := range tokens {
		if strings.Contains(value, token) {
			return true
		}
	}
	return false
}
