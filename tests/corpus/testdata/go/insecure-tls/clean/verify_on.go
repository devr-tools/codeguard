package fixtures

import "crypto/tls"

func strictTLSConfig() *tls.Config {
	return &tls.Config{
		InsecureSkipVerify: false,
		MinVersion:         tls.VersionTLS13,
	}
}

func tunableTLSConfig(allowInsecure bool) *tls.Config {
	return &tls.Config{InsecureSkipVerify: allowInsecure}
}
