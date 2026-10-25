package orders

import (
	"errors"
	"strings"
	"unicode"
	"unicode/utf8"
)

// SHOP.DELIVERY.DETAILS.1: the optional postal code and courier comment of a checkout. They are trimmed and checked
// here, on the server, whatever the Shop sent; an empty value is "not given" (stored as NULL). The limits match the
// address book and the orders table constraints.

const (
	maxDeliveryPostalCode     = 12
	maxDeliveryCourierComment = 500
)

// DeliveryDetailsError names the field and says what is wrong, in Russian.
type DeliveryDetailsError struct {
	Field   string
	Message string
}

func (e *DeliveryDetailsError) Error() string { return e.Message }

var ErrInvalidDeliveryDetails = errors.New("invalid delivery details")

func (e *DeliveryDetailsError) Is(target error) bool { return target == ErrInvalidDeliveryDetails }

func trimmed(p *string) string {
	if p == nil {
		return ""
	}
	return strings.TrimSpace(*p)
}

// normalizeDeliveryDetails trims and validates the two optional fields in place (empty becomes nil).
func normalizeDeliveryDetails(req *CreateOrderRequest) error {
	postal, comment := trimmed(req.DeliveryPostalCode), trimmed(req.DeliveryCourierComment)

	if utf8.RuneCountInString(postal) > maxDeliveryPostalCode {
		return &DeliveryDetailsError{"deliveryPostalCode", "Индекс слишком длинный (не более 12 символов)"}
	}
	for _, r := range postal {
		if !(unicode.IsDigit(r) || unicode.IsLetter(r) || r == ' ' || r == '-') {
			return &DeliveryDetailsError{"deliveryPostalCode", "Индекс может содержать только буквы, цифры, пробел и дефис"}
		}
	}
	if utf8.RuneCountInString(comment) > maxDeliveryCourierComment {
		return &DeliveryDetailsError{"deliveryCourierComment", "Комментарий для курьера слишком длинный (не более 500 символов)"}
	}
	for _, r := range comment {
		if unicode.IsControl(r) {
			return &DeliveryDetailsError{"deliveryCourierComment", "Комментарий для курьера содержит недопустимые символы"}
		}
	}

	req.DeliveryPostalCode, req.DeliveryCourierComment = nil, nil
	if postal != "" {
		req.DeliveryPostalCode = &postal
	}
	if comment != "" {
		req.DeliveryCourierComment = &comment
	}
	return nil
}
