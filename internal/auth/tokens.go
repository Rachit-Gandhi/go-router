package auth

import (
	"time"

	"github.com/golang-jwt/jwt/v4"
)

func (auth *AuthHandler) makeToken(userId string) (string, error) {
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
		"sub": userId,
		"exp": time.Now().Add(time.Duration(auth.TokenExpiry) * time.Hour).Unix(),
	})
	return token.SignedString([]byte(auth.JwtSecret))
}

func (auth *AuthHandler) validateToken(tokenString string) (string, error) {
	token, err := jwt.Parse(tokenString, func(token *jwt.Token) (interface{}, error) {
		return []byte(auth.JwtSecret), nil
	})
	if err != nil {
		return "", err
	}
	return token.Claims.(jwt.MapClaims)["sub"].(string), nil
}
