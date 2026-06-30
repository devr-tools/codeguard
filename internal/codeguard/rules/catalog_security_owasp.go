package rules

import "github.com/devr-tools/codeguard/internal/codeguard/core"

// securityRuleOWASP maps each security rule to its OWASP Top 10 (2021) category.
//
// Rules whose category depends on an external tool (security.command-check) are
// intentionally omitted: their category cannot be known statically.
var securityRuleOWASP = map[string]core.OWASPCategory{
	// Secrets and key material.
	"security.hardcoded-credential": core.OWASPA07AuthFailures,
	"security.hardcoded-secret":     core.OWASPA07AuthFailures,
	"security.high-entropy-string":  core.OWASPA07AuthFailures,
	"security.private-key":          core.OWASPA02CryptographicFailures,

	// Transport security.
	"security.insecure-tls":            core.OWASPA02CryptographicFailures,
	"security.typescript.insecure-tls": core.OWASPA02CryptographicFailures,
	"security.javascript.insecure-tls": core.OWASPA02CryptographicFailures,
	"security.python.insecure-tls":     core.OWASPA02CryptographicFailures,
	"security.rust.insecure-tls":       core.OWASPA02CryptographicFailures,
	"security.java.insecure-tls":       core.OWASPA02CryptographicFailures,
	"security.csharp.insecure-tls":     core.OWASPA02CryptographicFailures,
	"security.ruby.insecure-tls":       core.OWASPA02CryptographicFailures,

	// Command / code injection.
	"security.shell-execution":              core.OWASPA03Injection,
	"security.typescript.shell-execution":   core.OWASPA03Injection,
	"security.javascript.shell-execution":   core.OWASPA03Injection,
	"security.python.shell-execution":       core.OWASPA03Injection,
	"security.rust.shell-execution":         core.OWASPA03Injection,
	"security.java.shell-execution":         core.OWASPA03Injection,
	"security.csharp.shell-execution":       core.OWASPA03Injection,
	"security.ruby.shell-execution":         core.OWASPA03Injection,
	"security.typescript.dynamic-code":      core.OWASPA03Injection,
	"security.javascript.dynamic-code":      core.OWASPA03Injection,
	"security.python.dynamic-code":          core.OWASPA03Injection,
	"security.ruby.dynamic-code":            core.OWASPA03Injection,
	"security.typescript.vm-dynamic-code":   core.OWASPA03Injection,
	"security.javascript.vm-dynamic-code":   core.OWASPA03Injection,
	"security.typescript.string-timer-code": core.OWASPA03Injection,
	"security.javascript.string-timer-code": core.OWASPA03Injection,

	// Cross-site scripting (XSS is under A03:2021 Injection).
	"security.typescript.unsafe-html-sink": core.OWASPA03Injection,
	"security.javascript.unsafe-html-sink": core.OWASPA03Injection,

	// Untrusted-input taint flows reaching dangerous sinks.
	"security.typescript.taint-flow":           core.OWASPA03Injection,
	"security.javascript.taint-flow":           core.OWASPA03Injection,
	"security.typescript.untrusted-input-flow": core.OWASPA03Injection,
	"security.javascript.untrusted-input-flow": core.OWASPA03Injection,
	"security.taint.go":                        core.OWASPA03Injection,
	"security.taint.python":                    core.OWASPA03Injection,

	// Cross-origin message access control.
	"security.typescript.postmessage-wildcard": core.OWASPA01BrokenAccessControl,
	"security.javascript.postmessage-wildcard": core.OWASPA01BrokenAccessControl,

	// Known-vulnerable dependencies.
	"security.govulncheck": core.OWASPA06VulnerableComponents,

	// Security misconfiguration (A05).
	"security.cors-wildcard":       core.OWASPA05SecurityMisconfiguration,
	"security.debug-enabled":       core.OWASPA05SecurityMisconfiguration,
	"security.bind-all-interfaces": core.OWASPA05SecurityMisconfiguration,
	"security.dockerfile-root":     core.OWASPA05SecurityMisconfiguration,

	// Cryptographic failures (A02).
	"security.weak-hash":   core.OWASPA02CryptographicFailures,
	"security.weak-cipher": core.OWASPA02CryptographicFailures,

	// Software and data integrity failures (A08).
	"security.insecure-deserialization": core.OWASPA08IntegrityFailures,

	// Server-side request forgery (A10).
	"security.ssrf.go":     core.OWASPA10SSRF,
	"security.ssrf.python": core.OWASPA10SSRF,
}

// withSecurityOWASP returns a copy of catalog with OWASP Top 10 categories
// applied to the security rules in securityRuleOWASP. It is invoked from the
// `catalog` var initializer so the mapping is baked into every read path
// (Catalog, RuleCatalogForConfig, SARIF) regardless of init() ordering.
func withSecurityOWASP(catalog map[string]core.RuleMetadata) map[string]core.RuleMetadata {
	for id, category := range securityRuleOWASP {
		if rule, ok := catalog[id]; ok {
			rule.OWASPCategory = category
			catalog[id] = rule
		}
	}
	return catalog
}
