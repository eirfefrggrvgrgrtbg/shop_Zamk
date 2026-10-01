package marketing

import "errors"

var (
	ErrInvalidFundingMode                  = errors.New("invalid funding mode")
	ErrInvalidCampaignStatus                = errors.New("invalid campaign status")
	ErrInvalidDiscountType                  = errors.New("invalid discount type")
	ErrFixedDiscountNotAllowed              = errors.New("fixed discount is not supported for platform-funded campaigns in V1")
	ErrZamkShareExceedsCeiling              = errors.New("requested ZAMK share exceeds maximum ceiling of 25% (2500 bps)")
	ErrSellerFundedCannotHaveZamk           = errors.New("seller-funded campaigns cannot have ZAMK subsidy or budget")
	ErrApprovedExceedsRequested             = errors.New("approved financial terms cannot exceed requested terms")
	ErrInvalidBudgetCap                     = errors.New("approved ZAMK budget cap must be greater than zero for approved platform funding")
	ErrCustomerPriceNegative                = errors.New("customer price cannot become negative")
	ErrCampaignNotFound                     = errors.New("marketing campaign not found")
	ErrPromoCodeNotFound                    = errors.New("promo code not found")
	ErrPromoCodeDuplicate                   = errors.New("promo code already exists")
	ErrOpenPlatformFundingExists            = errors.New("seller already has an active or submitted platform-funded campaign")
	ErrCounterOfferInvalidState             = errors.New("counter offer can only be accepted or rejected in counter_offered status")
	ErrSellerUnauthorized                   = errors.New("seller is not authorized to modify this campaign")
	ErrCannotMutateApprovedFields           = errors.New("seller cannot mutate admin-approved financial fields")
	ErrInvalidPriceHistory                  = errors.New("invalid price history record")
	ErrInvalidOrderItemPromotion            = errors.New("invalid order item promotion snapshot")
	ErrCofundedRequiresSellerContribution   = errors.New("cofunded campaign requires positive seller discount percentage")
	ErrSellerRequiresDiscount               = errors.New("seller-funded campaign must specify positive discount")
	ErrZamkCannotHaveSellerDiscount         = errors.New("platform-funded campaign cannot have seller discount")
	ErrSubmittedPlatformRequiresRequested   = errors.New("submitted platform-funded campaign requires positive requested share and budget cap")
	ErrUnreviewedMustHaveZeroApproved       = errors.New("unreviewed platform campaign must have zero approved terms")
)
