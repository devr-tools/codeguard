package security

import (
	"path/filepath"
	"strings"

	"github.com/devr-tools/codeguard/internal/codeguard/checks/support"
	"github.com/devr-tools/codeguard/internal/codeguard/core"
)

// Every pattern below is a gatedPattern: a cheap literal pre-check runs before
// the regex, because these run on every line of every scanned file. See
// security_line_gates.go for the superset invariant the gates must hold to.
var (
	// String-literal based: matched on the raw line because the signal lives
	// inside string contents that source masking would blank.
	corsWildcardPattern = newGatedPattern("cors-wildcard",
		`(?i)access-control-allow-origin["']?\s*[:,=]\s*["']?\*`,
		foldGate("access-control-allow-origin"))
	bindAllPattern = newGatedPattern("bind-all-interfaces",
		`\b0\.0\.0\.0\b`,
		exactGate("0.0.0.0"))
	dockerfileUserRoot = newGatedPattern("dockerfile-root",
		`(?i)^\s*USER\s+root\b`,
		foldGate("root"))

	// Code/identifier based: matched on the raw line so string algorithm names
	// (e.g. getInstance("MD5")) and Go import paths (e.g. "crypto/md5"), which
	// masking would blank, are still visible. Known trade-off: comments that
	// mention these APIs can trigger the weak-hash/weak-cipher warns.
	debugEnabledPattern = newGatedPattern("debug-enabled",
		`\bdebug\s*=\s*True\b`,
		exactGate("debug"))
	weakHashPattern = newGatedPattern("weak-hash",
		`(?i)\bhashlib\.(?:md5|sha1)\s*\(|\b(?:md5|sha1)\.New\s*\(|crypto/(?:md5|sha1)|messagedigest\.getinstance\s*\(\s*"(?:md5|sha-?1)"|createhash\s*\(\s*['"](?:md5|sha1)['"]|\bDigest::(?:MD5|SHA1)\b|\b(?:MD5|SHA1)(?:CryptoServiceProvider|Managed)\b`,
		foldGate("md5", "sha1", "sha-1"))
	weakCipherPattern = newGatedPattern("weak-cipher",
		`(?i)crypto/(?:des|rc4)|cipher\.getinstance\s*\(\s*"(?:des|desede|rc4|[^"]*ecb[^"]*)"|createcipheriv\s*\(\s*['"](?:des|des-ede3|rc4|aes-\d+-ecb)|\bnew\s+(?:DESCryptoServiceProvider|RC2CryptoServiceProvider|TripleDESCryptoServiceProvider)\b|\bMODE_ECB\b`,
		foldGate("crypto/des", "crypto/rc4", "cipher.getinstance", "createcipheriv", "cryptoserviceprovider", "mode_ecb"))
	deserializePattern = newGatedPattern("insecure-deserialization",
		`(?i)\b(?:pickle|cpickle)\.loads?\s*\(|\bmarshal\.loads\s*\(|\byaml\.load\s*\(|\.readObject\s*\(|\bnew\s+ObjectInputStream\b|\bXMLDecoder\b|\bMarshal\.load\s*\(|\bunserialize\s*\(`,
		foldGate("pickle.load", "marshal.load", "yaml.load", ".readobject", "objectinputstream", "xmldecoder", "unserialize"))
)

// appendOWASPExtraLineFindings runs the language-agnostic OWASP-gap heuristics
// (A05 misconfiguration, A02 weak crypto, A08 insecure deserialization) for one
// source line.
func appendOWASPExtraLineFindings(env support.Context, file string, lineNo int, line sourceLine) []core.Finding {
	var findings []core.Finding
	commentOnly := rawLineCommentOnly(line.raw, line.masked)

	// The four raw-line rules read the line with any trailing comment removed.
	// Stripping walks the whole line, so it is done only when some gate says a
	// pattern could fire: the stripped line is a prefix of the raw line, so a
	// gate that rejects the raw line necessarily rejects the stripped one too.
	body := ""
	bodyScanned := !commentOnly && anyGatePasses(line.raw, line.ascii, corsWildcardPattern, bindAllPattern, weakHashPattern, weakCipherPattern)
	if bodyScanned {
		body = stripMaskedTrailingComment(line.raw, line.masked)
	}

	if bodyScanned && corsWildcardPattern.matchString(body, line.ascii) {
		findings = append(findings, env.NewFinding(support.FindingInput{RuleID: "security.cors-wildcard", Level: "warn", Path: file, Line: lineNo, Column: 1, Message: "wildcard CORS origin '*' allows any site to read responses"}))
	}
	if bodyScanned && bindAllPattern.matchString(body, line.ascii) {
		findings = append(findings, env.NewFinding(support.FindingInput{RuleID: "security.bind-all-interfaces", Level: "warn", Path: file, Line: lineNo, Column: 1, Message: "service binds to 0.0.0.0 (all network interfaces)"}))
	}
	if debugEnabledPattern.matchString(line.masked, line.ascii) {
		findings = append(findings, env.NewFinding(support.FindingInput{RuleID: "security.debug-enabled", Level: "warn", Path: file, Line: lineNo, Column: 1, Message: "debug mode enabled; disable in production"}))
	}
	if bodyScanned && weakHashPattern.matchString(body, line.ascii) {
		findings = append(findings, env.NewFinding(support.FindingInput{RuleID: "security.weak-hash", Level: "warn", Path: file, Line: lineNo, Column: 1, Message: "weak hash algorithm (MD5/SHA-1) should not be used for security"}))
	}
	if bodyScanned && weakCipherPattern.matchString(body, line.ascii) {
		findings = append(findings, env.NewFinding(support.FindingInput{RuleID: "security.weak-cipher", Level: "warn", Path: file, Line: lineNo, Column: 1, Message: "weak or insecure cipher (DES/RC4/ECB) should be replaced"}))
	}
	if isInsecureDeserialization(line.raw, line.ascii) {
		findings = append(findings, env.NewFinding(support.FindingInput{RuleID: "security.insecure-deserialization", Level: "warn", Path: file, Line: lineNo, Column: 1, Message: "insecure deserialization of potentially untrusted data"}))
	}
	if isDockerfile(file) && dockerfileUserRoot.matchString(line.raw, line.ascii) {
		findings = append(findings, env.NewFinding(support.FindingInput{RuleID: "security.dockerfile-root", Level: "warn", Path: file, Line: lineNo, Column: 1, Message: "container runs as root; set a non-root USER"}))
	}
	return findings
}

func rawLineCommentOnly(raw string, _ string) bool {
	trimmed := strings.TrimSpace(raw)
	return strings.HasPrefix(trimmed, "//") ||
		strings.HasPrefix(trimmed, "#") ||
		strings.HasPrefix(trimmed, "/*") ||
		strings.HasPrefix(trimmed, "*")
}

func stripMaskedTrailingComment(raw string, masked string) string {
	if len(raw) != len(masked) {
		return raw
	}
	for idx := 0; idx < len(raw); idx++ {
		if !strings.HasPrefix(raw[idx:], "//") && !strings.HasPrefix(raw[idx:], "#") && !strings.HasPrefix(raw[idx:], "/*") {
			continue
		}
		if strings.TrimSpace(masked[idx:]) == "" {
			return raw[:idx]
		}
	}
	return raw
}

// isInsecureDeserialization matches dangerous deserialization calls while
// excluding explicitly-safe YAML loaders (RE2 has no negative lookahead, so the
// safe case is filtered in code). The ToLower below is reached only for lines
// the gate already admitted, so the allocation is rare.
func isInsecureDeserialization(line string, ascii bool) bool {
	if !deserializePattern.matchString(line, ascii) {
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
