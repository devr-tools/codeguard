package security

import (
	"regexp"
	"strings"
)

const (
	hardcodedCredentialRule = "security.hardcoded-credential"
	hardcodedSecretRule     = "security.hardcoded-secret"
	privateKeyRule          = "security.private-key"
	highEntropyRule         = "security.high-entropy-string"
)

// secretPattern is the lower-confidence name-based heuristic: an assignment
// whose identifier looks secret-bearing next to a quoted value. It reports at
// warn. privateKeyPattern detects PEM key material and reports at fail.
var (
	secretPattern     = regexp.MustCompile(`(?i)(secret|token|api[_-]?key|password)\s*[:=]\s*["']([^"']{8,})["']`)
	privateKeyPattern = regexp.MustCompile(`-----BEGIN [A-Z ]*PRIVATE KEY-----`)

	// quotedLiteralPattern captures whitespace-free quoted literals for the
	// entropy pass (secrets are contiguous, so requiring no whitespace skips prose).
	quotedLiteralPattern = regexp.MustCompile("[\"'`]([^\"'`\\s]{12,})[\"'`]")
)

type credentialPattern struct {
	re  *regexp.Regexp
	msg string
}

// credentialPatterns are high-confidence, provider-specific credential formats.
// They are matched against the raw line because the token lives inside a string
// literal that source masking would blank. Patterns with a capture group have
// their captured value placeholder-checked; the rest match a self-contained token.
//
// INVARIANT: every pattern's required literal must appear in gateLiterals or
// gateFoldLiterals below, or the pattern is silently skipped on most lines.
var credentialPatterns = []credentialPattern{
	{regexp.MustCompile(`\b(?:AKIA|ASIA)[0-9A-Z]{16}\b`), "AWS access key id"},
	{regexp.MustCompile(`\b(?:ghp|gho|ghu|ghs|ghr)_[0-9A-Za-z]{36}\b`), "GitHub access token"},
	{regexp.MustCompile(`\bgithub_pat_[0-9A-Za-z_]{22,}\b`), "GitHub fine-grained token"},
	{regexp.MustCompile(`\bglpat-[0-9A-Za-z_-]{20}\b`), "GitLab personal access token"},
	{regexp.MustCompile(`\bxox[baprs]-[0-9A-Za-z-]{10,}\b`), "Slack token"},
	{regexp.MustCompile(`https://hooks\.slack\.com/services/T[A-Z0-9]+/B[A-Z0-9]+/[A-Za-z0-9]{16,}`), "Slack webhook URL"},
	{regexp.MustCompile(`\b(?:sk|rk)_live_[0-9A-Za-z]{20,}\b`), "Stripe live secret key"},
	{regexp.MustCompile(`\bAIza[0-9A-Za-z_-]{35}\b`), "Google API key"},
	{regexp.MustCompile(`\bnpm_[0-9A-Za-z]{36}\b`), "npm access token"},
	{regexp.MustCompile(`\bSG\.[0-9A-Za-z_-]{22}\.[0-9A-Za-z_-]{43}\b`), "SendGrid API key"},
	{regexp.MustCompile(`\bSK[0-9a-f]{32}\b`), "Twilio API key"},
	{regexp.MustCompile(`\bpypi-[A-Za-z0-9_-]{16,}\b`), "PyPI API token"},
	{regexp.MustCompile(`\bdckr_pat_[A-Za-z0-9_-]{20,}\b`), "Docker Hub access token"},
	{regexp.MustCompile(`(?i)AccountKey=([A-Za-z0-9+/=]{40,})`), "Azure storage account key"},
	{regexp.MustCompile(`(?i)\b(?:postgres|postgresql|mysql|mongodb(?:\+srv)?|redis|amqp|amqps)://[^:@/\s]+:([^@/\s]+)@`), "database connection string with embedded credentials"},
	{regexp.MustCompile(`(?i)authorization["']?\s*[:=]\s*["']?bearer\s+([A-Za-z0-9._\-]{16,})`), "hardcoded bearer token"},
	{regexp.MustCompile(`(?i)(?:aws_secret_access_key|client_secret|private_token)\s*[:=]\s*["']([^"']{16,})["']`), "hardcoded credential assignment"},
}

// placeholderPattern recognizes obvious non-secret filler so fixtures and
// templates do not fail a scan: redacted/changeme/placeholder/dummy/fake/example
// words, your-... hints, all-filler runs, and interpolation/env references.
var placeholderPattern = regexp.MustCompile(`(?i)^(?:[x*.\-_0]+|redacted|changeme|placeholder|dummy|fake|example|your[-_].*|<.*>|\$\{.*\}|\$\(.*\)?|\{\{.*\}\}|process\.env\..*|os\.environ.*)$`)

// gateLiterals are case-sensitive substrings that every built-in credential,
// private-key, or connection-string pattern requires. gateFoldLiterals are the
// case-insensitive markers for identifier-based patterns (always lowercase
// here). A line that contains none of these cannot match any built-in pattern,
// so the expensive per-pattern regexes are skipped. The gate is a cheap
// substring scan — `strings.Contains` is ~85x faster than the equivalent regex
// alternation, which dominated profiling.
var (
	gateLiterals = []string{
		"AKIA", "ASIA", "ghp_", "gho_", "ghu_", "ghs_", "ghr_", "github_pat_",
		"glpat-", "xox", "hooks.slack.com", "_live_", "AIza", "npm_", "SG.", "SK",
		"pypi-", "dckr_pat_", "PRIVATE KEY", "://",
	}
	gateFoldLiterals = []string{
		"secret", "token", "password", "apikey", "api_key", "api-key", "bearer", "accountkey",
	}
)

// matchPrivateKey, matchCredential, and matchNameBased are the built-in tiers,
// kept beside the patterns they apply. Each returns a position-agnostic *Match
// (scanLine stamps the line via located).
func matchPrivateKey(line string) *Match {
	if privateKeyPattern.MatchString(line) {
		return &Match{RuleID: privateKeyRule, Level: "fail", Message: "private key material detected"}
	}
	return nil
}

func matchCredential(line string) *Match {
	for _, pattern := range credentialPatterns {
		match := pattern.re.FindStringSubmatch(line)
		if match == nil {
			continue
		}
		if len(match) > 1 && match[len(match)-1] != "" && isPlaceholderSecret(match[len(match)-1]) {
			continue
		}
		return &Match{RuleID: hardcodedCredentialRule, Level: "fail", Message: "possible hardcoded credential detected (" + pattern.msg + "): " + maskSecret(credentialMatchValue(match))}
	}
	return nil
}

func matchNameBased(line string) *Match {
	match := secretPattern.FindStringSubmatch(line)
	if match == nil || isPlaceholderSecret(match[len(match)-1]) {
		return nil
	}
	return &Match{RuleID: hardcodedSecretRule, Level: "warn", Message: "possible hardcoded secret detected: " + maskSecret(match[len(match)-1])}
}

// builtinGatePasses reports whether a line could match any built-in pattern.
// False positives only cost a wasted regex run, never correctness.
func builtinGatePasses(line string) bool {
	for _, marker := range gateLiterals {
		if strings.Contains(line, marker) {
			return true
		}
	}
	for _, marker := range gateFoldLiterals {
		if asciiContainsFold(line, marker) {
			return true
		}
	}
	return false
}

// asciiContainsFold reports whether s contains sub, case-insensitively for ASCII
// letters, without allocating (unlike strings.Contains(strings.ToLower(s), sub)).
// sub must be lowercase ASCII.
func asciiContainsFold(s string, sub string) bool {
	n, m := len(s), len(sub)
	if m == 0 {
		return true
	}
	if m > n {
		return false
	}
	for i := 0; i <= n-m; i++ {
		j := 0
		for ; j < m; j++ {
			c := s[i+j]
			if c >= 'A' && c <= 'Z' {
				c += 'a' - 'A'
			}
			if c != sub[j] {
				break
			}
		}
		if j == m {
			return true
		}
	}
	return false
}
