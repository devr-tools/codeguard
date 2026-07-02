package fixtures

import (
	"crypto/aes"
	"crypto/cipher"
)

func seal(key []byte, nonce []byte, payload []byte) ([]byte, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}
	return gcm.Seal(nil, nonce, payload, nil), nil
}
