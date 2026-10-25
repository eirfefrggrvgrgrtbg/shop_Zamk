package addresses

import (
	"errors"
	"strings"
	"unicode"
	"unicode/utf8"
)

// SHOP.ADDRESSES.1: every address that reaches the database is trimmed and validated here. Limits follow the shape
// the delivery model already uses (recipient, phone, city, street, house, apartment) and protect the table from
// oversized or control-character input.

const (
	maxAddressesPerUser = 20
	maxBodyBytes        = 16 << 10
)

// ValidationError names the offending field (the API field name) and says what is wrong, in Russian.
type ValidationError struct {
	Field   string
	Message string
}

func (e *ValidationError) Error() string { return e.Message }

var ErrAddressLimit = errors.New("address limit reached")

type fieldRule struct {
	name     string
	label    string
	max      int
	required bool
}

var rules = []fieldRule{
	{"label", "Название", 60, false},
	{"recipientName", "ФИО получателя", 120, true},
	{"phone", "Телефон", 32, true},
	{"city", "Город", 120, true},
	{"street", "Улица", 200, true},
	{"house", "Дом", 30, true},
	{"apartment", "Квартира", 30, false},
	{"postalCode", "Индекс", 12, false},
	{"comment", "Комментарий", 500, false},
}

func clean(s string) string { return strings.TrimSpace(s) }

func hasControl(s string) bool {
	for _, r := range s {
		if unicode.IsControl(r) {
			return true
		}
	}
	return false
}

func optional(p **string) string {
	if *p == nil {
		return ""
	}
	return **p
}

func setOptional(p **string, v string) {
	if v == "" {
		*p = nil
		return
	}
	*p = &v
}

// normalizeAndValidate trims every text field in place and returns the first problem found.
func normalizeAndValidate(a *Address) *ValidationError {
	values := map[string]*string{}
	opt := map[string]**string{"label": &a.Label, "apartment": &a.Apartment, "postalCode": &a.PostalCode, "comment": &a.Comment}
	req := map[string]*string{"recipientName": &a.RecipientName, "phone": &a.Phone, "city": &a.City, "street": &a.Street, "house": &a.House}
	for k, p := range req {
		*p = clean(*p)
		values[k] = p
	}
	for k, pp := range opt {
		v := clean(optional(pp))
		setOptional(pp, v)
		tmp := v
		values[k] = &tmp
	}
	for _, r := range rules {
		v := *values[r.name]
		if r.required && v == "" {
			return &ValidationError{r.name, "Заполните поле «" + r.label + "»"}
		}
		if utf8.RuneCountInString(v) > r.max {
			return &ValidationError{r.name, "Поле «" + r.label + "» слишком длинное (не более " + itoa(r.max) + " символов)"}
		}
		if hasControl(v) {
			return &ValidationError{r.name, "Поле «" + r.label + "» содержит недопустимые символы"}
		}
	}
	digits := 0
	for _, r := range a.Phone {
		switch {
		case unicode.IsDigit(r):
			digits++
		case strings.ContainsRune("+-() .", r):
		default:
			return &ValidationError{"phone", "Телефон может содержать только цифры, пробелы и символы + - ( )"}
		}
	}
	if digits < 7 || digits > 15 {
		return &ValidationError{"phone", "Укажите корректный телефон (от 7 до 15 цифр)"}
	}
	if a.PostalCode != nil {
		for _, r := range *a.PostalCode {
			if !(unicode.IsDigit(r) || unicode.IsLetter(r) || r == ' ' || r == '-') {
				return &ValidationError{"postalCode", "Индекс может содержать только буквы, цифры, пробел и дефис"}
			}
		}
	}
	return nil
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	var b []byte
	for n > 0 {
		b = append([]byte{byte('0' + n%10)}, b...)
		n /= 10
	}
	return string(b)
}
