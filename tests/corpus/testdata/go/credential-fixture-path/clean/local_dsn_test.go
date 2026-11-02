package fixtures

// A loopback credential in a test file. The host is unroutable, so the value
// cannot authenticate against anything real; path scope plus that value
// evidence together make it a diagnostic rather than a finding.
func localDSN() string {
	return "postgres://app:app_local_pw_7231@127.0.0.1:5432/app_local"
}
