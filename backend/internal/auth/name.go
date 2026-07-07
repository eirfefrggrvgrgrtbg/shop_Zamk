package auth

import (
	"errors"
	"regexp"
	"strings"
)

var (
	ErrNameEmpty         = errors.New("Введите имя.")
	ErrLastNameEmpty     = errors.New("Введите фамилию.")
	ErrNameInvalid       = errors.New("Имя указано некорректно.")
	ErrLastNameInvalid   = errors.New("Фамилия указана некорректно.")
	ErrMiddleNameInvalid = errors.New("Отчество указано некорректно.")
)

var nameRegex = regexp.MustCompile(`^[a-zA-Zа-яА-ЯёЁ\s\-']+$`)

func ValidateNameFields(firstName, lastName, middleName *string) error {
	*firstName = strings.TrimSpace(*firstName)
	*lastName = strings.TrimSpace(*lastName)
	if middleName != nil {
		*middleName = strings.TrimSpace(*middleName)
	}

	if *firstName == "" {
		return ErrNameEmpty
	}
	if *lastName == "" {
		return ErrLastNameEmpty
	}

	if len([]rune(*firstName)) > 80 || !isValidName(*firstName) {
		return ErrNameInvalid
	}
	if len([]rune(*lastName)) > 80 || !isValidName(*lastName) {
		return ErrLastNameInvalid
	}
	if middleName != nil && *middleName != "" {
		if len([]rune(*middleName)) > 80 || !isValidName(*middleName) {
			return ErrMiddleNameInvalid
		}
	}

	return nil
}

func isValidName(s string) bool {
	if !nameRegex.MatchString(s) {
		return false
	}
	// Check for obvious junk like "---", "...", "'''", or all spaces
	junk := true
	for _, r := range s {
		if r != '-' && r != '\'' && r != ' ' && r != '.' {
			junk = false
			break
		}
	}
	return !junk
}
