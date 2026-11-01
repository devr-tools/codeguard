package checks_test

import (
	"strings"
	"testing"

	"github.com/devr-tools/codeguard/internal/codeguard/checks/security"
)

// The name-based heuristic must only accept a single quoted literal as the
// value. These source lines open a quote after the credential name and then
// concatenate code; the old pattern ran from that quote to the next quote
// of any kind and reported the code between them (previewed as "` + …red(").
func TestSecretNameHeuristicDoesNotSpanConcatenation(t *testing.T) {
	scanner, _ := security.BuildScanner(nil)
	lines := []string{
		"\t\t`client_secret = '` + cred(\"abcdefghij\", \"klmnop1234\") + `'`,",
		"\t\t`apiKey = \"` + cred(\"supersecret\", \"value1234\") + `\"`,",
		"x := `password := \"` + strings.Repeat(\"a\", 12) + `\"`",
		`line = 'token = "' + build_token(user) + '"'`,
		`sql = "db_pass = '" + os.environ["DB"] + "'"`,
	}
	for _, line := range lines {
		for _, match := range scanner.ScanContent(line) {
			if match.RuleID == "security.hardcoded-secret" {
				t.Errorf("name heuristic spanned concatenation in %q: %+v", line, match)
			}
		}
	}
}

// A plain, non-concatenated credential assignment is still reported, and a
// value may now contain the other quote character.
func TestSecretNameHeuristicStillMatchesSingleLiterals(t *testing.T) {
	scanner, _ := security.BuildScanner(nil)
	cases := map[string]string{
		`apiKey = "` + cred("s3cr3t", "Value99xyz") + `"`:            "s3cr…9xyz",
		`password := '` + cred("hunter", "2-is-long") + `'`:          "hunt…long",
		`"token": "` + cred("tok", "-abcdefgh-123") + `"`:            "tok-…-123",
		`db_pass = "` + cred("it's-a-", "long-pass") + `"`:           "it's…pass",
		`secret = "` + cred("left", "-part-1") + `" + suffix`:        "left…rt-1",
		`api_key="` + cred("abcd", "efgh1234") + `" # trailing note`: "abcd…1234",
	}
	for line, wantMask := range cases {
		matches := scanner.ScanContent(line)
		if len(matches) != 1 || matches[0].RuleID != "security.hardcoded-secret" {
			t.Errorf("%q: matches = %+v, want one hardcoded-secret", line, matches)
			continue
		}
		if !strings.Contains(matches[0].Message, wantMask) {
			t.Errorf("%q: message %q does not preview %q", line, matches[0].Message, wantMask)
		}
	}
}
