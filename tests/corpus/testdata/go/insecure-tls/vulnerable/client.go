package fixtures

import (
	"crypto/tls"
	"net/http"
)

func newInsecureClient() *http.Client {
	transport := &http.Transport{
		TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
	}
	return &http.Client{Transport: transport}
}
