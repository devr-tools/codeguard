package security

import (
	"path/filepath"
	"strings"

	"github.com/devr-tools/codeguard/internal/codeguard/checks/support"
	"github.com/devr-tools/codeguard/internal/codeguard/core"
)

// Every pattern below is a gatedPattern: a cheap literal pre-check runs before
// the regex, because these run on every line of a scanned file in the matching
// language. See security_line_gates.go for the superset invariant.
var (
	rustShellPattern = newGatedPattern("rust.shell-execution.literal",
		`\b(?:std::process::)?Command::new\s*\(\s*"(?:sh|bash|zsh|fish|cmd|powershell|pwsh)"`,
		exactGate("Command::new"))
	rustShellCodePattern = newGatedPattern("rust.shell-execution.call",
		`\b(?:std::process::)?Command::new\s*\(`,
		exactGate("Command::new"))
	rustTLSPattern = newGatedPattern("rust.insecure-tls",
		`\bdanger_accept_invalid_(?:certs|hostnames)\s*\(\s*true\s*\)`,
		exactGate("danger_accept_invalid_"))
	javaShellPattern = newGatedPattern("java.shell-execution",
		`\b(?:Runtime\.getRuntime\(\)\.exec|new\s+ProcessBuilder)\s*\(`,
		exactGate("Runtime.getRuntime", "ProcessBuilder"))
	javaTLSPattern = newGatedPattern("java.insecure-tls",
		`\b(?:NoopHostnameVerifier\.INSTANCE|ALLOW_ALL_HOSTNAME_VERIFIER|TrustAllStrategy\.INSTANCE)\b|setHostnameVerifier\s*\(.*->\s*true`,
		exactGate("NoopHostnameVerifier", "ALLOW_ALL_HOSTNAME_VERIFIER", "TrustAllStrategy", "setHostnameVerifier"))
	csharpShellPattern = newGatedPattern("csharp.shell-execution",
		`\b(?:Process\.Start|new\s+ProcessStartInfo)\s*\(`,
		exactGate("Process.Start", "ProcessStartInfo"))
	csharpTLSPattern = newGatedPattern("csharp.insecure-tls",
		`\bDangerousAcceptAnyServerCertificateValidator\b|ServerCertificateCustomValidationCallback\s*=\s*[^;=]*=>\s*true`,
		exactGate("DangerousAcceptAnyServerCertificateValidator", "ServerCertificateCustomValidationCallback"))
	rubyShellPattern = newGatedPattern("ruby.shell-execution",
		`\b(?:system|exec|spawn)\s*\(|\bOpen3\.(?:capture2|capture2e|capture3|pipeline|pipeline_r|pipeline_rw|pipeline_start|popen2|popen2e|popen3)\s*\(`,
		exactGate("system", "exec", "spawn", "Open3."))
	rubyTLSPattern = newGatedPattern("ruby.insecure-tls",
		`\bVERIFY_NONE\b`,
		exactGate("VERIFY_NONE"))
	rubyEvalPattern = newGatedPattern("ruby.dynamic-code",
		`\b(?:eval|instance_eval|class_eval)\s*\(`,
		exactGate("eval"))
)

func appendAdditionalLanguageLineFindings(env support.Context, file string, lineNo int, line sourceLine) []core.Finding {
	switch {
	case isRustFile(file):
		return appendRustLineFindings(env, file, lineNo, line)
	case isCPPFile(file):
		return appendCPPLineFindings(env, file, lineNo, line)
	case isJavaFile(file):
		return appendJavaLineFindings(env, file, lineNo, line)
	case isCSharpFile(file):
		return appendCSharpLineFindings(env, file, lineNo, line)
	case isRubyFile(file):
		return appendRubyLineFindings(env, file, lineNo, line)
	default:
		return nil
	}
}

// appendRustLineFindings matches the call structure on the masked line, then
// reads the interpreter name from the raw line since string contents are
// blanked by masking.
func appendRustLineFindings(env support.Context, file string, lineNo int, line sourceLine) []core.Finding {
	switch {
	case rustTLSPattern.matchString(line.masked, line.ascii):
		return []core.Finding{env.NewFinding(support.FindingInput{RuleID: "security.rust.insecure-tls", Level: "fail", Path: file, Line: lineNo, Column: 1, Message: "Rust TLS verification is disabled"})}
	case rustShellCodePattern.matchString(line.masked, line.ascii) && rustShellPattern.matchString(line.raw, line.ascii):
		return []core.Finding{env.NewFinding(support.FindingInput{RuleID: "security.rust.shell-execution", Level: "warn", Path: file, Line: lineNo, Column: 1, Message: "Rust shell execution primitive should be reviewed"})}
	default:
		return nil
	}
}

func appendJavaLineFindings(env support.Context, file string, lineNo int, source sourceLine) []core.Finding {
	line := source.masked
	switch {
	case javaTLSPattern.matchString(line, source.ascii):
		return []core.Finding{env.NewFinding(support.FindingInput{RuleID: "security.java.insecure-tls", Level: "fail", Path: file, Line: lineNo, Column: 1, Message: "Java TLS verification is disabled"})}
	case javaShellPattern.matchString(line, source.ascii):
		return []core.Finding{env.NewFinding(support.FindingInput{RuleID: "security.java.shell-execution", Level: "warn", Path: file, Line: lineNo, Column: 1, Message: "Java shell execution primitive should be reviewed"})}
	default:
		return nil
	}
}

func appendCSharpLineFindings(env support.Context, file string, lineNo int, source sourceLine) []core.Finding {
	line := source.masked
	switch {
	case csharpTLSPattern.matchString(line, source.ascii):
		return []core.Finding{env.NewFinding(support.FindingInput{RuleID: "security.csharp.insecure-tls", Level: "fail", Path: file, Line: lineNo, Column: 1, Message: "C# TLS verification is disabled"})}
	case csharpShellPattern.matchString(line, source.ascii):
		return []core.Finding{env.NewFinding(support.FindingInput{RuleID: "security.csharp.shell-execution", Level: "warn", Path: file, Line: lineNo, Column: 1, Message: "C# shell execution primitive should be reviewed"})}
	default:
		return nil
	}
}

func appendRubyLineFindings(env support.Context, file string, lineNo int, source sourceLine) []core.Finding {
	line := source.raw
	switch {
	case rubyTLSPattern.matchString(line, source.ascii):
		return []core.Finding{env.NewFinding(support.FindingInput{RuleID: "security.ruby.insecure-tls", Level: "fail", Path: file, Line: lineNo, Column: 1, Message: "Ruby TLS verification is disabled"})}
	case rubyEvalPattern.matchString(line, source.ascii):
		return []core.Finding{env.NewFinding(support.FindingInput{RuleID: "security.ruby.dynamic-code", Level: "warn", Path: file, Line: lineNo, Column: 1, Message: "dynamic code execution should be reviewed"})}
	case rubyShellPattern.matchString(line, source.ascii):
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
