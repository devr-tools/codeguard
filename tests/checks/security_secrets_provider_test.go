package checks_test

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/devr-tools/codeguard/internal/codeguard/checks/security"
	"github.com/devr-tools/codeguard/internal/codeguard/core"
	"github.com/devr-tools/codeguard/pkg/codeguard"
)

// providerKeyBody is a valid-shaped, non-filler token body. Every fixture is
// assembled at runtime via cred so no contiguous provider token is committed.
const providerKeyBody = "4eC39HqLyjWDarjtT1zdp7dc"

// Provider-prefixed keys with a valid shape are matched by the provider tier,
// so they report as hardcoded-credential at high confidence even when the
// assigned identifier is also secret-named (which used to let the name-based
// heuristic win at low confidence).
func TestSecretScannerProviderPrefixedKeysAreHighConfidence(t *testing.T) {
	scanner, _ := security.BuildScanner(nil)
	cases := map[string]string{
		"stripe underscore":    `const k = "` + cred("sk_", "live_"+providerKeyBody) + `"`,
		"stripe hyphen":        `const k = "` + cred("sk-", "live-"+providerKeyBody) + `"`,
		"stripe hyphen named":  `apiKey = "` + cred("sk-", "live-"+providerKeyBody) + `"`,
		"stripe restricted":    `secret = "` + cred("rk_", "live_"+providerKeyBody) + `"`,
		"openai project":       `OPENAI_API_KEY = "` + cred("sk-", "proj-"+providerKeyBody+providerKeyBody) + `"`,
		"openai legacy":        `const k = "` + cred("sk-", "abcdefghij0123456789"+"T3Blbk"+"FJ"+"abcdefghij0123456789") + `"`,
		"anthropic":            `ANTHROPIC_API_KEY = "` + cred("sk-", "ant-api03-"+providerKeyBody+providerKeyBody) + `"`,
		"hugging face":         `token = "` + cred("hf_", "AbCdEfGhIjKlMnOpQrStUvWxYzAbCdEfGh") + `"`,
		"github classic":       `token = "` + cred("ghp_", "0123456789abcdefghijklmnopqrstuvwxyz") + `"`,
		"github fine-grained":  `token = "` + cred("github_", "pat_0123456789abcdefghijkl") + `"`,
		"slack bot":            `token = "` + cred("xox", "b-0123456789-abcdefABCDEF") + `"`,
		"aws access key named": `aws_key_secret = "` + cred("AKIA", "1234567890ABCDEF") + `"`,
	}
	for name, line := range cases {
		t.Run(name, func(t *testing.T) {
			matches := scanner.ScanContent(line)
			if len(matches) != 1 {
				t.Fatalf("matches = %+v, want exactly one", matches)
			}
			got := matches[0]
			if got.RuleID != "security.hardcoded-credential" || got.Confidence != core.ConfidenceHigh || got.Level != "fail" {
				t.Fatalf("match = %+v, want hardcoded-credential/high/fail", got)
			}
		})
	}
}

// Shapes that only resemble a provider key stay out of the provider tier:
// too-short bodies, Stripe test-mode and publishable keys, and a bare "sk-".
func TestSecretScannerProviderShapesRequireValidBodies(t *testing.T) {
	scanner, _ := security.BuildScanner(nil)
	for _, line := range []string{
		`const k = "` + cred("sk-", "live-short") + `"`,
		`const k = "` + cred("pk_", "live_"+providerKeyBody) + `"`,
		`const k = "` + cred("sk_", "test_"+providerKeyBody) + `"`,
		`const k = "` + cred("sk-", "1234") + `"`,
		`const k = "` + cred("hf_", "short") + `"`,
	} {
		for _, match := range scanner.ScanContent(line) {
			if match.RuleID == "security.hardcoded-credential" {
				t.Errorf("%q matched the provider tier: %+v", line, match)
			}
		}
	}
}

// Documentation placeholders built on a provider prefix (sk_test_xxx,
// sk_live_your_key) are filler, not credentials, and must not be reported at
// all; a real-shaped provider key in the same docs file is still reported at
// the documentation review tier exactly as before.
func TestSecretProviderPlaceholderInDocsIsTrueNegative(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "docs", "billing.md"), strings.Join([]string{
		"# Billing",
		"",
		"Set `STRIPE_SECRET_KEY=" + cred("sk_", "test_xxx") + "` in your environment.",
		"",
		"```python",
		"stripe_secret = \"" + cred("sk_", "test_xxxxxxxxxxxx") + "\"",
		"api_key = \"" + cred("sk_", "live_your_key_here") + "\"",
		"```",
		"",
	}, "\n"))
	report := fixtureDemotionReport(t, dir, nil)
	for _, section := range report.Sections {
		for _, finding := range section.Findings {
			if strings.HasPrefix(finding.RuleID, "security.hardcoded") {
				t.Fatalf("placeholder reported: %#v", finding)
			}
		}
	}
}

// Fixture and documentation grading is unchanged by the stronger provider
// tier: a real-shaped provider key under a test path is the ambiguous review
// tier (warn, medium), not a CI failure and not low confidence.
func TestSecretProviderKeyInFixturePathKeepsAmbiguousGrade(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "billing", "client_test.go"),
		"package billing\n\nconst apiKey = \""+cred("sk-", "live-"+providerKeyBody)+"\"\n")
	writeFile(t, filepath.Join(dir, "billing", "client.go"),
		"package billing\n\nconst liveKey = \""+cred("sk-", "live-"+providerKeyBody)+"\"\n")

	report := fixtureDemotionReport(t, dir, nil)
	byPath := map[string]codeguard.Finding{}
	for _, section := range report.Sections {
		for _, finding := range section.Findings {
			if finding.RuleID == "security.hardcoded-credential" {
				byPath[filepath.Base(finding.Path)] = finding
			}
		}
	}
	fixture, ok := byPath["client_test.go"]
	if !ok || fixture.Level != "warn" || fixture.Confidence != "medium" || fixture.Metadata["classification"] != "ambiguous_fixture" {
		t.Fatalf("fixture finding = %#v, want ambiguous_fixture warn/medium", fixture)
	}
	production, ok := byPath["client.go"]
	if !ok || production.Level != "fail" || production.Confidence != "high" {
		t.Fatalf("production finding = %#v, want fail/high", production)
	}
}
