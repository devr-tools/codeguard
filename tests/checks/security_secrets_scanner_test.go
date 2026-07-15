package checks_test

import (
	"strings"
	"testing"

	"github.com/devr-tools/codeguard/internal/codeguard/checks/security"
	"github.com/devr-tools/codeguard/internal/codeguard/core"
)

// gateSamples are one matching line per built-in pattern. They guard the
// invariant that the cheap literal gate is a superset of every built-in pattern:
// a pattern added without a corresponding gate marker would be skipped on most
// lines, and ScanContent here would return no match for its sample.
// gateSamples are one matching line per built-in pattern, assembled via cred so
// no full, contiguous secret literal is committed to source. They guard the
// invariant that the cheap literal gate is a superset of every built-in pattern:
// a pattern added without a corresponding gate marker would be skipped on most
// lines, and ScanContent here would return no match for its sample.
func gateSamples() []string {
	return []string{
		`const k = "` + cred("AKIA", "1234567890ABCDEF") + `"`,
		`const k = "` + cred("ghp_", "0123456789abcdefghijklmnopqrstuvwxyz") + `"`,
		`const k = "` + cred("github_", "pat_0123456789abcdefghijkl") + `"`,
		`const k = "` + cred("glpat-", "0123456789abcdefABCD") + `"`,
		`const k = "` + cred("xox", "b-0123456789-abcdefABCDEF") + `"`,
		`const u = "` + cred("https://hooks.slack.com/services/", "T01/B04/abcdefABCDEF0123456789") + `"`,
		`const k = "` + cred("sk_", "live_0123456789abcdefABCDEFGH") + `"`,
		`const k = "` + cred("AIza", "0123456789abcdefABCDEFGHIJKLMNOPQRS") + `"`,
		`const k = "` + cred("npm_", "0123456789abcdefghijklmnopqrstuvwxyz") + `"`,
		`const k = "` + cred("SG.", "0123456789abcdefghijkl.0123456789abcdefghijklmnopqrstuvwxyzABCDEFG") + `"`,
		`const k = "` + cred("SK", "0123456789abcdef0123456789abcdef") + `"`,
		`const k = "` + cred("pypi-", "AgEIcHlwaS5vcmcabcdef") + `"`,
		`const k = "` + cred("dckr_", "pat_aBcDeFgHiJkLmNoPqRsT") + `"`,
		`conn := "` + cred("AccountKey=", "0123456789abcdefABCDEF0123456789abcdefABCDEF0123456789") + `"`,
		`db := "` + cred("postgres://admin:", "s3cr3tP4ssw0rd@db.host.net:5432/app") + `"`,
		`h := "` + cred("Authorization: Bearer ", "abcdefghijklmnop0123456789") + `"`,
		`client_secret = '` + cred("abcdefghij", "klmnop1234") + `'`,
		`apiKey = "` + cred("supersecret", "value1234") + `"`,
		`-----BEGIN RSA PRIVATE KEY-----`,
	}
}

func TestSecretScannerGateCoversBuiltins(t *testing.T) {
	scanner, _ := security.BuildScanner(nil)
	for _, sample := range gateSamples() {
		if len(scanner.ScanContent(sample)) == 0 {
			t.Errorf("scanner missed a built-in sample (gate marker likely missing): %q", sample)
		}
	}
}

func TestBuildScannerReportsUnusablePatterns(t *testing.T) {
	scanner, issues := security.BuildScanner(&core.SecretsRulesConfig{
		AllowPatterns: []string{`//\s*ok`, `(`},
		CustomPatterns: []core.CustomSecretPattern{
			{ID: "good", Regex: `\bacme_live_[0-9a-f]{16}\b`},
			{ID: "bad-regex", Regex: `[`},
			{ID: "   ", Regex: `\btoken\b`},
		},
	})
	if len(issues) != 3 {
		t.Fatalf("issues = %v, want 3 (bad allow pattern, bad custom regex, empty custom id)", issues)
	}
	for _, want := range []string{"allow_patterns[1]", `custom_patterns["bad-regex"]`, "empty id"} {
		found := false
		for _, issue := range issues {
			if strings.Contains(issue, want) {
				found = true
			}
		}
		if !found {
			t.Errorf("no issue mentions %q: %v", want, issues)
		}
	}

	// The valid entries still participate in the scan.
	if matches := scanner.ScanContent(`key = "acme_live_0123456789abcdef"`); len(matches) != 1 || matches[0].RuleID != "good" {
		t.Fatalf("valid custom pattern did not survive invalid siblings: %+v", matches)
	}
	if matches := scanner.ScanContent(`key = "` + cred("AKIA", "1234567890ABCDEF") + `" // ok`); len(matches) != 0 {
		t.Fatalf("valid allow pattern did not survive invalid sibling: %+v", matches)
	}
}

func TestBuildScannerNoIssuesForValidConfig(t *testing.T) {
	_, issues := security.BuildScanner(&core.SecretsRulesConfig{
		AllowPatterns:  []string{`//\s*ok`},
		CustomPatterns: []core.CustomSecretPattern{{ID: "good", Regex: `token-[0-9]+`}},
	})
	if len(issues) != 0 {
		t.Fatalf("issues = %v, want none for a valid config", issues)
	}
}

func benchSource() string {
	const block = "func handler(ctx context.Context, req *Request) (*Response, error) {\n" +
		"\treturn &Response{Status: 200, Body: req.Payload}, nil\n"
	out := make([]byte, 0, len(block)*1000+128)
	for i := 0; i < 1000; i++ {
		out = append(out, block...)
	}
	out = append(out, "const awsKey = \""+cred("AKIA", "1234567890ABCDEF")+"\"\n"...)
	return string(out)
}

func BenchmarkSecretScanContent(b *testing.B) {
	scanner, _ := security.BuildScanner(nil)
	source := benchSource()
	b.SetBytes(int64(len(source)))
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = scanner.ScanContent(source)
	}
}

func BenchmarkSecretScanContentEntropy(b *testing.B) {
	enabled := true
	scanner, _ := security.BuildScanner(&core.SecretsRulesConfig{Entropy: &core.SecretsEntropyConfig{Enabled: &enabled}})
	source := benchSource()
	b.SetBytes(int64(len(source)))
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = scanner.ScanContent(source)
	}
}
