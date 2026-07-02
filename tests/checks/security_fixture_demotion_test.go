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

func TestSecurityFixturePathsDemoteCredentialFindings(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name string
		path string
	}{
		{"testdata dir", "testdata/creds.txt"},
		{"fixtures dir", "fixtures/creds.txt"},
		{"dunder fixtures dir", "src/__fixtures__/creds.ts"},
		{"go test file", "pkg/handler_test.go"},
		{"ts test file", "web/app.test.ts"},
		{"python test file", "scripts/util_test.py"},
		{"ts spec file", "src/api.spec.ts"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			dir := t.TempDir()
			writeFile(t, filepath.Join(dir, filepath.FromSlash(tc.path)), "key = \""+cred("AKIA", "1234567890ABCDEF")+"\"\n")

			report := fixtureDemotionReport(t, dir, nil)
			// Demoted, never silent: the finding is still present at warn.
			assertSectionStatus(t, report, "Security", "warn")
			assertFindingLevel(t, report, "Security", "security.hardcoded-credential", "warn")
			assertFindingConfidence(t, report, "Security", "security.hardcoded-credential", "low")
			finding := findFinding(t, report, "Security", "security.hardcoded-credential")
			if !strings.HasSuffix(finding.Message, " (fixture path)") {
				t.Fatalf("message missing fixture-path suffix: %q", finding.Message)
			}
		})
	}
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
	if strings.Contains(finding.Message, "(fixture path)") {
		t.Fatalf("non-fixture finding unexpectedly demoted: %q", finding.Message)
	}
}

func TestSecurityFixtureDemotionDisabledKeepsOldBehavior(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "testdata", "creds.txt"), "key = \""+cred("AKIA", "1234567890ABCDEF")+"\"\n")

	report := fixtureDemotionReport(t, dir, boolPtr(false))
	assertSectionStatus(t, report, "Security", "fail")
	assertFindingLevel(t, report, "Security", "security.hardcoded-credential", "fail")
	assertFindingConfidence(t, report, "Security", "security.hardcoded-credential", "high")
	finding := findFinding(t, report, "Security", "security.hardcoded-credential")
	if strings.Contains(finding.Message, "(fixture path)") {
		t.Fatalf("finding demoted with toggle off: %q", finding.Message)
	}
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
	if !strings.HasSuffix(finding.Message, " (fixture path)") {
		t.Fatalf("message missing fixture-path suffix: %q", finding.Message)
	}
}

func TestSecurityFixtureDemotionDemotesHighEntropyString(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "testdata", "blob.txt"), "blob = \"k7Jx9PqL2mNvB4wR8tZc3aYd5eHfUgQ1\"\n")

	report := secretsScanConfig(t, dir, &codeguard.SecretsRulesConfig{
		Enabled: boolPtr(true),
		Entropy: &codeguard.SecretsEntropyConfig{Enabled: boolPtr(true), Level: "fail"},
	}, "go")
	assertSectionStatus(t, report, "Security", "warn")
	assertFindingLevel(t, report, "Security", "security.high-entropy-string", "warn")
	assertFindingConfidence(t, report, "Security", "security.high-entropy-string", "low")
	finding := findFinding(t, report, "Security", "security.high-entropy-string")
	if !strings.HasSuffix(finding.Message, " (fixture path)") {
		t.Fatalf("message missing fixture-path suffix: %q", finding.Message)
	}
}

func TestSecurityFixtureDemotionKeepsPrivateKeyAtFail(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "testdata", "key.pem"), "-----BEGIN RSA PRIVATE KEY-----\nabcdef\n-----END RSA PRIVATE KEY-----\n")

	report := fixtureDemotionReport(t, dir, nil)
	// Private key material is dangerous wherever it lives; it is never demoted.
	assertSectionStatus(t, report, "Security", "fail")
	assertFindingLevel(t, report, "Security", "security.private-key", "fail")
	finding := findFinding(t, report, "Security", "security.private-key")
	if strings.Contains(finding.Message, "(fixture path)") {
		t.Fatalf("private-key finding unexpectedly demoted: %q", finding.Message)
	}
}
