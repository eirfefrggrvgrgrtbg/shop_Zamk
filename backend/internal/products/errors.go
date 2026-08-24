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
	ErrValidation              = errors.New("validation failed")

	ErrProductCategoryRequired     = errors.New("category is required for moderation")
	ErrProductMediaRequired        = errors.New("at least one image is required")
	ErrProductMediaNotReady        = errors.New("all images must have explicit 4:5 renditions before submission")
	ErrProductMainImageMissing     = errors.New("a main image is required")
	ErrProductVariantsRequired     = errors.New("at least one active variant is required")
	ErrProductPriceInvalid         = errors.New("variant price must be greater than 0")
	ErrProductSKURequired          = errors.New("variant seller SKU is required")
	ErrProductCompositionInvalid   = errors.New("material composition must total exactly 100%")
	ErrProductSizeChartRequired    = errors.New("category requires a size chart")
	ErrProductSizeChartIncomplete  = errors.New("missing required measurement in size chart row")
)

type ValidationError struct {
	Message string
}

func (e *ValidationError) Error() string {
	return e.Message
}

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

