package auth

import (
	"testing"

	"golang.org/x/crypto/bcrypt"
)

func TestHashPassword_UsesProvidedCost(t *testing.T) {
	h, err := HashPassword("correct horse battery staple", 12)
	if err != nil {
		t.Fatalf("HashPassword returned error: %v", err)
	}

	cost, err := bcrypt.Cost([]byte(h))
	if err != nil {
		t.Fatalf("bcrypt.Cost returned error: %v", err)
	}
	if cost != 12 {
		t.Fatalf("unexpected bcrypt cost: got %d, want %d", cost, 12)
	}
}

func TestHashPassword_InvalidCost(t *testing.T) {
	if _, err := HashPassword("pw", 0); err == nil {
		t.Fatalf("expected error for invalid cost")
	}
}
