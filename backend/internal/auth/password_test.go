package auth

import (
	"testing"
)

func TestValidatePassword(t *testing.T) {
	name := "John Doe"
	email := "johndoe@example.com"

	tests := []struct {
		name     string
		password string
		expected error
	}{
		{"Valid password", "SuperSecr3t!", nil},
		{"Too short", "Sh0rt!", ErrPasswordTooShort},
		{"Too long", "A1!b" + string(make([]byte, 130)) + "c2@", ErrPasswordTooShort},
		{"No uppercase", "supersecr3t!", ErrPasswordNoUpper},
		{"No lowercase", "SUPERSECR3T!", ErrPasswordNoLower},
		{"No digit", "SuperSecret!", ErrPasswordNoDigit},
		{"No special", "SuperSecr3tT", ErrPasswordNoSpecial},
		{"Common weak", "MyQwerty123!", ErrPasswordTooSimple},
		{"Obvious date", "Cool2024Pass!", ErrPasswordTooSimple},
		{"Repeated chars", "Aaa123!@#$", ErrPasswordRepeated},
		{"Sequential digits asc", "Pass012345!", ErrPasswordSequential},
		{"Sequential digits desc", "Pass987654!", ErrPasswordSequential},
		{"Contains email part", "PassJohnDoe1!", ErrPasswordContainsInfo},
		{"Contains name part", "SecretJohn1!", ErrPasswordContainsInfo},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidatePassword(tt.password, name, email)
			if err != tt.expected {
				t.Errorf("expected %v, got %v", tt.expected, err)
			}
		})
	}
}
