package rules

import "github.com/devr-tools/codeguard/internal/codeguard/core"

// securityExtraCatalog holds the language-agnostic OWASP-gap rules added to
// close coverage for A05 (Security Misconfiguration), A02 (Cryptographic
// Failures), and A08 (Software and Data Integrity Failures). They are
// heuristic, text-based, repository-wide checks and default to "warn".
var securityExtraCatalog = map[string]core.RuleMetadata{
	"security.cors-wildcard": {
		ID:               "security.cors-wildcard",
		Section:          "Security",
		DefaultLevel:     "warn",
		ExecutionModel:   core.RuleExecutionModelLanguageAgnostic,
		LanguageCoverage: core.RepositoryWideRuleLanguageCoverage(),
		Title:            "Wildcard CORS origin",
		Description:      "Warns when Access-Control-Allow-Origin is set to the wildcard '*', which lets any site read cross-origin responses.",
		HowToFix:         "Reflect a validated allowlist of trusted origins instead of returning '*', especially for credentialed endpoints.",
	},
	"security.debug-enabled": {
		ID:               "security.debug-enabled",
		Section:          "Security",
		DefaultLevel:     "warn",
		ExecutionModel:   core.RuleExecutionModelLanguageAgnostic,
		LanguageCoverage: core.RepositoryWideRuleLanguageCoverage(),
		Title:            "Debug mode enabled",
		Description:      "Warns when a framework debug flag is enabled (e.g. debug=True), which can expose stack traces, consoles, or secrets in production.",
		HowToFix:         "Drive debug mode from environment configuration and ensure it is disabled in production builds.",
	},
	"security.bind-all-interfaces": {
		ID:               "security.bind-all-interfaces",
		Section:          "Security",
		DefaultLevel:     "warn",
		ExecutionModel:   core.RuleExecutionModelLanguageAgnostic,
		LanguageCoverage: core.RepositoryWideRuleLanguageCoverage(),
		Title:            "Service binds to all interfaces",
		Description:      "Warns when a service binds to 0.0.0.0, exposing it on every network interface.",
		HowToFix:         "Bind to a specific interface (e.g. 127.0.0.1) or restrict exposure with a firewall or network policy.",
	},
	"security.dockerfile-root": {
		ID:               "security.dockerfile-root",
		Section:          "Security",
		DefaultLevel:     "warn",
		ExecutionModel:   core.RuleExecutionModelLanguageAgnostic,
		LanguageCoverage: core.RepositoryWideRuleLanguageCoverage(),
		Title:            "Container runs as root",
		Description:      "Warns when a Dockerfile explicitly sets USER root, so the container process runs with root privileges.",
		HowToFix:         "Add a non-root USER instruction and run the container as an unprivileged user.",
	},
	"security.weak-hash": {
		ID:               "security.weak-hash",
		Section:          "Security",
		DefaultLevel:     "warn",
		ExecutionModel:   core.RuleExecutionModelLanguageAgnostic,
		LanguageCoverage: core.RepositoryWideRuleLanguageCoverage(),
		Title:            "Weak hash algorithm",
		Description:      "Warns when a broken hash algorithm (MD5 or SHA-1) is used, which is unsafe for signatures, integrity, or password storage.",
		HowToFix:         "Use SHA-256 or stronger for integrity, and a dedicated password hash (bcrypt, scrypt, or Argon2) for credentials.",
	},
	"security.weak-cipher": {
		ID:               "security.weak-cipher",
		Section:          "Security",
		DefaultLevel:     "warn",
		ExecutionModel:   core.RuleExecutionModelLanguageAgnostic,
		LanguageCoverage: core.RepositoryWideRuleLanguageCoverage(),
		Title:            "Weak or insecure cipher",
		Description:      "Warns when a weak cipher or mode is used (DES, 3DES, RC4, or ECB block mode).",
		HowToFix:         "Use AES-GCM or ChaCha20-Poly1305 with a unique nonce; avoid ECB mode and legacy ciphers.",
	},
	"security.insecure-deserialization": {
		ID:               "security.insecure-deserialization",
		Section:          "Security",
		DefaultLevel:     "warn",
		ExecutionModel:   core.RuleExecutionModelLanguageAgnostic,
		LanguageCoverage: core.RepositoryWideRuleLanguageCoverage(),
		Title:            "Insecure deserialization",
		Description:      "Warns when untrusted data may be deserialized through a dangerous API (pickle, yaml.load, Java readObject, Marshal.load, unserialize), which can lead to remote code execution.",
		HowToFix:         "Deserialize only trusted data, prefer safe loaders (e.g. yaml.safe_load), or use a data-only format such as JSON.",
	},
}
