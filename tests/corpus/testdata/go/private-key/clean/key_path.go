package fixtures

import "path/filepath"

func serverKeyPath(dir string) string {
	return filepath.Join(dir, "tls", "server.key")
}
