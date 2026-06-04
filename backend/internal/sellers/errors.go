package sellers

import "errors"

var (
	ErrSellerNotFound     = errors.New("seller not found")
	ErrSellerUserNotFound = errors.New("seller user not found")
	ErrDuplicateSlug      = errors.New("seller slug already exists")
	ErrDuplicateEmail     = errors.New("user email already exists")
)
