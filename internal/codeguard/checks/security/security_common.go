package security

import (
	"path/filepath"
	"regexp"
	"strings"

	"github.com/devr-tools/codeguard/internal/codeguard/checks/support"
	"github.com/devr-tools/codeguard/internal/codeguard/core"
)

var (
	secretPattern            = regexp.MustCompile(`(?i)(secret|token|api[_-]?key|password)\s*[:=]\s*["'][^"']{8,}["']`)
	privateKeyPattern        = regexp.MustCompile(`-----BEGIN [A-Z ]*PRIVATE KEY-----`)
	pythonShellPattern       = regexp.MustCompile(`\bsubprocess\.(?:run|Popen|call|check_call|check_output|getoutput|getstatusoutput)\s*\(.*shell\s*=\s*True`)
	pythonSystemPattern      = regexp.MustCompile(`\bos\.system\s*\(`)
	pythonEvalPattern        = regexp.MustCompile(`\b(?:eval|exec)\s*\(`)
	pythonInsecureTLSPattern = regexp.MustCompile(`\bverify\s*=\s*False\b|\bssl\._create_unverified_context\s*\(`)
)

func findingsForFile(env support.Context, file string, data []byte) []core.Finding {
	findings := make([]core.Finding, 0)
	source := strings.ReplaceAll(string(data), "\r\n", "\n")
	lines := strings.Split(source, "\n")
	maskedLines := strings.Split(maskedSourceForFile(file, source), "\n")

	for idx, line := range lines {
		lineNo := idx + 1
		findings = append(findings, appendCommonLineFindings(env, file, lineNo, line)...)
		findings = append(findings, appendLanguageLineFindings(env, file, lineNo, line, maskedLines[idx])...)
		findings = append(findings, appendOWASPExtraLineFindings(env, file, lineNo, line, maskedLines[idx])...)
	}
	if isTypeScriptFile(file) {
		findings = append(findings, typeScriptFindingsForFile(env, file, source)...)
	}
	findings = append(findings, taintFindingsForFile(env, file, source)...)
	return findings
}

// maskedSourceForFile blanks comments and string contents for languages with
// a structured lexer, so security line patterns cannot match inside them.
// Masking is byte-for-byte, so line numbers are preserved.
func maskedSourceForFile(file string, source string) string {
	switch {
	case isPythonFile(file):
		return support.MaskPythonSource(source)
	case isRustFile(file):
		return support.MaskCLikeSource(source, support.CLikeRust)
	case isJavaFile(file), isCSharpFile(file):
		return support.MaskCLikeSource(source, support.CLikeJava)
	default:
		return source
	}
}

func appendCommonLineFindings(env support.Context, file string, lineNo int, line string) []core.Finding {
	switch {
	case secretPattern.MatchString(line):
		return []core.Finding{env.NewFinding(support.FindingInput{RuleID: "security.hardcoded-secret", Level: "fail", Path: file, Line: lineNo, Column: 1, Message: "possible hardcoded secret detected"})}
	case privateKeyPattern.MatchString(line):
		return []core.Finding{env.NewFinding(support.FindingInput{RuleID: "security.private-key", Level: "fail", Path: file, Line: lineNo, Column: 1, Message: "private key material detected"})}
	case strings.Contains(line, "InsecureSkipVerify: true"):
		return []core.Finding{env.NewFinding(support.FindingInput{RuleID: "security.insecure-tls", Level: "fail", Path: file, Line: lineNo, Column: 1, Message: "InsecureSkipVerify is enabled"})}
	case strings.Contains(line, "exec.Command(") || strings.Contains(line, "os/exec"):
		return []core.Finding{env.NewFinding(support.FindingInput{RuleID: "security.shell-execution", Level: "warn", Path: file, Line: lineNo, Column: 1, Message: "shell execution primitive should be reviewed"})}
	default:
		return nil
	}
}

// appendLanguageLineFindings matches structural patterns against the masked
// line so comments and string contents cannot trigger findings, while
// patterns that must read literal values receive the raw line.
func appendLanguageLineFindings(env support.Context, file string, lineNo int, raw string, masked string) []core.Finding {
	findings := make([]core.Finding, 0, 3)
	if isTypeScriptFile(file) {
		findings = append(findings, appendTypeScriptLineFindings(env, file, lineNo, raw)...)
	}
	if isPythonFile(file) {
		findings = append(findings, appendPythonLineFindings(env, file, lineNo, masked)...)
	}
	findings = append(findings, appendAdditionalLanguageLineFindings(env, file, lineNo, raw, masked)...)
	return findings
}

func appendPythonLineFindings(env support.Context, file string, lineNo int, line string) []core.Finding {
	switch {
	case pythonInsecureTLSPattern.MatchString(line):
		return []core.Finding{env.NewFinding(support.FindingInput{RuleID: "security.python.insecure-tls", Level: "fail", Path: file, Line: lineNo, Column: 1, Message: "Python TLS verification is disabled"})}
	case pythonShellPattern.MatchString(line) || pythonSystemPattern.MatchString(line):
		return []core.Finding{env.NewFinding(support.FindingInput{RuleID: "security.python.shell-execution", Level: "warn", Path: file, Line: lineNo, Column: 1, Message: "Python shell execution primitive should be reviewed"})}
	case pythonEvalPattern.MatchString(line):
		return []core.Finding{env.NewFinding(support.FindingInput{RuleID: "security.python.dynamic-code", Level: "warn", Path: file, Line: lineNo, Column: 1, Message: "dynamic code execution should be reviewed"})}
	default:
		return nil
	}
}

func isPythonFile(path string) bool {
	return strings.EqualFold(filepath.Ext(path), ".py")
}
