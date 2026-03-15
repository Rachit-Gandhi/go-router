package auth

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
)

// SessionClaims are persisted in the control-plane auth cookie.
type SessionClaims struct {
	OrgID          string `json:"org_id"`
	UserID         string `json:"user_id"`
	RefreshTokenID string `json:"refresh_token_id"`
	ExpiresAtUnix  int64  `json:"expires_at_unix"`
}

// SessionCodec encrypts/decrypts session claims.
type SessionCodec struct {
	aead cipher.AEAD
}

// NewSessionCodec creates a codec using the first 32 bytes of the provided key material.
func NewSessionCodec(keyMaterial string) (*SessionCodec, error) {
	key := sha256.Sum256([]byte(keyMaterial))
	block, err := aes.NewCipher(key[:])
	if err != nil {
		return nil, fmt.Errorf("new aes cipher: %w", err)
	}
	aead, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("new gcm: %w", err)
	}
	return &SessionCodec{aead: aead}, nil
}

// Seal encrypts session claims to a cookie-safe string.
func (s *SessionCodec) Seal(claims SessionClaims) (string, error) {
	plaintext, err := json.Marshal(claims)
	if err != nil {
		return "", fmt.Errorf("marshal claims: %w", err)
	}

	nonce := make([]byte, s.aead.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return "", fmt.Errorf("read nonce: %w", err)
	}

	ciphertext := s.aead.Seal(nil, nonce, plaintext, nil)
	out := append(nonce, ciphertext...)
	return base64.RawURLEncoding.EncodeToString(out), nil
}

// Open decrypts a cookie string back into session claims.
func (s *SessionCodec) Open(token string) (SessionClaims, error) {
	var claims SessionClaims

	raw, err := base64.RawURLEncoding.DecodeString(token)
	if err != nil {
		return claims, fmt.Errorf("decode token: %w", err)
	}
	nonceSize := s.aead.NonceSize()
	if len(raw) <= nonceSize {
		return claims, fmt.Errorf("token too short")
	}

	nonce := raw[:nonceSize]
	ciphertext := raw[nonceSize:]
	plaintext, err := s.aead.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		return claims, fmt.Errorf("decrypt token: %w", err)
	}
	if err := json.Unmarshal(plaintext, &claims); err != nil {
		return claims, fmt.Errorf("unmarshal claims: %w", err)
	}
	return claims, nil
}
