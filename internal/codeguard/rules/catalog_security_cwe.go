package rules

import "github.com/devr-tools/codeguard/internal/codeguard/core"

// ruleCWE maps rules to the MITRE CWE weaknesses they detect. It is kept in a
// side table, like securityRuleOWASP, so catalog entries stay focused on
// behaviour and the weakness mapping can be reviewed in one place.
//
// Each id is the most specific CWE that matches every sink the rule reports.
// Rules that cover several sink classes (taint engines) list one CWE per
// class. Rules whose weakness depends on an external tool
// (security.command-check) or that report codeguard's own configuration
// (security.secrets-config) are intentionally absent.
var ruleCWE = map[string][]string{
	// Secrets and key material.
	"security.hardcoded-credential": {"CWE-798"},
	"security.hardcoded-secret":     {"CWE-798"},
	"security.high-entropy-string":  {"CWE-798"},
	"security.private-key":          {"CWE-321", "CWE-798"},

	// Transport security: certificate verification disabled.
	"security.insecure-tls":            {"CWE-295"},
	"security.typescript.insecure-tls": {"CWE-295"},
	"security.javascript.insecure-tls": {"CWE-295"},
	"security.python.insecure-tls":     {"CWE-295"},
	"security.cpp.insecure-tls":        {"CWE-295"},
	"security.rust.insecure-tls":       {"CWE-295"},
	"security.java.insecure-tls":       {"CWE-295"},
	"security.csharp.insecure-tls":     {"CWE-295"},
	"security.ruby.insecure-tls":       {"CWE-295"},

	// OS command injection.
	"security.shell-execution":            {"CWE-78"},
	"security.typescript.shell-execution": {"CWE-78"},
	"security.javascript.shell-execution": {"CWE-78"},
	"security.python.shell-execution":     {"CWE-78"},
	"security.cpp.shell-execution":        {"CWE-78"},
	"security.rust.shell-execution":       {"CWE-78"},
	"security.java.shell-execution":       {"CWE-78"},
	"security.csharp.shell-execution":     {"CWE-78"},
	"security.ruby.shell-execution":       {"CWE-78"},

	// Unbounded C string APIs (gets, strcpy, strcat, sprintf, vsprintf).
	"security.cpp.unsafe-c-api": {"CWE-120", "CWE-676"},

	// Dynamically evaluated code (eval injection).
	"security.typescript.dynamic-code":      {"CWE-95"},
	"security.javascript.dynamic-code":      {"CWE-95"},
	"security.python.dynamic-code":          {"CWE-95"},
	"security.ruby.dynamic-code":            {"CWE-95"},
	"security.typescript.vm-dynamic-code":   {"CWE-95"},
	"security.javascript.vm-dynamic-code":   {"CWE-95"},
	"security.typescript.string-timer-code": {"CWE-95"},
	"security.javascript.string-timer-code": {"CWE-95"},

	// Cross-site scripting.
	"security.typescript.unsafe-html-sink": {"CWE-79"},
	"security.javascript.unsafe-html-sink": {"CWE-79"},

	// Taint engines. The TypeScript/JavaScript engine models shell, SQL, HTML,
	// code-evaluation, and outbound-URL sinks.
	"security.typescript.taint-flow":           {"CWE-78", "CWE-79", "CWE-89", "CWE-94", "CWE-918"},
	"security.javascript.taint-flow":           {"CWE-78", "CWE-79", "CWE-89", "CWE-94", "CWE-918"},
	"security.typescript.untrusted-input-flow": {"CWE-78", "CWE-79", "CWE-89", "CWE-94", "CWE-918"},
	"security.javascript.untrusted-input-flow": {"CWE-78", "CWE-79", "CWE-89", "CWE-94", "CWE-918"},
	// Go: exec.Command, SQL query text, file paths, and template parsing.
	"security.taint.go": {"CWE-78", "CWE-89", "CWE-22", "CWE-1336"},
	// Python: os.system/subprocess, eval/exec, and SQL execute.
	"security.taint.python": {"CWE-78", "CWE-95", "CWE-89"},
	// C++: shell and process execution APIs.
	"security.taint.cpp": {"CWE-78"},

	// Server-side request forgery.
	"security.ssrf.go":     {"CWE-918"},
	"security.ssrf.python": {"CWE-918"},
	"security.ssrf.cpp":    {"CWE-918"},

	// Cross-origin postMessage to the wildcard origin.
	"security.typescript.postmessage-wildcard": {"CWE-201"},
	"security.javascript.postmessage-wildcard": {"CWE-201"},

	// Known-vulnerable dependencies.
	"security.govulncheck":               {"CWE-1395"},
	"supply_chain.vulnerable-dependency": {"CWE-1395"},

	// Security misconfiguration.
	"security.cors-wildcard":       {"CWE-942"},
	"security.debug-enabled":       {"CWE-489"},
	"security.bind-all-interfaces": {"CWE-1327"},
	"security.dockerfile-root":     {"CWE-250"},

	// Cryptographic failures.
	"security.weak-hash":   {"CWE-328"},
	"security.weak-cipher": {"CWE-327"},

	// Integrity and information exposure.
	"security.insecure-deserialization":   {"CWE-502"},
	"security.log-secret-exposure":        {"CWE-532"},
	"security.unsanitized-error-response": {"CWE-209"},

	// GitHub Actions workflow security.
	"ci.workflow-script-injection":      {"CWE-94"},
	"ci.workflow-dangerous-trigger":     {"CWE-829"},
	"ci.workflow-write-all-permissions": {"CWE-275"},
	"ci.workflow-missing-permissions":   {"CWE-275"},
	"ci.workflow-remote-script-pipe":    {"CWE-494"},
	"ci.workflow-fork-trigger-secrets":  {"CWE-829"},
}

// withRuleCWE returns catalog with the ruleCWE mapping applied. Like
// withSecurityOWASP it runs from the `catalog` var initializer so every read
// path sees the mapping regardless of init() ordering.
func withRuleCWE(catalog map[string]core.RuleMetadata) map[string]core.RuleMetadata {
	for id, cwes := range ruleCWE {
		if rule, ok := catalog[id]; ok {
			rule.CWE = append([]string(nil), cwes...)
			catalog[id] = rule
		}
	}
	return catalog
}
