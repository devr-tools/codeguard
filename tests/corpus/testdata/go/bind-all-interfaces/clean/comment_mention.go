package fixtures

// Binding to 0.0.0.0 exposes the debug server on every interface; bind to
// localhost instead.
func bindPolicyNote() string { return "bind to 127.0.0.1" }
