package fixtures

import "crypto/tls"

func localProxyConfig() *tls.Config {
	trustEverything := &tls.Config{InsecureSkipVerify: true}
	return trustEverything
}
