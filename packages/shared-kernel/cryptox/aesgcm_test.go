package cryptox

import (
	"strings"
	"testing"
)

func TestAESGCM_RoundTrip(t *testing.T) {
	key := make([]byte, 32)
	for i := 0; i < len(key); i++ {
		key[i] = byte(i)
	}

	c, err := NewAESGCM(key, "k1")
	if err != nil {
		t.Fatalf("NewAESGCM: %v", err)
	}

	plain := "secret-token-123"
	enc, err := c.EncryptString(plain)
	if err != nil {
		t.Fatalf("EncryptString: %v", err)
	}
	if strings.Contains(enc, plain) {
		t.Fatalf("encrypted payload should not contain plaintext")
	}

	dec, err := c.DecryptString(enc)
	if err != nil {
		t.Fatalf("DecryptString: %v", err)
	}
	if dec != plain {
		t.Fatalf("roundtrip mismatch: got %q want %q", dec, plain)
	}
}

func TestAESGCM_NonDeterministic(t *testing.T) {
	key := make([]byte, 32)
	for i := 0; i < len(key); i++ {
		key[i] = byte(255 - i)
	}
	c, _ := NewAESGCM(key, "k1")

	plain := "same"
	enc1, err := c.EncryptString(plain)
	if err != nil {
		t.Fatalf("EncryptString: %v", err)
	}
	enc2, err := c.EncryptString(plain)
	if err != nil {
		t.Fatalf("EncryptString: %v", err)
	}
	if enc1 == enc2 {
		t.Fatalf("expected different ciphertexts for same plaintext (random IV)")
	}
}

func TestAESGCM_TamperFails(t *testing.T) {
	key := make([]byte, 32)
	for i := 0; i < len(key); i++ {
		key[i] = byte(i + 1)
	}
	c, _ := NewAESGCM(key, "k1")

	enc, _ := c.EncryptString("plain")
	enc = strings.Replace(enc, "A", "B", 1)
	if _, err := c.DecryptString(enc); err == nil {
		t.Fatalf("expected decrypt to fail for tampered payload")
	}
}
