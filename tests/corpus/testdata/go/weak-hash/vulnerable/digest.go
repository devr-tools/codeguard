package fixtures

import (
	"crypto/md5"
	"crypto/sha1"
	"fmt"
)

func legacyChecksums(data []byte) (string, string) {
	m := md5.New()
	s := sha1.New()
	m.Write(data)
	s.Write(data)
	return fmt.Sprintf("%x", m.Sum(nil)), fmt.Sprintf("%x", s.Sum(nil))
}
