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

	claims := jwt.MapClaims{
		"sub": userID,
		"exp": jwt.NewNumericDate(now.Add(m.ttl)),
		"iat": jwt.NewNumericDate(now),
	}
	if email != "" {
		claims["email"] = email
	}

	t := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	s, err := t.SignedString(m.secret)
	if err != nil {
		return "", err
	}
	return s, err
}

func (m *JWTManager) ParseAndValidate(tokenStr string) (userID string, err error) {
	token, err := jwt.Parse(tokenStr, func(t *jwt.Token) (any, error) {
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

	claims, ok := token.Claims.(jwt.MapClaims)
	if !ok {
		return "", fmt.Errorf("invalid claims")
	}

	sub, _ := claims["sub"].(string)
	if sub == "" {
		return "", fmt.Errorf("missing sub")
	}
	return sub, nil
}
