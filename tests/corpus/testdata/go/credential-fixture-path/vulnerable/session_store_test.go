package fixtures

// A production-looking credential that happens to live in a Go test file. The
// test path is only one piece of evidence, so the finding is demoted to the
// review tier but must stay visible: a real leaked credential does not stop
// being real because of the file it was committed in.
func sessionStoreDSN() string {
	return "postgres://svc_sessions:Qh7dRm2VtbLx9@sessions.prod.internal:5432/sessions"
}
