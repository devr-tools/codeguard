package rules

import "github.com/devr-tools/codeguard/internal/codeguard/core"

// securityExtraCatalog holds the language-agnostic OWASP-gap rules added to
// close coverage for A05 (Security Misconfiguration), A02 (Cryptographic
// Failures), A08 (Software and Data Integrity Failures), and A09 (Security
// Logging and Monitoring Failures). They are heuristic, text-based checks and
// default to "warn".
var securityExtraCatalog = map[string]core.RuleMetadata{
	"security.log-secret-exposure": {
		ID:               "security.log-secret-exposure",
		Section:          "Security",
		DefaultLevel:     "warn",
		ExecutionModel:   core.RuleExecutionModelLanguageAgnostic,
		LanguageCoverage: core.FixedRuleLanguageCoverage(core.RuleLanguageGo, core.RuleLanguagePython, core.RuleLanguageTypeScript, core.RuleLanguageJavaScript),
		Title:            "Secret-bearing value passed to a logging call",
		Description:      "Warns when a secret-bearing value appears inside the argument list of a logging call: Go log./logger./slog./zap./logrus. Print/Info/Error/Debug/Warn/Fatal/Panic variants; Python logging./logger./log. level methods and print; TS/JS console./logger./log. methods. Matching runs on masked source (comments and string contents blanked), so a secret word merely appearing in a comment or a plain format string never fires. On the call line it fires only when: (1) an argument identifier has a secret-named snake_case/camelCase component - password, passwd, secret, token, api_key/apikey, private_key, credential, authorization (whole components, so 'tokenizer' does not match; f-string and template-literal interpolations such as f\"{token}\" and `${token}` count); (2) a short whitespace-free string literal naming a secret is used as a structured-logging key (e.g. \"password\", value); (3) a string literal containing a secret keyword is concatenated with '+' to an expression (e.g. \"Authorization: Bearer \" + tok); or (4) a literal embeds '<keyword>=' or '<keyword>:' immediately followed by a string format directive (%s/%v/%q or '{'). \"token count: %d\" with a non-secret argument matches none of these.",
		HowToFix:         "Never log secret material. Log a redacted or derived value instead (length, hash fingerprint, last four characters) or drop the field entirely.",
		FixTemplate:      core.FixTemplate{Kind: guided, Text: "Log a redacted or derived value instead of the secret itself.\n\nBefore:\nlog.Printf(\"login ok token=%s\", token)\n\nAfter:\nlog.Printf(\"login ok token_sha256=%s\", fingerprint(token))\n// or drop the field entirely:\nlog.Printf(\"login ok for user %s\", userID)"},
	},
	"security.unsanitized-error-response": {
		ID:               "security.unsanitized-error-response",
		Section:          "Security",
		DefaultLevel:     "warn",
		ExecutionModel:   core.RuleExecutionModelLanguageAgnostic,
		LanguageCoverage: core.FixedRuleLanguageCoverage(core.RuleLanguageGo, core.RuleLanguagePython, core.RuleLanguageTypeScript, core.RuleLanguageJavaScript),
		Title:            "Raw error value written to HTTP response",
		Description:      "Warns when a raw error value is written directly into an HTTP response, leaking internal details to clients and starving server-side logs of the diagnostic detail incident response needs. Conservative single-line patterns, matched on masked source: Go http.Error(w, err.Error(), ...) and fmt.Fprint/Fprintf/Fprintln with a response-writer-named first argument (w, wr, rw, res, resp, rsp, writer) and a raw err argument; TS/JS res/resp/response .send/.json/.end called with an error-named identifier (err, error, e, ex), including res.status(...).send(err.stack || err.message) chains; Python return str(<exc>) or HttpResponse(str(<exc>)) on a line inside an except block, where <exc> is the block's 'as' alias. Errors passed to a logger but not written to the response do not fire.",
		HowToFix:         "Return a generic client-facing message and log the detailed error server-side (with request correlation) so clients learn nothing internal and incidents stay diagnosable.",
		FixTemplate:      core.FixTemplate{Kind: guided, Text: "Return a generic message to the client and log the detail server-side.\n\nBefore:\nhttp.Error(w, err.Error(), http.StatusInternalServerError)\n\nAfter:\nlog.Printf(\"handle %s: %v\", r.URL.Path, err)\nhttp.Error(w, \"internal server error\", http.StatusInternalServerError)"},
	},
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
