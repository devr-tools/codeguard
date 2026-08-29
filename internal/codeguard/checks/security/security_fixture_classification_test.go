package security

import "testing"

func TestClassifyFixtureCandidateRequiresCombinedEvidence(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name, path, line, secretType string
		want                         fixtureClassification
	}{
		{"provider credential in test", "testdata/aws.json", `{"accessKey":"AKIA1234567890ABCDEF"}`, "aws_access_key", fixtureConfirmed},
		{"private key in fixture", "fixtures/key.pem", "-----BEGIN PRIVATE KEY-----", "private_key", fixtureConfirmed},
		{"high entropy in test", "src/auth.test.ts", `const token = "k7Jx9PqL2mNvB4wR8tZc3aYd5eHfUgQ1"`, "high_entropy", fixtureConfirmed},
		{"explicit fake symbol and dummy value", "pkg/auth_test.go", `const FakeAuthToken = "fixture-token-for-example.com"`, "named_secret", fixtureLikelySynthetic},
		{"path alone", "testdata/auth.json", `{"password":"hunter2hunter2"}`, "named_secret", fixtureAmbiguous},
		{"production file", "config/auth.json", `{"password":"fixture-password"}`, "named_secret", fixtureConfirmed},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			ruleID := hardcodedSecretRule
			if tc.secretType == "aws_access_key" {
				ruleID = hardcodedCredentialRule
			}
			got := classifyFixtureCandidate(tc.path, tc.line, Match{RuleID: ruleID, SecretType: tc.secretType})
			if got.Classification != tc.want {
				t.Fatalf("classification = %q, want %q (evidence %v)", got.Classification, tc.want, got.Evidence)
			}
		})
	}
}

func TestScannerDetectsConcatenatedProviderCredential(t *testing.T) {
	t.Parallel()
	scanner, issues := BuildScanner(nil)
	if len(issues) != 0 {
		t.Fatal(issues)
	}
	matches := scanner.ScanContent(`const token = "AKIA1234" + "567890ABCDEF"`)
	if len(matches) != 1 || matches[0].RuleID != hardcodedCredentialRule {
		t.Fatalf("matches = %#v, want provider credential", matches)
	}
}
