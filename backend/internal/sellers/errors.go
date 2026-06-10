package sellers

import "errors"

var (
	ErrSellerNotFound     = errors.New("seller not found")
	ErrSellerUserNotFound = errors.New("seller user not found")
	ErrDuplicateSlug      = errors.New("seller slug already exists")
	ErrDuplicateEmail     = errors.New("user email already exists")
)

var (
	ErrSellerNotPending    = errors.New("seller is not in pending status")
	ErrSellerIncomplete    = errors.New("seller profile is incomplete")
	ErrReasonRequired      = errors.New("reason is required for this status change")
	ErrWarningNotFound     = errors.New("warning not found")
	ErrViolationNotFound   = errors.New("violation not found")
	ErrAlreadyResolved     = errors.New("already resolved or cancelled")
)

// VerifyMissingFieldsError is returned when VerifySeller finds incomplete profile fields.
type VerifyMissingFieldsError struct {
	Fields []string
}

func (e *VerifyMissingFieldsError) Error() string {
	return "seller profile is incomplete: missing fields: " + joinFields(e.Fields)
}

func joinFields(fields []string) string {
	result := ""
	for i, f := range fields {
		if i > 0 {
			result += ", "
		}
		result += f
	}
	return result
}
