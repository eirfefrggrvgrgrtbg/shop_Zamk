package reviews

import "errors"

var (
	ErrReviewNotFound      = errors.New("review not found")
	ErrOrderNotDelivered   = errors.New("order is not delivered")
	ErrDuplicateReview     = errors.New("duplicate review for order item")
	ErrReviewAlreadyExists = ErrDuplicateReview
	ErrItemNotPurchased    = errors.New("item not purchased or order does not belong to user")
	ErrInvalidRating       = errors.New("rating must be between 1 and 5")
	ErrReviewTextTooLong   = errors.New("review text must be at most 1000 characters")
	ErrProductNotFound     = errors.New("product not found")
)
