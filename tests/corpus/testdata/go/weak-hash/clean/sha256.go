package fixtures

import (
	"crypto/sha256"
	"fmt"
)

func checksum(data []byte) string {
	digest := sha256.New()
	digest.Write(data)
	return fmt.Sprintf("%x", digest.Sum(nil))
}
