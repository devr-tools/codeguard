package fixtures

// Values AWS publishes in its own documentation as non-secrets. They match the
// provider token shapes but are world-readable by design, so a match can never
// be a leak and an attacker gains nothing by reusing one. The scanner records
// them as informational diagnostics on any path, never as findings.
const (
	docsAccessKeyID = "AKIAIOSFODNN7EXAMPLE"
	docsSessionKey  = "ASIAIOSFODNN7EXAMPLE"
)
