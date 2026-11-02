package security

import (
	"path/filepath"
	"strings"

	"github.com/devr-tools/codeguard/internal/codeguard/checks/support"
	"github.com/devr-tools/codeguard/internal/codeguard/core"
)

// Hardcoded secret/credential and private-key detection lives in
// security_secrets.go, which runs as a dedicated repository-wide pass so it also
// covers TypeScript/JavaScript targets.
//
// Every pattern below is a gatedPattern: a cheap literal pre-check runs before
// the regex, because these run on every line of every scanned file. See
// security_line_gates.go for the superset invariant the gates must hold to.
var (
	pythonShellPattern = newGatedPattern("python.shell-execution",
		`\bsubprocess\.(?:run|Popen|call|check_call|check_output|getoutput|getstatusoutput)\s*\(.*shell\s*=\s*True`,
		exactGate("subprocess."))
	pythonSystemPattern = newGatedPattern("python.os-system",
		`\bos\.system\s*\(`,
		exactGate("os.system"))
	pythonEvalPattern = newGatedPattern("python.dynamic-code",
		`\b(?:eval|exec)\s*\(`,
		exactGate("eval", "exec"))
	pythonInsecureTLSPattern = newGatedPattern("python.insecure-tls",
		`\bverify\s*=\s*False\b|\bssl\._create_unverified_context\s*\(`,
		exactGate("verify", "ssl._create_unverified_context"))

	// Line-scan forms of the Go-oriented base checks, matched against masked
	// source. They tolerate missing whitespace (InsecureSkipVerify:true) and
	// require a call shape for shell execution so import paths alone cannot
	// fire. NOTE: these run on every line of every NON-Go file too (goParsed is
	// false for them), which is why their gate matters as much as the Go one.
	goInsecureTLSLinePattern = newGatedPattern("go.insecure-tls",
		`\bInsecureSkipVerify\s*[:=]\s*true\b`,
		exactGate("InsecureSkipVerify"))
	goShellCallLinePattern = newGatedPattern("go.shell-execution",
		`\b(?:exec\.Command(?:Context)?|syscall\.Exec)\s*\(`,
		exactGate("exec.Command", "syscall.Exec"))
)

func findingsForFile(env support.Context, file string, data []byte) []core.Finding {
	findings := make([]core.Finding, 0)
	source := strings.ReplaceAll(string(data), "\r\n", "\n")
	lines := strings.Split(source, "\n")
	maskedLines := strings.Split(maskedSourceForFile(file, source), "\n")

	// Go files get AST-based detection for the base checks; the line scan
	// below is the fallback for files that do not parse.
	goParsed := false
	if isGoFile(file) {
		var goFindings []core.Finding
		goFindings, goParsed = goSecurityFindings(env, file, data)
		findings = append(findings, goFindings...)
	}

	for idx, raw := range lines {
		lineNo := idx + 1
		// One sourceLine per line: it carries the raw and masked forms together
		// with the ASCII flag the literal gates read, so the scan for non-ASCII
		// bytes is paid once per line rather than once per pattern.
		line := newSourceLine(raw, maskedLines[idx])
		if !goParsed {
			findings = append(findings, appendCommonLineFindings(env, file, lineNo, line)...)
		}
		findings = append(findings, appendLanguageLineFindings(env, file, lineNo, line)...)
		findings = append(findings, appendOWASPExtraLineFindings(env, file, lineNo, line)...)
	}
	if isCPPFile(file) {
		findings = append(findings, appendCPPFileFindings(env, file, strings.Join(maskedLines, "\n"))...)
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
	case isGoFile(file):
		return support.MaskCLikeSource(source, support.CLikeGo)
	case isPythonFile(file):
		return support.MaskPythonSource(source)
	case isRustFile(file):
		return support.MaskCLikeSource(source, support.CLikeRust)
	case isCPPFile(file):
		return support.MaskCLikeSource(source, support.CLikeCPP)
	case isJavaFile(file), isCSharpFile(file):
		return support.MaskCLikeSource(source, support.CLikeJava)
	default:
		return source
	}
}

// appendCommonLineFindings is the line-scan form of the Go-oriented base
// checks. For Go files it runs only when the file fails to parse (parseable
// files go through the AST pass in security_go_ast.go); it always receives the
// masked line, so comments and string literals cannot fire where a masker
// exists for the language.
func appendCommonLineFindings(env support.Context, file string, lineNo int, source sourceLine) []core.Finding {
	line := source.masked
	switch {
	case goInsecureTLSLinePattern.matchString(line, source.ascii):
		return []core.Finding{env.NewFinding(support.FindingInput{RuleID: "security.insecure-tls", Level: "fail", Path: file, Line: lineNo, Column: 1, Message: "InsecureSkipVerify is enabled"})}
	case goShellCallLinePattern.matchString(line, source.ascii):
		return []core.Finding{env.NewFinding(support.FindingInput{RuleID: "security.shell-execution", Level: "warn", Path: file, Line: lineNo, Column: 1, Message: "shell execution primitive should be reviewed"})}
	default:
		return nil
	}
}

// appendLanguageLineFindings matches structural patterns against the masked
// line so comments and string contents cannot trigger findings, while
// patterns that must read literal values receive the raw line.
//
// PERF: it returns a nil slice when nothing fires. The previous
// make([]core.Finding, 0, 3) allocated a three-element backing array for every
// line of every scanned file — ~769 MB of allocations across a seven-repository
// scan, almost all of it immediately discarded.
func appendLanguageLineFindings(env support.Context, file string, lineNo int, line sourceLine) []core.Finding {
	var findings []core.Finding
	if isTypeScriptFile(file) {
		findings = append(findings, appendTypeScriptLineFindings(env, file, lineNo, line)...)
	}
	if isPythonFile(file) {
		findings = append(findings, appendPythonLineFindings(env, file, lineNo, line)...)
	}
	return append(findings, appendAdditionalLanguageLineFindings(env, file, lineNo, line)...)
}

func appendPythonLineFindings(env support.Context, file string, lineNo int, source sourceLine) []core.Finding {
	line := source.masked
	switch {
	case pythonInsecureTLSPattern.matchString(line, source.ascii):
		return []core.Finding{env.NewFinding(support.FindingInput{RuleID: "security.python.insecure-tls", Level: "fail", Path: file, Line: lineNo, Column: 1, Message: "Python TLS verification is disabled"})}
	case pythonShellPattern.matchString(line, source.ascii) || pythonSystemPattern.matchString(line, source.ascii):
		return []core.Finding{env.NewFinding(support.FindingInput{RuleID: "security.python.shell-execution", Level: "warn", Path: file, Line: lineNo, Column: 1, Message: "Python shell execution primitive should be reviewed"})}
	case pythonEvalPattern.matchString(line, source.ascii):
		return []core.Finding{env.NewFinding(support.FindingInput{RuleID: "security.python.dynamic-code", Level: "warn", Path: file, Line: lineNo, Column: 1, Message: "dynamic code execution should be reviewed"})}
	default:
		return nil
	}
}

func isPythonFile(path string) bool {
	return strings.EqualFold(filepath.Ext(path), ".py")
}
