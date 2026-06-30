package checks_test

import (
	"context"
	"path/filepath"
	"strings"
	"testing"

	"github.com/devr-tools/codeguard/pkg/codeguard"
)

func boolPtr(v bool) *bool { return &v }

func secretsScanConfig(t *testing.T, dir string, secrets *codeguard.SecretsRulesConfig, language string) codeguard.Report {
	t.Helper()
	cfg := codeguard.ExampleConfig()
	cfg.Name = "security-secrets"
	cfg.Targets = []codeguard.TargetConfig{{Name: "repo", Path: dir, Language: language}}
	cfg.Checks.Security = true
	cfg.Checks.Design = false
	cfg.Checks.Prompts = false
	cfg.Checks.CI = false
	cfg.Checks.Quality = false
	cfg.Checks.SupplyChain = false
	cfg.Checks.SecurityRules.GovulncheckMode = "off"
	cfg.Checks.SecurityRules.Secrets = secrets

	report, err := codeguard.Run(context.Background(), cfg)
	if err != nil {
		t.Fatalf("run: %v", err)
	}
	return report
}

func TestSecurityDetectsKnownCredentialFormats(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name     string
		path     string
		language string
		source   string
	}{
		{"aws", "config.go", "go", goConst(cred("AKIA", "1234567890ABCDEF"))},
		{"github", "config.go", "go", goConst(cred("ghp_", "0123456789abcdefghijklmnopqrstuvwxyz"))},
		{"gitlab", "config.go", "go", goConst(cred("glpat-", "0123456789abcdefABCD"))},
		{"slack", "config.go", "go", goConst(cred("xox", "b-0123456789-abcdefABCDEF"))},
		{"stripe", "config.go", "go", goConst(cred("sk_", "live_0123456789abcdefABCDEFGH"))},
		{"google", "config.go", "go", goConst(cred("AIza", "0123456789abcdefABCDEFGHIJKLMNOPQRS"))},
		{"npm", "config.go", "go", goConst(cred("npm_", "0123456789abcdefghijklmnopqrstuvwxyz"))},
		{"twilio", "config.go", "go", goConst(cred("SK", "0123456789abcdef0123456789abcdef"))},
		{"pypi", "config.go", "go", goConst(cred("pypi-", "AgEIcHlwaS5vcmcabcdef"))},
		{"docker", "config.go", "go", goConst(cred("dckr_", "pat_aBcDeFgHiJkLmNoPqRsT"))},
		{"slack_webhook", "config.go", "go", goConst(cred("https://hooks.slack.com/services/", "T01ABCD23/B04EFGH56/abcdefABCDEF0123456789"))},
		{"db_conn", "config.go", "go", goConst(cred("postgres://admin:", "s3cr3tP4ssw0rd@db.example.net:5432/app"))},
		{"bearer", "config.go", "go", goConst(cred("Authorization: Bearer ", "abcdefghijklmnop0123456789"))},
		{"assignment", "config.go", "go", "package main\nconst c = \"client_secret = '" + cred("abcdefghij", "klmnop1234") + "'\"\n"},
		// TypeScript target: proves the scan now covers TS, which bypasses findingsForFile.
		{"typescript", "src/secret.ts", "typescript", "export const key = \"" + cred("AKIA", "1234567890ABCDEF") + "\";\n"},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			dir := t.TempDir()
			writeFile(t, filepath.Join(dir, filepath.FromSlash(tc.path)), tc.source)

			report := secretsScanConfig(t, dir, nil, tc.language)
			assertSectionStatus(t, report, "Security", "fail")
			assertFindingRulePresent(t, report, "Security", "security.hardcoded-credential")
		})
	}
}

func TestSecuritySkipsPlaceholderValues(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "config.go"), "package main\n"+
		"const a = `api_key = \"your-api-key-here\"`\n"+
		"const b = `password = \"${ENV_TOKEN}\"`\n"+
		"const c = `secret = \"xxxxxxxxxxxx\"`\n"+
		"const d = `client_secret = \"example-client-secret-value\"`\n")

	report := secretsScanConfig(t, dir, nil, "go")
	assertSectionStatus(t, report, "Security", "pass")
}

func TestSecuritySecretsAllowPathsSkipsFixtures(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "testdata", "fixture.go"), "package fixture\nconst k = \""+cred("AKIA", "1234567890ABCDEF")+"\"\n")

	allowed := secretsScanConfig(t, dir, &codeguard.SecretsRulesConfig{
		Enabled:    boolPtr(true),
		AllowPaths: []string{"testdata/**"},
	}, "go")
	assertSectionStatus(t, allowed, "Security", "pass")

	// Without the allowlist the same fixture fails.
	blocked := secretsScanConfig(t, dir, nil, "go")
	assertSectionStatus(t, blocked, "Security", "fail")
}

func TestSecuritySecretsAllowPatternsSkipsLine(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "config.go"), "package main\nconst k = \""+cred("AKIA", "1234567890ABCDEF")+"\" // sample\n")

	report := secretsScanConfig(t, dir, &codeguard.SecretsRulesConfig{
		Enabled:       boolPtr(true),
		AllowPatterns: []string{`//\s*sample`},
	}, "go")
	assertSectionStatus(t, report, "Security", "pass")
}

func TestSecuritySecretsCustomPatternFails(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "config.go"), "package main\nconst k = \"acme_live_0123456789abcdef\"\n")

	report := secretsScanConfig(t, dir, &codeguard.SecretsRulesConfig{
		Enabled: boolPtr(true),
		CustomPatterns: []codeguard.CustomSecretPattern{{
			ID:      "security.acme-token",
			Regex:   `\bacme_live_[0-9a-f]{16}\b`,
			Message: "Acme live token must not be committed",
			Level:   "fail",
		}},
	}, "go")
	assertSectionStatus(t, report, "Security", "fail")
	assertFindingRulePresent(t, report, "Security", "security.acme-token")
}

func TestSecurityEntropyDetectsUnknownSecret(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "config.go"), "package main\nconst blob = \"k7Jx9PqL2mNvB4wR8tZc3aYd5eHfUgQ1\"\n")

	// Off by default: a non-format, non-name-based literal passes.
	off := secretsScanConfig(t, dir, nil, "go")
	assertSectionStatus(t, off, "Security", "pass")

	// Enabled: the high-entropy literal is reported.
	on := secretsScanConfig(t, dir, &codeguard.SecretsRulesConfig{
		Enabled: boolPtr(true),
		Entropy: &codeguard.SecretsEntropyConfig{Enabled: boolPtr(true)},
	}, "go")
	assertSectionStatus(t, on, "Security", "warn")
	assertFindingRulePresent(t, on, "Security", "security.high-entropy-string")
}

func TestSecurityCredentialFindingMasksValue(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "config.go"), "package main\nconst k = \""+cred("AKIA", "1234567890ABCDEF")+"\"\n")

	report := secretsScanConfig(t, dir, nil, "go")
	for _, section := range report.Sections {
		if section.Name != "Security" {
			continue
		}
		for _, finding := range section.Findings {
			if finding.RuleID != "security.hardcoded-credential" {
				continue
			}
			if strings.Contains(finding.Message, "1234567890") {
				t.Fatalf("finding message leaks the full secret: %q", finding.Message)
			}
			if !strings.Contains(finding.Message, "…") {
				t.Fatalf("finding message not masked: %q", finding.Message)
			}
			return
		}
	}
	t.Fatal("no hardcoded-credential finding found")
}

func TestSecuritySkipsBinaryFiles(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	// A NUL byte marks the content as binary; the embedded credential must not be reported.
	writeFile(t, filepath.Join(dir, "blob.bin"), ""+cred("AKIA", "1234567890ABCDEF")+"\x00\x01\x02binarydata")

	report := secretsScanConfig(t, dir, nil, "go")
	assertSectionStatus(t, report, "Security", "pass")
}

func TestSecurityScansLongLinePrefix(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	// A credential near the start of a very long (minified-style) line is still found.
	long := "const k = \"" + cred("AKIA", "1234567890ABCDEF") + "\"; const pad = \"" + strings.Repeat("a", 200000) + "\"\n"
	writeFile(t, filepath.Join(dir, "min.go"), "package main\n"+long)

	report := secretsScanConfig(t, dir, nil, "go")
	assertSectionStatus(t, report, "Security", "fail")
	assertFindingRulePresent(t, report, "Security", "security.hardcoded-credential")
}

func TestSecuritySecretsDisabled(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "config.go"), "package main\nconst k = \""+cred("AKIA", "1234567890ABCDEF")+"\"\n")

	report := secretsScanConfig(t, dir, &codeguard.SecretsRulesConfig{Enabled: boolPtr(false)}, "go")
	assertSectionStatus(t, report, "Security", "pass")
}
