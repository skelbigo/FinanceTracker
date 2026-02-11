package cryptox

import (
	"errors"
	"testing"
)

func TestService_EncryptDecrypt_RoundTrip(t *testing.T) {
	key := make([]byte, 32)
	for i := 0; i < len(key); i++ {
		key[i] = byte(i)
	}

	svc, err := NewService(true, key, "k1")
	if err != nil {
		t.Fatalf("NewService: %v", err)
	}

	plain := "token-abc"
	stored, err := svc.EncryptForStorage(plain)
	if err != nil {
		t.Fatalf("EncryptForStorage: %v", err)
	}
	if stored == plain {
		t.Fatalf("expected stored value to be encrypted")
	}

	dec, err := svc.DecryptFromStorage(stored)
	if err != nil {
		t.Fatalf("DecryptFromStorage: %v", err)
	}
	if dec != plain {
		t.Fatalf("roundtrip mismatch: got %q want %q", dec, plain)
	}
}

func TestService_Disabled_PassThroughPlaintext(t *testing.T) {
	key := make([]byte, 32)
	for i := 0; i < len(key); i++ {
		key[i] = byte(7)
	}

	svc, err := NewService(false, key, "k1")
	if err != nil {
		t.Fatalf("NewService: %v", err)
	}

	plain := "not-encrypted"
	stored, err := svc.EncryptForStorage(plain)
	if err != nil {
		t.Fatalf("EncryptForStorage: %v", err)
	}
	if stored != plain {
		t.Fatalf("expected pass-through when disabled")
	}

	dec, err := svc.DecryptFromStorage(stored)
	if err != nil {
		t.Fatalf("DecryptFromStorage: %v", err)
	}
	if dec != plain {
		t.Fatalf("expected pass-through when disabled")
	}
}

func TestService_Disabled_CanDecryptPayloadIfKeyPresent(t *testing.T) {
	key := make([]byte, 32)
	for i := 0; i < len(key); i++ {
		key[i] = byte(i + 10)
	}

	enabled, _ := NewService(true, key, "k1")
	disabled, _ := NewService(false, key, "k1")

	stored, err := enabled.EncryptForStorage("secret")
	if err != nil {
		t.Fatalf("EncryptForStorage: %v", err)
	}

	dec, err := disabled.DecryptFromStorage(stored)
	if err != nil {
		t.Fatalf("DecryptFromStorage (disabled): %v", err)
	}
	if dec != "secret" {
		t.Fatalf("expected decrypt to work even when disabled, got %q", dec)
	}
}

func TestService_DecryptFailure_ReturnsErrDecryptFailed(t *testing.T) {
	key := make([]byte, 32)
	for i := 0; i < len(key); i++ {
		key[i] = byte(1)
	}

	svc, _ := NewService(true, key, "k1")
	stored, _ := svc.EncryptForStorage("secret")
	stored = stored + "x"

	_, err := svc.DecryptFromStorage(stored)
	if err == nil {
		t.Fatalf("expected error")
	}
	if !errors.Is(err, ErrDecryptFailed) {
		t.Fatalf("expected ErrDecryptFailed, got %v", err)
	}
}

func TestService_Keyring_DecryptOldKidAndEncryptWithActiveKid(t *testing.T) {
	keyOld := make([]byte, 32)
	keyNew := make([]byte, 32)
	for i := 0; i < 32; i++ {
		keyOld[i] = byte(i)
		keyNew[i] = byte(100 + i)
	}

	oldSvc, err := NewService(true, keyOld, "k1")
	if err != nil {
		t.Fatalf("NewService(old): %v", err)
	}
	storedOld, err := oldSvc.EncryptForStorage("secret")
	if err != nil {
		t.Fatalf("EncryptForStorage(old): %v", err)
	}
	if !contains(storedOld, `"kid":"k1"`) {
		t.Fatalf("expected old payload to have kid=k1, got: %s", storedOld)
	}

	keyring := map[string][]byte{"k1": keyOld, "k2": keyNew}
	rotatingSvc, err := NewServiceWithKeyring(true, "k2", keyring)
	if err != nil {
		t.Fatalf("NewServiceWithKeyring: %v", err)
	}

	dec, err := rotatingSvc.DecryptFromStorage(storedOld)
	if err != nil {
		t.Fatalf("DecryptFromStorage: %v", err)
	}
	if dec != "secret" {
		t.Fatalf("expected decrypt to work with old key, got %q", dec)
	}

	storedNew, err := rotatingSvc.EncryptForStorage("secret")
	if err != nil {
		t.Fatalf("EncryptForStorage(active): %v", err)
	}
	if !contains(storedNew, `"kid":"k2"`) {
		t.Fatalf("expected new payload to have kid=k2, got: %s", storedNew)
	}
}

func contains(s, sub string) bool { return len(s) >= len(sub) && (stringIndex(s, sub) >= 0) }

func stringIndex(s, sub string) int {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return i
		}
	}
	return -1
}
