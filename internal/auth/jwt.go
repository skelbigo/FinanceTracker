package auth

import (
	"fmt"
	"github.com/golang-jwt/jwt/v5"
	"time"
)

type JWTManager struct {
	secret []byte
	ttl    time.Duration
}

func NewJWTManager(secret string, accessTTL time.Duration) *JWTManager {
	return &JWTManager{
		secret: []byte(secret),
		ttl:    accessTTL,
	}
}

func (m *JWTManager) GenerateAccessToken(userID string, email string) (string, error) {
	now := time.Now()

	claims := jwt.RegisteredClaims{
		Subject:   userID,
		ExpiresAt: jwt.NewNumericDate(now.Add(m.ttl)),
		IssuedAt:  jwt.NewNumericDate(now),
	}

	t := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return t.SignedString(m.secret)
}

func (m *JWTManager) ParseAndValidate(tokenStr string) (userID string, err error) {
	var claims jwt.RegisteredClaims

	token, err := jwt.ParseWithClaims(tokenStr, &claims, func(t *jwt.Token) (any, error) {
		if t.Method != jwt.SigningMethodHS256 {
			return nil, fmt.Errorf("unexpected signing method: %v", t.Header["alg"])
		}
		return m.secret, nil
	})
	if err != nil {
		return "", err
	}
	if !token.Valid {
		return "", fmt.Errorf("invalid token")
	}

	if claims.Subject == "" {
		return "", fmt.Errorf("invalid token")
	}

	return claims.Subject, nil
}
