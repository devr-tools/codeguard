package security

import (
	"path/filepath"
	"regexp"
	"strings"

	"github.com/devr-tools/codeguard/internal/codeguard/checks/support"
	"github.com/devr-tools/codeguard/internal/codeguard/core"
)

var (
	rustShellPattern     = regexp.MustCompile(`\b(?:std::process::)?Command::new\s*\(\s*"(?:sh|bash|zsh|fish|cmd|powershell|pwsh)"`)
	rustShellCodePattern = regexp.MustCompile(`\b(?:std::process::)?Command::new\s*\(`)
	rustTLSPattern       = regexp.MustCompile(`\bdanger_accept_invalid_(?:certs|hostnames)\s*\(\s*true\s*\)`)
	javaShellPattern     = regexp.MustCompile(`\b(?:Runtime\.getRuntime\(\)\.exec|new\s+ProcessBuilder)\s*\(`)
	javaTLSPattern       = regexp.MustCompile(`\b(?:NoopHostnameVerifier\.INSTANCE|ALLOW_ALL_HOSTNAME_VERIFIER|TrustAllStrategy\.INSTANCE)\b|setHostnameVerifier\s*\(.*->\s*true`)
	csharpShellPattern   = regexp.MustCompile(`\b(?:Process\.Start|new\s+ProcessStartInfo)\s*\(`)
	csharpTLSPattern     = regexp.MustCompile(`\bDangerousAcceptAnyServerCertificateValidator\b|ServerCertificateCustomValidationCallback\s*=\s*[^;=]*=>\s*true`)
	rubyShellPattern     = regexp.MustCompile(`\b(?:system|exec|spawn)\s*\(|\bOpen3\.(?:capture2|capture2e|capture3|pipeline|pipeline_r|pipeline_rw|pipeline_start|popen2|popen2e|popen3)\s*\(`)
	rubyTLSPattern       = regexp.MustCompile(`\bVERIFY_NONE\b`)
	rubyEvalPattern      = regexp.MustCompile(`\b(?:eval|instance_eval|class_eval)\s*\(`)
)

func appendAdditionalLanguageLineFindings(env support.Context, file string, lineNo int, raw string, masked string) []core.Finding {
	switch {
	case isRustFile(file):
		return appendRustLineFindings(env, file, lineNo, raw, masked)
	case isJavaFile(file):
		return appendJavaLineFindings(env, file, lineNo, masked)
	case isCSharpFile(file):
		return appendCSharpLineFindings(env, file, lineNo, masked)
	case isRubyFile(file):
		return appendRubyLineFindings(env, file, lineNo, raw)
	default:
		return nil
	}
}

// appendRustLineFindings matches the call structure on the masked line, then
// reads the interpreter name from the raw line since string contents are
// blanked by masking.
func appendRustLineFindings(env support.Context, file string, lineNo int, raw string, masked string) []core.Finding {
	switch {
	case rustTLSPattern.MatchString(masked):
		return []core.Finding{env.NewFinding(support.FindingInput{RuleID: "security.rust.insecure-tls", Level: "fail", Path: file, Line: lineNo, Column: 1, Message: "Rust TLS verification is disabled"})}
	case rustShellCodePattern.MatchString(masked) && rustShellPattern.MatchString(raw):
		return []core.Finding{env.NewFinding(support.FindingInput{RuleID: "security.rust.shell-execution", Level: "warn", Path: file, Line: lineNo, Column: 1, Message: "Rust shell execution primitive should be reviewed"})}
	default:
		return nil
	}
}

func appendJavaLineFindings(env support.Context, file string, lineNo int, line string) []core.Finding {
	switch {
	case javaTLSPattern.MatchString(line):
		return []core.Finding{env.NewFinding(support.FindingInput{RuleID: "security.java.insecure-tls", Level: "fail", Path: file, Line: lineNo, Column: 1, Message: "Java TLS verification is disabled"})}
	case javaShellPattern.MatchString(line):
		return []core.Finding{env.NewFinding(support.FindingInput{RuleID: "security.java.shell-execution", Level: "warn", Path: file, Line: lineNo, Column: 1, Message: "Java shell execution primitive should be reviewed"})}
	default:
		return nil
	}
}

func appendCSharpLineFindings(env support.Context, file string, lineNo int, line string) []core.Finding {
	switch {
	case csharpTLSPattern.MatchString(line):
		return []core.Finding{env.NewFinding(support.FindingInput{RuleID: "security.csharp.insecure-tls", Level: "fail", Path: file, Line: lineNo, Column: 1, Message: "C# TLS verification is disabled"})}
	case csharpShellPattern.MatchString(line):
		return []core.Finding{env.NewFinding(support.FindingInput{RuleID: "security.csharp.shell-execution", Level: "warn", Path: file, Line: lineNo, Column: 1, Message: "C# shell execution primitive should be reviewed"})}
	default:
		return nil
	}
}

func appendRubyLineFindings(env support.Context, file string, lineNo int, line string) []core.Finding {
	switch {
	case rubyTLSPattern.MatchString(line):
		return []core.Finding{env.NewFinding(support.FindingInput{RuleID: "security.ruby.insecure-tls", Level: "fail", Path: file, Line: lineNo, Column: 1, Message: "Ruby TLS verification is disabled"})}
	case rubyEvalPattern.MatchString(line):
		return []core.Finding{env.NewFinding(support.FindingInput{RuleID: "security.ruby.dynamic-code", Level: "warn", Path: file, Line: lineNo, Column: 1, Message: "dynamic code execution should be reviewed"})}
	case rubyShellPattern.MatchString(line):
		return []core.Finding{env.NewFinding(support.FindingInput{RuleID: "security.ruby.shell-execution", Level: "warn", Path: file, Line: lineNo, Column: 1, Message: "Ruby shell execution primitive should be reviewed"})}
	default:
		return nil
	}
}

func isRustFile(path string) bool {
	return strings.EqualFold(filepath.Ext(path), ".rs")
}

func isJavaFile(path string) bool {
	return strings.EqualFold(filepath.Ext(path), ".java")
}

func isCSharpFile(path string) bool {
	return strings.EqualFold(filepath.Ext(path), ".cs")
}

func isRubyFile(path string) bool {
	return strings.EqualFold(filepath.Ext(path), ".rb")
}
