package auth

import (
	"errors"
	"unicode"

	"golang.org/x/crypto/bcrypt"
)

// MinPasswordLength is enforced for both admin panel passwords and
// generated SSH account passwords.
const MinPasswordLength = 12

var ErrWeakPassword = errors.New("password must be at least 12 characters and include upper, lower, digit and symbol characters")

// ValidatePasswordStrength rejects anything that would make brute-forcing
// the panel or an SSH account trivial. This is intentionally stricter than
// most consumer sites because a compromised account here means shell access.
func ValidatePasswordStrength(pw string) error {
	if len(pw) < MinPasswordLength {
		return ErrWeakPassword
	}
	var hasUpper, hasLower, hasDigit, hasSymbol bool
	for _, r := range pw {
		switch {
		case unicode.IsUpper(r):
			hasUpper = true
		case unicode.IsLower(r):
			hasLower = true
		case unicode.IsDigit(r):
			hasDigit = true
		case unicode.IsPunct(r), unicode.IsSymbol(r):
			hasSymbol = true
		}
	}
	if !(hasUpper && hasLower && hasDigit && hasSymbol) {
		return ErrWeakPassword
	}
	return nil
}

func HashPassword(pw string) (string, error) {
	b, err := bcrypt.GenerateFromPassword([]byte(pw), bcrypt.DefaultCost)
	if err != nil {
		return "", err
	}
	return string(b), nil
}

func CheckPassword(hash, pw string) bool {
	return bcrypt.CompareHashAndPassword([]byte(hash), []byte(pw)) == nil
}
