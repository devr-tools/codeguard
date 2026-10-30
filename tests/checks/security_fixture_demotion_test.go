package checks_test

import (
	"context"
	"path/filepath"
	"strings"
	"testing"

	"github.com/devr-tools/codeguard/pkg/codeguard"
)

// fixtureDemotionReport runs a security-only scan of dir with the fixture
// demotion toggle set explicitly (nil exercises the default).
func fixtureDemotionReport(t *testing.T, dir string, demote *bool) codeguard.Report {
	t.Helper()
	cfg := codeguard.ExampleConfig()
	cfg.Name = "security-fixture-demotion"
	cfg.Targets = []codeguard.TargetConfig{{Name: "repo", Path: dir, Language: "go"}}
	cfg.Checks.Security = true
	cfg.Checks.Design = false
	cfg.Checks.Prompts = false
	cfg.Checks.CI = false
	cfg.Checks.Quality = false
	cfg.Checks.SupplyChain = false
	cfg.Checks.SecurityRules.GovulncheckMode = "off"
	// ExampleConfig allowlists testdata/** for the secret scan; clear it so
	// these tests exercise the demotion path rather than the allowlist.
	cfg.Checks.SecurityRules.Secrets = nil
	cfg.Checks.SecurityRules.DemoteFixtureFindings = demote

	report, err := codeguard.Run(context.Background(), cfg)
	if err != nil {
		t.Fatalf("run: %v", err)
	}
	return report
}

// TestSecurityFixturePathsDemoteProviderCredentialsToReview pins the core
// trade-off: a credential-shaped literal in a test path is moved to the review
// tier (warn, medium confidence) rather than suppressed. A real leaked key does
// not stop being real because of the file it lives in, so the finding must stay
// reportable and baselinable — it simply no longer fails CI on a fixture.
func TestSecurityFixturePathsDemoteProviderCredentialsToReview(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name string
		path string
	}{
		{"testdata dir", "testdata/creds.txt"},
		{"fixtures dir", "fixtures/creds.txt"},
		{"dunder fixtures dir", "src/__fixtures__/creds.ts"},
		{"dunder tests dir", "src/__tests__/creds.ts"},
		{"mocks dir", "src/__mocks__/creds.ts"},
		{"tests dir", "tests/creds.txt"},
		{"singular test dir", "test/creds.txt"},
		{"spec dir", "spec/creds.txt"},
		{"e2e dir", "e2e/creds.txt"},
		{"cypress dir", "cypress/creds.js"},
		{"go test file", "pkg/handler_test.go"},
		{"ts test file", "web/app.test.ts"},
		{"tsx test file", "web/app.test.tsx"},
		{"js spec file", "web/app.spec.js"},
		{"jsx spec file", "web/app.spec.jsx"},
		{"mjs test file", "web/app.test.mjs"},
		{"storybook file", "web/Button.stories.tsx"},
		{"python test suffix", "scripts/util_test.py"},
		{"pytest prefix", "scripts/test_util.py"},
		{"pytest conftest", "scripts/conftest.py"},
		{"ruby spec file", "lib/user_spec.rb"},
		{"java test class", "src/AccountTest.java"},
		{"csharp test class", "src/AccountTests.cs"},
		{"rust test file", "crates/router_test.rs"},
		{"cpp test file", "src/engine_test.cc"},
		{"ts spec file", "src/api.spec.ts"},
		{"json fixture", "fixtures/auth.json"},
		{"cpp fixture", "testdata/auth_test.cpp"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			dir := t.TempDir()
			writeFile(t, filepath.Join(dir, filepath.FromSlash(tc.path)), "key = \""+cred("AKIA", "1234567890ABCDEF")+"\"\n")

			report := fixtureDemotionReport(t, dir, nil)
			assertFindingLevel(t, report, "Security", "security.hardcoded-credential", "warn")
			assertFindingConfidence(t, report, "Security", "security.hardcoded-credential", "medium")
			finding := findFinding(t, report, "Security", "security.hardcoded-credential")
			if finding.Metadata["classification"] != "ambiguous_fixture" {
				t.Fatalf("classification = %q, want ambiguous_fixture", finding.Metadata["classification"])
			}
			if !strings.HasSuffix(finding.Message, " (ambiguous credential-shaped fixture)") {
				t.Fatalf("message missing ambiguous fixture suffix: %q", finding.Message)
			}
		})
	}
}

// TestSecurityDocumentationCredentialStaysVisible covers the documentation
// scope. A credential pasted into a committed .md is a real and common leak
// vector, so it is demoted rather than dropped; only the value evidence (see
// the vendor-placeholder test) can silence it.
func TestSecurityDocumentationCredentialStaysVisible(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "docs", "okta-authentication.md"), "Send the header:\n\n    Authorization: Bearer 7f3b91c0d4e58a62b1d09f4c\n")

	report := fixtureDemotionReport(t, dir, nil)
	assertFindingLevel(t, report, "Security", "security.hardcoded-credential", "warn")
	finding := findFinding(t, report, "Security", "security.hardcoded-credential")
	if !strings.HasSuffix(finding.Message, " (ambiguous credential-shaped documentation example)") {
		t.Fatalf("message missing documentation suffix: %q", finding.Message)
	}
}

// TestSecurityVendorDocumentedPlaceholderIsDiagnostic covers the one value
// class that is recognized regardless of path: literals a vendor publishes in
// its own documentation as non-secrets. They are world-readable by design, so a
// match can never be a leak and reusing one gains an attacker nothing.
func TestSecurityVendorDocumentedPlaceholderIsDiagnostic(t *testing.T) {
	t.Parallel()
	for _, path := range []string{"src/config.go", "docs/setup.md", "pkg/config_test.go"} {
		t.Run(path, func(t *testing.T) {
			t.Parallel()
			dir := t.TempDir()
			writeFile(t, filepath.Join(dir, filepath.FromSlash(path)), "key = \""+cred("AKIAIOSFODNN7", "EXAMPLE")+"\"\n")

			report := fixtureDemotionReport(t, dir, nil)
			assertFindingRuleAbsent(t, report, "Security", "security.hardcoded-credential")
			assertSyntheticFixtureDiagnostic(t, report)
		})
	}
}

// TestSecurityFixtureCredentialWithSyntheticBodyIsDiagnostic checks that
// suppression inside a fixture path needs positive evidence about the VALUE,
// not just the path.
func TestSecurityFixtureCredentialWithSyntheticBodyIsDiagnostic(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name  string
		value string
	}{
		{"filler run", cred("AKIA", "0000000000000000")},
		{"fake marker", cred("AKIA", "FAKE000000000000")},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			dir := t.TempDir()
			writeFile(t, filepath.Join(dir, "pkg", "client_test.go"), "package pkg\n\nvar key = \""+tc.value+"\"\n")

			report := fixtureDemotionReport(t, dir, nil)
			assertFindingRuleAbsent(t, report, "Security", "security.hardcoded-credential")
			assertSyntheticFixtureDiagnostic(t, report)
		})
	}
}

// TestSecurityFixtureUnroutableHostCredentialIsDiagnostic covers a credential
// aimed at a reserved or loopback host: it cannot authenticate against anything
// real, so pairing it with a fixture path is enough to silence it.
func TestSecurityFixtureUnroutableHostCredentialIsDiagnostic(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "pkg", "store_test.go"), "package pkg\n\nvar dsn = \"postgres://app:p4ssw0rdish@127.0.0.1:5432/app\"\n")

	report := fixtureDemotionReport(t, dir, nil)
	assertFindingRuleAbsent(t, report, "Security", "security.hardcoded-credential")
	assertSyntheticFixtureDiagnostic(t, report)
}

func TestSecurityFixtureDemotionSkipsNonFixturePaths(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "src", "config.go"), goConst(cred("AKIA", "1234567890ABCDEF")))

	report := fixtureDemotionReport(t, dir, nil)
	assertSectionStatus(t, report, "Security", "fail")
	assertFindingLevel(t, report, "Security", "security.hardcoded-credential", "fail")
	assertFindingConfidence(t, report, "Security", "security.hardcoded-credential", "high")
	finding := findFinding(t, report, "Security", "security.hardcoded-credential")
	if finding.Metadata["classification"] != "confirmed" {
		t.Fatalf("classification = %q, want confirmed", finding.Metadata["classification"])
	}
}

// TestSecurityFixtureDemotionDisabledKeepsOldBehavior exercises the operator
// escape hatch: with security_rules.demote_fixture_findings off, every match is
// reported exactly as the detector produced it.
func TestSecurityFixtureDemotionDisabledKeepsOldBehavior(t *testing.T) {
	t.Parallel()
	for _, path := range []string{"testdata/creds.txt", "docs/setup.md"} {
		t.Run(path, func(t *testing.T) {
			t.Parallel()
			dir := t.TempDir()
			writeFile(t, filepath.Join(dir, filepath.FromSlash(path)), "key = \""+cred("AKIA", "1234567890ABCDEF")+"\"\n")

			report := fixtureDemotionReport(t, dir, boolPtr(false))
			assertSectionStatus(t, report, "Security", "fail")
			assertFindingLevel(t, report, "Security", "security.hardcoded-credential", "fail")
			assertFindingConfidence(t, report, "Security", "security.hardcoded-credential", "high")
		})
	}
}

// TestSecurityFixtureDemotionDisabledKeepsVendorPlaceholders proves the toggle
// also restores the vendor-placeholder matches, so nothing the detector found is
// hidden from an operator who asked for strict reporting.
func TestSecurityFixtureDemotionDisabledKeepsVendorPlaceholders(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "pkg", "config_test.go"), "package pkg\n\nvar key = \""+cred("AKIAIOSFODNN7", "EXAMPLE")+"\"\n")

	report := fixtureDemotionReport(t, dir, boolPtr(false))
	assertFindingLevel(t, report, "Security", "security.hardcoded-credential", "fail")
}

func TestSecurityFixtureDemotionMarksNameBasedSecret(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "testdata", "settings.txt"), "password = \"hunter2hunter2\"\n")

	report := fixtureDemotionReport(t, dir, nil)
	// Already warn-level; demotion still marks it and pins confidence to low.
	assertFindingLevel(t, report, "Security", "security.hardcoded-secret", "warn")
	assertFindingConfidence(t, report, "Security", "security.hardcoded-secret", "low")
	finding := findFinding(t, report, "Security", "security.hardcoded-secret")
	if !strings.HasSuffix(finding.Message, " (ambiguous credential-shaped fixture)") {
		t.Fatalf("message missing ambiguous fixture suffix: %q", finding.Message)
	}
}

func TestSecurityFixtureKeepsHighEntropyStringStrict(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "testdata", "blob.txt"), "blob = \"k7Jx9PqL2mNvB4wR8tZc3aYd5eHfUgQ1\"\n")

	report := secretsScanConfig(t, dir, &codeguard.SecretsRulesConfig{
		Enabled: boolPtr(true),
		Entropy: &codeguard.SecretsEntropyConfig{Enabled: boolPtr(true), Level: "fail"},
	}, "go")
	assertSectionStatus(t, report, "Security", "fail")
	assertFindingLevel(t, report, "Security", "security.high-entropy-string", "fail")
	assertFindingConfidence(t, report, "Security", "security.high-entropy-string", "low")
	finding := findFinding(t, report, "Security", "security.high-entropy-string")
	if finding.Metadata["classification"] != "confirmed" {
		t.Fatalf("classification = %q, want confirmed", finding.Metadata["classification"])
	}
}

func TestSecurityFixtureDemotionKeepsPrivateKeyAtFail(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "testdata", "key.pem"), "-----BEGIN RSA PRIVATE KEY-----\nabcdef\n-----END RSA PRIVATE KEY-----\n") // codeguard:ignore security.private-key -- synthetic PEM written to a temp dir to prove key material is never demoted

	report := fixtureDemotionReport(t, dir, nil)
	// Private key material is dangerous wherever it lives; it is never demoted.
	assertSectionStatus(t, report, "Security", "fail")
	assertFindingLevel(t, report, "Security", "security.private-key", "fail")
	finding := findFinding(t, report, "Security", "security.private-key")
	if strings.Contains(finding.Message, "(fixture path)") {
		t.Fatalf("private-key finding unexpectedly demoted: %q", finding.Message)
	}
}

func TestSecurityLikelySyntheticFixtureIsDiagnosticNotFinding(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "testdata", "auth.go"), "package testdata\nconst FakePassword = \"fixture-password-local\"\n")

	report := fixtureDemotionReport(t, dir, nil)
	assertFindingRuleAbsent(t, report, "Security", "security.hardcoded-secret")
	assertSyntheticFixtureDiagnostic(t, report)
}

func TestSecurityFixtureSymbolDoesNotSuppressRealSecretValue(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "testdata", "auth.go"), "package testdata\nconst FakePassword = \"actual-secret-value\"\n")

	report := fixtureDemotionReport(t, dir, nil)
	assertFindingRulePresent(t, report, "Security", "security.hardcoded-secret")
	finding := findFinding(t, report, "Security", "security.hardcoded-secret")
	if finding.Metadata["classification"] == "likely_synthetic_fixture" {
		t.Fatalf("fixture-shaped symbol suppressed a non-synthetic value: %#v", finding)
	}
}

func TestSecuritySyntheticTokensRequireValueComponents(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "testdata", "auth.go"), "package testdata\nconst FakePassword = \"latest-local-admin-secret\"\n")

	report := fixtureDemotionReport(t, dir, nil)
	assertFindingRulePresent(t, report, "Security", "security.hardcoded-secret")
}

// assertSyntheticFixtureDiagnostic asserts the Security section carries a
// likely-synthetic-fixture diagnostic, which is how a suppressed credential
// stays auditable instead of vanishing.
func assertSyntheticFixtureDiagnostic(t *testing.T, report codeguard.Report) {
	t.Helper()
	for _, section := range report.Sections {
		if section.Name != "Security" {
			continue
		}
		for _, diagnostic := range section.Diagnostics {
			if diagnostic.Kind == "likely_synthetic_fixture" {
				return
			}
		}
		t.Fatalf("diagnostics = %#v, want a likely_synthetic_fixture entry", section.Diagnostics)
	}
	t.Fatal("Security section missing")
}
