package security

import "testing"

func TestClassifyFixtureCandidateRequiresCombinedEvidence(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name, path, line, secretType, ruleID string
		want                                 fixtureClassification
	}{
		// Key material and the entropy heuristic are never path-demoted.
		{"private key in fixture", "fixtures/key.pem", "-----BEGIN PRIVATE KEY-----", "private_key", privateKeyRule, fixtureConfirmed},
		{"high entropy in test", "src/auth.test.ts", `const token = "k7Jx9PqL2mNvB4wR8tZc3aYd5eHfUgQ1"`, "high_entropy", highEntropyRule, fixtureConfirmed},

		// A provider-format credential in a test path is demoted, not
		// suppressed: the value could still be a live key.
		{"provider credential in test", "testdata/aws.json", `{"accessKey":"AKIA1234567890ABCDEF"}`, "api_key", hardcodedCredentialRule, fixtureAmbiguous},
		{"provider credential in pytest module", "api/test_client.py", `KEY = "AKIA1234567890ABCDEF"`, "api_key", hardcodedCredentialRule, fixtureAmbiguous},
		{"provider credential in production code", "internal/config/aws.go", `key := "AKIA1234567890ABCDEF"`, "api_key", hardcodedCredentialRule, fixtureConfirmed},

		// Value-level synthetic evidence is what unlocks suppression.
		{"filler body in test", "pkg/client_test.go", `key := "AKIAFAKE000000000000"`, "api_key", hardcodedCredentialRule, fixtureLikelySynthetic},
		{"unroutable dsn in test", "pkg/store_test.go", `dsn := "postgres://app:p4ssw0rdish@127.0.0.1:5432/app"`, "connection_string", hardcodedCredentialRule, fixtureLikelySynthetic},
		{"vendor placeholder in production code", "cmd/main.go", `key := "AKIAIOSFODNN7EXAMPLE"`, "api_key", hardcodedCredentialRule, fixtureLikelySynthetic},
		{"vendor secret placeholder in docs", "docs/aws.md", `aws_secret_access_key = wJalrXUtnFEMI/K7MDENG/bPxRfiCYEXAMPLEKEY`, "credential", hardcodedCredentialRule, fixtureLikelySynthetic},

		// Documentation is its own scope, demoted but never silent.
		{"bearer token in docs", "docs/okta-authentication.md", "Authorization: Bearer 7f3b91c0d4e58a62b1d09f4c", "token", hardcodedCredentialRule, fixtureAmbiguous},
		{"bearer token in a config file", "deploy/gateway.yaml", "authorization: Bearer 7f3b91c0d4e58a62b1d09f4c", "token", hardcodedCredentialRule, fixtureConfirmed},

		// The name-based tier keeps the stricter combined bar.
		{"explicit fake symbol and dummy value", "pkg/auth_test.go", `const FakeAuthToken = "fixture-token-for-example.com"`, "named_secret", hardcodedSecretRule, fixtureLikelySynthetic},
		{"path alone", "testdata/auth.json", `{"password":"hunter2hunter2"}`, "named_secret", hardcodedSecretRule, fixtureAmbiguous},
		{"production file", "config/auth.json", `{"password":"fixture-password"}`, "named_secret", hardcodedSecretRule, fixtureConfirmed},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := classifyFixtureCandidate(tc.path, tc.line, Match{RuleID: tc.ruleID, SecretType: tc.secretType})
			if got.Classification != tc.want {
				t.Fatalf("classification = %q, want %q (evidence %v)", got.Classification, tc.want, got.Evidence)
			}
		})
	}
}

func TestClassifyPathScopeCoversCommonConventions(t *testing.T) {
	t.Parallel()
	tests := []struct {
		path string
		want pathScope
	}{
		{"pkg/handler_test.go", scopeFixture},
		{"scripts/util_test.py", scopeFixture},
		{"api/test_settings.py", scopeFixture},
		{"api/conftest.py", scopeFixture},
		{"web/app.test.ts", scopeFixture},
		{"web/app.test.tsx", scopeFixture},
		{"web/app.spec.js", scopeFixture},
		{"web/app.spec.jsx", scopeFixture},
		{"web/app.test.mjs", scopeFixture},
		{"lib/parser_spec.rb", scopeFixture},
		{"src/main/java/com/acme/AccountTest.java", scopeFixture},
		{"src/Acme/AccountTests.cs", scopeFixture},
		{"crates/api/src/router_test.rs", scopeFixture},
		{"src/engine_test.cc", scopeFixture},
		{"packages/api/src/__tests__/client.ts", scopeFixture},
		{"packages/api/src/__mocks__/client.ts", scopeFixture},
		{"e2e/checkout/flow.ts", scopeFixture},
		{"cypress/support/commands.js", scopeFixture},
		{"web/components/Button.stories.tsx", scopeFixture},
		{"test/integration/run.sh", scopeFixture},
		{"tests/checks/security_test.go", scopeFixture},
		{"spec/models/user.rb", scopeFixture},
		{"testdata/creds.txt", scopeFixture},

		{"docs/okta-authentication.md", scopeDocumentation},
		{"README.md", scopeDocumentation},
		{"guide/setup.rst", scopeDocumentation},
		{"examples/hooks/devin/config.yaml", scopeDocumentation},

		{"internal/config/aws.go", scopeProduction},
		{"deploy/gateway.yaml", scopeProduction},
		{"docker-compose.ci.yml", scopeProduction},
		// Case-sensitive PascalCase matching keeps this out of the fixture
		// scope even though its lowercased name ends in "test.java".
		{"src/main/java/com/acme/Latest.java", scopeProduction},
	}
	for _, tc := range tests {
		t.Run(tc.path, func(t *testing.T) {
			if got := classifyPathScope(tc.path); got != tc.want {
				t.Fatalf("classifyPathScope(%q) = %q, want %q", tc.path, got, tc.want)
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
