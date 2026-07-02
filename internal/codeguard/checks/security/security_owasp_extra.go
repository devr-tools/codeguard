package security

import (
	"path/filepath"
	"regexp"
	"strings"

	"github.com/devr-tools/codeguard/internal/codeguard/checks/support"
	"github.com/devr-tools/codeguard/internal/codeguard/core"
)

var (
	// String-literal based: matched on the raw line because the signal lives
	// inside string contents that source masking would blank.
	corsWildcardPattern = regexp.MustCompile(`(?i)access-control-allow-origin["']?\s*[:,=]\s*["']?\*`)
	bindAllPattern      = regexp.MustCompile(`\b0\.0\.0\.0\b`)
	dockerfileUserRoot  = regexp.MustCompile(`(?i)^\s*USER\s+root\b`)

	// Code/identifier based: matched on the raw line so string algorithm names
	// (e.g. getInstance("MD5")) and Go import paths (e.g. "crypto/md5"), which
	// masking would blank, are still visible. Known trade-off: comments that
	// mention these APIs can trigger the weak-hash/weak-cipher warns.
	debugEnabledPattern = regexp.MustCompile(`\bdebug\s*=\s*True\b`)
	weakHashPattern     = regexp.MustCompile(`(?i)\bhashlib\.(?:md5|sha1)\s*\(|\b(?:md5|sha1)\.New\s*\(|crypto/(?:md5|sha1)|messagedigest\.getinstance\s*\(\s*"(?:md5|sha-?1)"|createhash\s*\(\s*['"](?:md5|sha1)['"]|\bDigest::(?:MD5|SHA1)\b|\b(?:MD5|SHA1)(?:CryptoServiceProvider|Managed)\b`)
	weakCipherPattern   = regexp.MustCompile(`(?i)crypto/(?:des|rc4)|cipher\.getinstance\s*\(\s*"(?:des|desede|rc4|[^"]*ecb[^"]*)"|createcipheriv\s*\(\s*['"](?:des|des-ede3|rc4|aes-\d+-ecb)|\bnew\s+(?:DESCryptoServiceProvider|RC2CryptoServiceProvider|TripleDESCryptoServiceProvider)\b|\bMODE_ECB\b`)
	deserializePattern  = regexp.MustCompile(`(?i)\b(?:pickle|cpickle)\.loads?\s*\(|\bmarshal\.loads\s*\(|\byaml\.load\s*\(|\.readObject\s*\(|\bnew\s+ObjectInputStream\b|\bXMLDecoder\b|\bMarshal\.load\s*\(|\bunserialize\s*\(`)
)

// appendOWASPExtraLineFindings runs the language-agnostic OWASP-gap heuristics
// (A05 misconfiguration, A02 weak crypto, A08 insecure deserialization) for one
// source line. raw is the unmodified line; masked has comments and string
// contents blanked where a masker exists for the language.
func appendOWASPExtraLineFindings(env support.Context, file string, lineNo int, raw string, masked string) []core.Finding {
	findings := make([]core.Finding, 0)

	if corsWildcardPattern.MatchString(raw) {
		findings = append(findings, env.NewFinding(support.FindingInput{RuleID: "security.cors-wildcard", Level: "warn", Path: file, Line: lineNo, Column: 1, Message: "wildcard CORS origin '*' allows any site to read responses"}))
	}
	if bindAllPattern.MatchString(raw) {
		findings = append(findings, env.NewFinding(support.FindingInput{RuleID: "security.bind-all-interfaces", Level: "warn", Path: file, Line: lineNo, Column: 1, Message: "service binds to 0.0.0.0 (all network interfaces)"}))
	}
	if debugEnabledPattern.MatchString(masked) {
		findings = append(findings, env.NewFinding(support.FindingInput{RuleID: "security.debug-enabled", Level: "warn", Path: file, Line: lineNo, Column: 1, Message: "debug mode enabled; disable in production"}))
	}
	if weakHashPattern.MatchString(raw) {
		findings = append(findings, env.NewFinding(support.FindingInput{RuleID: "security.weak-hash", Level: "warn", Path: file, Line: lineNo, Column: 1, Message: "weak hash algorithm (MD5/SHA-1) should not be used for security"}))
	}
	if weakCipherPattern.MatchString(raw) {
		findings = append(findings, env.NewFinding(support.FindingInput{RuleID: "security.weak-cipher", Level: "warn", Path: file, Line: lineNo, Column: 1, Message: "weak or insecure cipher (DES/RC4/ECB) should be replaced"}))
	}
	if isInsecureDeserialization(raw) {
		findings = append(findings, env.NewFinding(support.FindingInput{RuleID: "security.insecure-deserialization", Level: "warn", Path: file, Line: lineNo, Column: 1, Message: "insecure deserialization of potentially untrusted data"}))
	}
	if isDockerfile(file) && dockerfileUserRoot.MatchString(raw) {
		findings = append(findings, env.NewFinding(support.FindingInput{RuleID: "security.dockerfile-root", Level: "warn", Path: file, Line: lineNo, Column: 1, Message: "container runs as root; set a non-root USER"}))
	}
	return findings
}

// isInsecureDeserialization matches dangerous deserialization calls while
// excluding explicitly-safe YAML loaders (RE2 has no negative lookahead, so the
// safe case is filtered in code).
func isInsecureDeserialization(line string) bool {
	if !deserializePattern.MatchString(line) {
		return false
	}
	lower := strings.ToLower(line)
	if strings.Contains(lower, "yaml.load") && (strings.Contains(lower, "safeloader") || strings.Contains(lower, "csafeloader")) {
		return false
	}
	return true
}

func isDockerfile(path string) bool {
	base := filepath.Base(path)
	if strings.EqualFold(filepath.Ext(path), ".dockerfile") {
		return true
	}
	return base == "Dockerfile" || strings.HasPrefix(base, "Dockerfile.")
}
