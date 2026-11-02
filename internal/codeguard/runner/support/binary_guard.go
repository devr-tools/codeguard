package support

import checkSupport "github.com/devr-tools/codeguard/internal/codeguard/checks/support"

// LooksBinary reports whether data appears to be object code or another
// non-text blob rather than source. It forwards to checks/support.LooksBinary,
// which documents where the guard is (and is deliberately not) applied.
func LooksBinary(data []byte) bool {
	return checkSupport.LooksBinary(data)
}
