package products

import "errors"

var (
	ErrProductNotFound        = errors.New("product not found")
	ErrDuplicateSlug          = errors.New("slug already exists")
	ErrUnauthorized           = errors.New("unauthorized to modify this product")
	ErrInvalidStatusTransition = errors.New("invalid product status transition")
	ErrSellerNotFound         = errors.New("seller profile not found for user")
)
