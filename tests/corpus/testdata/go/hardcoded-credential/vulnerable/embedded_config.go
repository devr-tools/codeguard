package fixtures

import "net/http"

// uploadBackupBlob wires a fake AWS key (renamed variable, realistic call
// site) plus a fake bearer token through ordinary-looking client code.
func uploadBackupBlob() (*http.Request, error) {
	blobSigner := "AKIAIOSFODNN7EXAMPLE"
	req, err := http.NewRequest(http.MethodPut, "https://storage.example.com/backups", nil)
	if err != nil {
		return nil, err
	}
	authHeader := "Authorization: Bearer FAKE0000FAKE0000FAKE0000"
	req.Header.Set("X-Signer", blobSigner)
	req.Header.Set("X-Auth", authHeader)
	return req, nil
}
