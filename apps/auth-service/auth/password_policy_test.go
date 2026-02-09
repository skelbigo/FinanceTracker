package auth

import "testing"

func TestValidatePasswordPolicy_CommonPasswordsRejected(t *testing.T) {
	t.Parallel()

	cases := []string{
		"password",
		"Password",
		"  password  ",
		"123456",
		"qwerty",
	}

	for _, pw := range cases {
		if err := ValidatePasswordPolicy(pw); err == nil {
			t.Fatalf("expected error for %q", pw)
		}
	}
}

func TestValidatePasswordPolicy_MinLength(t *testing.T) {
	t.Parallel()

	if err := ValidatePasswordPolicy("short"); err == nil {
		t.Fatalf("expected error for short password")
	}
}

func TestValidatePasswordPolicy_MaxBytes(t *testing.T) {
	t.Parallel()

	pw := make([]byte, 73)
	for i := range pw {
		pw[i] = 'a'
	}
	if err := ValidatePasswordPolicy(string(pw)); err == nil {
		t.Fatalf("expected error for too long password")
	}
}

func TestValidatePasswordPolicy_AcceptsReasonablePassword(t *testing.T) {
	t.Parallel()

	if err := ValidatePasswordPolicy("correcthorseb"); err != nil {
		t.Fatalf("expected nil error, got: %v", err)
	}
}
