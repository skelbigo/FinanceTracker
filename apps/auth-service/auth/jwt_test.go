package auth

import (
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

func TestJWTManager_GenerateAccessToken_includesStandardClaimsAndJTI(t *testing.T) {
	m := NewJWTManager("test-secret", 10*time.Minute)

	tok, err := m.GenerateAccessToken("user-123")
	if err != nil {
		t.Fatalf("GenerateAccessToken() error = %v", err)
	}

	var claims jwt.RegisteredClaims
	parsed, err := jwt.ParseWithClaims(tok, &claims, func(t *jwt.Token) (any, error) {
		return []byte("test-secret"), nil
	}, jwt.WithValidMethods([]string{jwt.SigningMethodHS256.Alg()}))
	if err != nil {
		t.Fatalf("ParseWithClaims() error = %v", err)
	}
	if !parsed.Valid {
		t.Fatalf("token is not valid")
	}

	if claims.Subject != "user-123" {
		t.Fatalf("sub = %q, want %q", claims.Subject, "user-123")
	}
	if claims.IssuedAt == nil {
		t.Fatalf("iat is nil")
	}
	if claims.ExpiresAt == nil {
		t.Fatalf("exp is nil")
	}
	if claims.ID == "" {
		t.Fatalf("jti (claims.ID) is empty")
	}
}
