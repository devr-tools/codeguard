package fixtures

import "net/http"

// setAuthHeader hardcodes a fake bearer token, but passes it through the
// two-argument Header.Set form (comma between name and value) that the
// bearer pattern does not currently recognize.
func setAuthHeader(req *http.Request) {
	req.Header.Set("Authorization", "Bearer FAKE1111FAKE1111FAKE1111")
}
