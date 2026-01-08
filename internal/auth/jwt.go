package auth

import (
	"errors"
	"github.com/golang-jwt/jwt/v5"
	"time"
)

var ErrInvalidToken = errors.New("invalid token")
var ErrExpiredToken = errors.New("expired token")

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

func (m *JWTManager) GenerateAccessToken(userID string) (string, error) {
	now := time.Now()
	claims := jwt.RegisteredClaims{
		Subject:   userID,
		ExpiresAt: jwt.NewNumericDate(now.Add(m.ttl)),
		IssuedAt:  jwt.NewNumericDate(now),
	}

	return jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString(m.secret)
}

func (m *JWTManager) ParseAndValidate(tokenStr string) (string, error) {
	var claims jwt.RegisteredClaims

	token, err := jwt.ParseWithClaims(
		tokenStr,
		&claims,
		func(t *jwt.Token) (any, error) { return m.secret, nil },
		jwt.WithValidMethods([]string{jwt.SigningMethodHS256.Alg()}),
	)
	if err != nil {
		if errors.Is(err, jwt.ErrTokenExpired) {
			return "", ErrExpiredToken
		}
		return "", ErrInvalidToken
	}
	if !token.Valid || claims.Subject == "" {
		return "", ErrInvalidToken
	}
	return claims.Subject, nil
}
