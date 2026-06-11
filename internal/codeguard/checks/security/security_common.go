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

	for idx, line := range lines {
		lineNo := idx + 1
		findings = append(findings, appendCommonLineFindings(env, file, lineNo, line)...)
		findings = append(findings, appendLanguageLineFindings(env, file, lineNo, line)...)
	}
	if isTypeScriptFile(file) {
		findings = append(findings, typeScriptFindingsForFile(env, file, source)...)
	}
	return findings
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

func appendLanguageLineFindings(env support.Context, file string, lineNo int, line string) []core.Finding {
	findings := make([]core.Finding, 0, 3)
	if isTypeScriptFile(file) {
		findings = append(findings, appendTypeScriptLineFindings(env, file, lineNo, line)...)
	}
	if isPythonFile(file) {
		findings = append(findings, appendPythonLineFindings(env, file, lineNo, line)...)
	}
	findings = append(findings, appendAdditionalLanguageLineFindings(env, file, lineNo, line)...)
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
