package observability

import (
	"regexp"

	"github.com/google/uuid"
)

// safeRequestIDRegex enforces that an incoming request ID is bounded and safe.
// Max length 64, alphanumeric plus hyphens and underscores.
var safeRequestIDRegex = regexp.MustCompile(`^[a-zA-Z0-9_\-]{1,64}$`)

// IsValidRequestID validates whether the given request ID string is safe and bounded.
func IsValidRequestID(id string) bool {
	if len(id) < 1 || len(id) > 64 {
		return false
	}
	return safeRequestIDRegex.MatchString(id)
}

// NewRequestID generates a new canonical UUID v4 request ID.
func NewRequestID() string {
	return uuid.NewString()
}
