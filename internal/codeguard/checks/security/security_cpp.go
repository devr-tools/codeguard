package security

import (
	"regexp"

	"github.com/devr-tools/codeguard/internal/codeguard/checks/support"
	"github.com/devr-tools/codeguard/internal/codeguard/core"
)

var (
	cppShellPattern      = regexp.MustCompile(`\b(?:(?:std\s*::\s*)?system|popen|_popen)\s*\(`)
	cppCurlTLSPattern    = regexp.MustCompile(`\bcurl_easy_setopt\s*\([^;]*(?:CURLOPT_SSL_VERIFYPEER|CURLOPT_SSL_VERIFYHOST)\s*,\s*(?:0(?:[uUlL]*)?|false)\b`)
	cppOpenSSLTLSPattern = regexp.MustCompile(`\bSSL_CTX_set_verify\s*\([^;]*\bSSL_VERIFY_NONE\b`)
	cppVerifyNonePattern = regexp.MustCompile(`\b(?:set_verify_mode\s*\([^;]*\bverify_none\b|set_validate_certificates\s*\(\s*false\b)`)
	cppUnsafeCAPIPattern = regexp.MustCompile(`\b(?:gets|strcpy|strcat|sprintf|vsprintf)\s*\(`)
)

func appendCPPLineFindings(env support.Context, file string, lineNo int, line string) []core.Finding {
	switch {
	case cppCurlTLSPattern.MatchString(line), cppOpenSSLTLSPattern.MatchString(line), cppVerifyNonePattern.MatchString(line):
		return []core.Finding{env.NewFinding(support.FindingInput{RuleID: "security.cpp.insecure-tls", Level: "fail", Path: file, Line: lineNo, Column: 1, Message: "C++ TLS certificate or hostname verification is disabled"})}
	case cppShellPattern.MatchString(line):
		return []core.Finding{env.NewFinding(support.FindingInput{RuleID: "security.cpp.shell-execution", Level: "warn", Path: file, Line: lineNo, Column: 1, Message: "C++ shell execution primitive should be reviewed"})}
	case cppUnsafeCAPIPattern.MatchString(line):
		return []core.Finding{env.NewFinding(support.FindingInput{RuleID: "security.cpp.unsafe-c-api", Level: "warn", Path: file, Line: lineNo, Column: 1, Message: "unbounded C string API should be replaced with a bounds-aware operation"})}
	default:
		return nil
	}
}

func isCPPFile(path string) bool {
	return support.IsCPPPath(path, true)
}
