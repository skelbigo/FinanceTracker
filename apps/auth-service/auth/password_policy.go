package auth

import (
	"errors"
	"fmt"
	"strings"
	"unicode/utf8"
)

var ErrWeakPassword = errors.New("weak password")

type PasswordPolicyError struct {
	Msg string
}

func (e PasswordPolicyError) Error() string { return e.Msg }
func (e PasswordPolicyError) Unwrap() error { return ErrWeakPassword }

const (
	defaultPasswordMinChars = 10
	defaultPasswordMaxBytes = 72
)

func ValidatePasswordPolicy(password string) error {
	if password == "" {
		return PasswordPolicyError{Msg: "password is required"}
	}

	if len([]byte(password)) > defaultPasswordMaxBytes {
		return PasswordPolicyError{Msg: fmt.Sprintf("password is too long (max %d bytes)", defaultPasswordMaxBytes)}
	}

	if utf8.RuneCountInString(password) < defaultPasswordMinChars {
		return PasswordPolicyError{Msg: fmt.Sprintf("password must be at least %d characters", defaultPasswordMinChars)}
	}

	norm := strings.ToLower(strings.TrimSpace(password))
	if _, ok := commonPasswordDenyList[norm]; ok {
		return PasswordPolicyError{Msg: "password is too common"}
	}

	return nil
}

var commonPasswordDenyList = map[string]struct{}{
	"123456":           {},
	"123456789":        {},
	"12345678":         {},
	"12345":            {},
	"1234567890":       {},
	"987654321":        {},
	"987654":           {},
	"654321":           {},
	"123321":           {},
	"112233":           {},
	"qwerty":           {},
	"password":         {},
	"111111":           {},
	"11111111":         {},
	"000000":           {},
	"1234567":          {},
	"123123":           {},
	"121212":           {},
	"222222":           {},
	"333333":           {},
	"444444":           {},
	"555555":           {},
	"666666":           {},
	"7777777":          {},
	"888888":           {},
	"999999":           {},
	"iloveyou":         {},
	"admin":            {},
	"admin123":         {},
	"adminadmin":       {},
	"welcome":          {},
	"login":            {},
	"letmein":          {},
	"monkey":           {},
	"dragon":           {},
	"shadow":           {},
	"sunshine":         {},
	"princess":         {},
	"football":         {},
	"baseball":         {},
	"abc123":           {},
	"trustno1":         {},
	"1234":             {},
	"12345678910":      {},
	"1q2w3e4r":         {},
	"1qaz2wsx":         {},
	"qazwsx":           {},
	"passw0rd":         {},
	"password1":        {},
	"password2":        {},
	"password123":      {},
	"password!":        {},
	"pass1234":         {},
	"qwerty123":        {},
	"qwerty1":          {},
	"qwerty12":         {},
	"qwertyuiop":       {},
	"qwertyui":         {},
	"asdfghjkl":        {},
	"asdfasdf":         {},
	"zxcvbnm":          {},
	"test":             {},
	"tester":           {},
	"changeme":         {},
	"root":             {},
	"root123":          {},
	"guest":            {},
	"guest123":         {},
	"user":             {},
	"user123":          {},
	"useruser":         {},
	"default":          {},
	"secret":           {},
	"superman":         {},
	"batman":           {},
	"pokemon":          {},
	"master":           {},
	"master123":        {},
	"starwars":         {},
	"whatever":         {},
	"freedom":          {},
	"hello":            {},
	"love":             {},
	"123qwe":           {},
	"qwe123":           {},
	"admin!":           {},
	"пароль":           {},
	"пароль123":        {},
	"senha":            {},
	"passwordpassword": {},
}
