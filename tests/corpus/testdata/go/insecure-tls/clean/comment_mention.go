package fixtures

// Code review checklist: reject any diff that sets InsecureSkipVerify: true
// outside of tests.
func tlsReviewChecklist() string { return "verify certificates" }
