package products

import "errors"

var (
	ErrProductNotFound         = errors.New("product not found")
	ErrDuplicateSlug           = errors.New("slug already exists")
	ErrUnauthorized            = errors.New("unauthorized to modify this product")
	ErrInvalidStatusTransition = errors.New("invalid product status transition")
	ErrSellerNotFound          = errors.New("seller profile not found for user")
	ErrSellerNotActive         = errors.New("seller is not active")
	ErrSellerBlocked           = errors.New("seller is blocked or archived")
	ErrSellerHasNoPrimaryBrand = errors.New("seller has no active primary brand")
	ErrSellerHasMultiplePrimaryBrands = errors.New("seller has multiple active primary brands")
	ErrProductNotEditable      = errors.New("product cannot be edited in its current state")
	ErrRejectionReasonRequired = errors.New("rejection reason is required")
	ErrProductOptimisticLockFailed = errors.New("optimistic lock failed")
	ErrInvalidPreviewToken     = errors.New("invalid or expired preview token")
	ErrPreviewUnavailable      = errors.New("preview is currently unavailable")
	ErrPreviewProductUnavailable = errors.New("product is not available for preview")
	ErrRedisUnavailable        = errors.New("redis service unavailable")
)

type ErrNotPublishable struct {
	Reasons []string
}

func (e *ErrNotPublishable) Error() string {
	return "product is not publishable"
}

type DuplicateSKUError struct {
	SKU string
}

func (e *DuplicateSKUError) Error() string {
	return "SKU already exists: " + e.SKU
}

