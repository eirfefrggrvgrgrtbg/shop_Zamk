package auth

import (
	"errors"
	"strings"
	"unicode"

	"golang.org/x/crypto/bcrypt"
)

var (
	ErrPasswordTooShort     = errors.New("Пароль слишком короткий.")
	ErrPasswordNoUpper      = errors.New("Пароль должен содержать заглавную букву.")
	ErrPasswordNoLower      = errors.New("Пароль должен содержать строчную букву.")
	ErrPasswordNoDigit      = errors.New("Пароль должен содержать цифру.")
	ErrPasswordNoSpecial    = errors.New("Пароль должен содержать спецсимвол.")
	ErrPasswordTooSimple    = errors.New("Пароль слишком простой.")
	ErrPasswordContainsInfo = errors.New("Пароль не должен содержать имя, фамилию или email.")
)

func IsPasswordError(err error) bool {
	return errors.Is(err, ErrPasswordTooShort) ||
		errors.Is(err, ErrPasswordNoUpper) ||
		errors.Is(err, ErrPasswordNoLower) ||
		errors.Is(err, ErrPasswordNoDigit) ||
		errors.Is(err, ErrPasswordNoSpecial) ||
		errors.Is(err, ErrPasswordTooSimple) ||
		errors.Is(err, ErrPasswordContainsInfo)
}

var commonWeakPasswords = []string{
	"password", "qwerty", "qwerty123", "123456", "123456789",
	"admin", "admin123", "zamk", "zamk123",
}

var obviousDates = []string{
	"2023", "2024", "2025", "2026", "01012000", "11112000",
}

func ValidatePassword(password, name, email string) error {
	if len(password) < 10 || len(password) > 128 {
		return ErrPasswordTooShort
	}

	var hasUpper, hasLower, hasDigit, hasSpecial bool
	specialChars := "!@#$%^&*.?-_+="

	for _, char := range password {
		switch {
		case unicode.IsUpper(char) && char >= 'A' && char <= 'Z':
			hasUpper = true
		case unicode.IsLower(char) && char >= 'a' && char <= 'z':
			hasLower = true
		case unicode.IsDigit(char):
			hasDigit = true
		case strings.ContainsRune(specialChars, char):
			hasSpecial = true
		}
	}

	if !hasUpper {
		return ErrPasswordNoUpper
	}
	if !hasLower {
		return ErrPasswordNoLower
	}
	if !hasDigit {
		return ErrPasswordNoDigit
	}
	if !hasSpecial {
		return ErrPasswordNoSpecial
	}

	lowerPass := strings.ToLower(password)

	for _, weak := range commonWeakPasswords {
		if strings.Contains(lowerPass, weak) {
			return ErrPasswordTooSimple
		}
	}
	for _, date := range obviousDates {
		if strings.Contains(lowerPass, date) {
			return ErrPasswordTooSimple
		}
	}

	if hasRepeatedCharacters(lowerPass) {
		return ErrPasswordTooSimple
	}

	if hasSequentialDigits(lowerPass) {
		return ErrPasswordTooSimple
	}

	if containsUserInfo(lowerPass, name, email) {
		return ErrPasswordContainsInfo
	}

	return nil
}

func hasRepeatedCharacters(s string) bool {
	count := 1
	for i := 1; i < len(s); i++ {
		if s[i] == s[i-1] {
			count++
			if count >= 3 {
				return true
			}
		} else {
			count = 1
		}
	}
	return false
}

func hasSequentialDigits(s string) bool {
	// Ascending
	for i := 0; i <= len(s)-3; i++ {
		if isDigit(s[i]) && isDigit(s[i+1]) && isDigit(s[i+2]) {
			if s[i+1] == s[i]+1 && s[i+2] == s[i]+2 {
				return true
			}
		}
	}
	// Descending
	for i := 0; i <= len(s)-3; i++ {
		if isDigit(s[i]) && isDigit(s[i+1]) && isDigit(s[i+2]) {
			if s[i+1] == s[i]-1 && s[i+2] == s[i]-2 {
				return true
			}
		}
	}
	return false
}

func isDigit(c byte) bool {
	return c >= '0' && c <= '9'
}

func containsUserInfo(lowerPass, name, email string) bool {
	// Check email local-part
	if email != "" {
		parts := strings.Split(email, "@")
		if len(parts) > 0 && len(parts[0]) >= 3 { // only check if length >= 3
			if strings.Contains(lowerPass, strings.ToLower(parts[0])) {
				return true
			}
		}
	}

	// Check name (split by space for first/last name)
	if name != "" {
		nameParts := strings.Fields(name)
		for _, part := range nameParts {
			if len(part) >= 3 { // only check if length >= 3
				if strings.Contains(lowerPass, strings.ToLower(part)) {
					return true
				}
			}
		}
	}
	return false
}

func HashPassword(password string) (string, error) {
	// Note: Validation should be done before hashing using ValidatePassword
	if len(password) < 8 || len(password) > 128 {
		return "", errors.New("password must be between 8 and 128 characters")
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return "", err
	}
	return string(hash), nil
}

func CheckPassword(password, hash string) bool {
	err := bcrypt.CompareHashAndPassword([]byte(hash), []byte(password))
	return err == nil
}
