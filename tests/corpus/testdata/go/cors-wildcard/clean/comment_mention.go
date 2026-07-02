package fixtures

// Returning Access-Control-Allow-Origin: * on credentialed endpoints leaks
// responses to any site.
func corsPolicyNote() string { return "use an allowlist" }
