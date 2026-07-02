package fixtures

// connectString uses an abbreviated identifier the name-based heuristic does
// not recognize, so this hardcoded value is currently missed.
func connectString() string {
	dbPass := "Zx9Qw3Rt7Yu1Io5P"
	return "user=app pass=" + dbPass
}
