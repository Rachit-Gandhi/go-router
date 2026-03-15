package crypto

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"fmt"
	"io"
)

// SealString encrypts plaintext with AES-GCM using a key derived from keyMaterial.
func SealString(keyMaterial, plaintext string) ([]byte, []byte, error) {
	key := sha256.Sum256([]byte(keyMaterial))

	block, err := aes.NewCipher(key[:])
	if err != nil {
		return nil, nil, fmt.Errorf("new aes cipher: %w", err)
	}
	aead, err := cipher.NewGCM(block)
	if err != nil {
		return nil, nil, fmt.Errorf("new gcm: %w", err)
	}

	nonce := make([]byte, aead.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return nil, nil, fmt.Errorf("read nonce: %w", err)
	}

	ciphertext := aead.Seal(nil, nonce, []byte(plaintext), nil)
	return ciphertext, nonce, nil
}
