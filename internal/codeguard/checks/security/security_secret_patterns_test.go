package security

import "testing"

func TestScannerDetectsBearerHeaderSetCommaForm(t *testing.T) {
	matches := BuildDefaultScanner(t).ScanContent(`package fixtures

func setAuthHeader(req *http.Request) {
	req.Header.Set("Authorization", "Bearer REAL1111REAL2222REAL3333")
}
`)
	requireOneMatch(t, matches, hardcodedCredentialRule, 4)
	if matches[0].SecretType != "token" {
		t.Fatalf("secret type = %q, want token", matches[0].SecretType)
	}
}

func TestScannerDetectsAbbreviatedDatabasePasswordNames(t *testing.T) {
	for _, line := range []string{
		`dbPass := "Zx9Qw3Rt7Yu1Io5P"`,
		`db_pass = "Zx9Qw3Rt7Yu1Io5P"`,
		`db-pass: "Zx9Qw3Rt7Yu1Io5P"`,
		`"db_pass": "Zx9Qw3Rt7Yu1Io5P"`,
		`'db-pass': "Zx9Qw3Rt7Yu1Io5P"`,
	} {
		t.Run(line, func(t *testing.T) {
			matches := BuildDefaultScanner(t).ScanContent(line + "\n")
			requireOneMatch(t, matches, hardcodedSecretRule, 1)
			if matches[0].SecretType != "password" {
				t.Fatalf("secret type = %q, want password", matches[0].SecretType)
			}
		})
	}
}

func TestScannerSuppressesAbbreviatedDatabasePasswordPlaceholders(t *testing.T) {
	for _, line := range []string{
		`dbPass := "changeme"`,
		`db_pass = "placeholder"`,
		`"db_pass": "example"`,
	} {
		t.Run(line, func(t *testing.T) {
			matches := BuildDefaultScanner(t).ScanContent(line + "\n")
			if len(matches) != 0 {
				t.Fatalf("matches = %#v, want none", matches)
			}
		})
	}
}

func TestScannerSuppressesCommentOnlyPrivateKeyMarkers(t *testing.T) {
	content := `// -----BEGIN PRIVATE KEY-----
# -----BEGIN RSA PRIVATE KEY-----
/* -----BEGIN PRIVATE KEY----- */
 * -----BEGIN PRIVATE KEY-----
const key = ` + "`" + `
-----BEGIN PRIVATE KEY-----
FAKEFAKEFAKEFAKEFAKEFAKEFAKEFAKE
-----END PRIVATE KEY-----
` + "`" + `
`
	matches := BuildDefaultScanner(t).ScanContent(content)
	requireOneMatch(t, matches, privateKeyRule, 6)
}

func BuildDefaultScanner(t *testing.T) Scanner {
	t.Helper()
	scanner, issues := BuildScanner(nil)
	if len(issues) != 0 {
		t.Fatalf("unexpected scanner issues: %v", issues)
	}
	if !scanner.Enabled() {
		t.Fatal("default scanner should be enabled")
	}
	return scanner
}

func requireOneMatch(t *testing.T, matches []Match, rule string, line int) {
	t.Helper()
	if len(matches) != 1 {
		t.Fatalf("matches = %#v, want exactly one", matches)
	}
	if matches[0].RuleID != rule || matches[0].Line != line {
		t.Fatalf("match = %#v, want rule %s at line %d", matches[0], rule, line)
	}
}
