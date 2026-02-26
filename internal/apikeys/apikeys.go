package apikeys

import (
	"fmt"

	"github.com/alexedwards/argon2id"
	"github.com/dchest/uniuri"
)

const apiKeyPrefix = "go-"

func CreateApiKey() string {
	secureString := uniuri.NewLen(20)
	return apiKeyPrefix + secureString
}

func CreateApiKeyHash(apiKey string) (string, error) {
	hash, err := argon2id.CreateHash(apiKey, argon2id.DefaultParams)
	if err != nil {
		return "", fmt.Errorf("failed to create api key hash: %w", err)
	}
	return hash, nil
}

func VerifyApiKeyHash(apiKey, hash string) (bool, error) {
	valid, err := argon2id.ComparePasswordAndHash(apiKey, hash)
	if err != nil {
		return false, fmt.Errorf("failed to verify api key hash: %w", err)
	}
	return valid, nil
}
