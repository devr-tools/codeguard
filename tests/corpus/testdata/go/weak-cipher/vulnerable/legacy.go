package fixtures

import (
	"crypto/des"
	"crypto/rc4"
)

func legacyBlock(key []byte) error {
	if _, err := des.NewCipher(key); err != nil {
		return err
	}
	_, err := rc4.NewCipher(key)
	return err
}
