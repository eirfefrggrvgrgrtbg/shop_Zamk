package marketing

import (
	"errors"
	"fmt"
	"math"
)

var (
	ErrIntegerOverflow = errors.New("monetary calculation exceeds maximum integer boundary")
)

// SplitDiscountResult represents the exact financial breakdown for a unit under a co-funded or seller promotion.
type SplitDiscountResult struct {
	BasePriceCents         int64
	SellerDiscountCents    int64
	ZamkSubsidyCents       int64
	CustomerPaidCents      int64
	CommissionBaseCents    int64
	CommissionChargedCents int64
	SellerEarningCents     int64
}

// LineSplitDiscountResult represents the full unit and line-total breakdown for an order item under a promotion.
type LineSplitDiscountResult struct {
	BaseUnitPriceCents          int64
	SellerDiscountUnitCents     int64
	ZamkSubsidyUnitCents        int64
	CustomerPaidUnitPriceCents  int64
	CommissionBaseUnitCents     int64
	Quantity                    int
	TotalSellerDiscountCents    int64
	TotalZamkSubsidyCents       int64
	TotalCustomerPaidCents      int64
	TotalCommissionBaseCents    int64
	CommissionRateBps           int
	TotalCommissionChargedCents int64
	SellerNetEarningTotalCents  int64
}

// RoundBps calculates the monetary value in cents for a given basis point percentage using canonical ZAMK integer arithmetic.
// Formula: (cents * bps) / 10000.
// Checks integer overflow boundary against math.MaxInt64.
func RoundBps(cents int64, bps int) int64 {
	if cents <= 0 || bps <= 0 {
		return 0
	}
	// Check for overflow before multiplying: cents * bps <= math.MaxInt64
	if int64(bps) > 0 && cents > math.MaxInt64/int64(bps) {
		// Overflow boundary reached: return maximum representable value
		return math.MaxInt64 / 10000
	}
	return (cents * int64(bps)) / 10000
}

// CalculateSplitDiscount computes the canonical financial breakdown for a percentage-based co-funded or seller promotion (per unit).
// Invariants enforced:
// 1. Commission Base = Base Price - Seller Discount (ZAMK subsidy does NOT reduce the commission base).
// 2. Customer Paid = Base Price - Seller Discount - ZAMK Subsidy.
// 3. Customer Paid >= 0.
// 4. Commission is calculated strictly from Commission Base.
// 5. Seller Net Earning = Commission Base - Commission Charged.
func CalculateSplitDiscount(basePriceCents int64, sellerDiscountBps int, zamkShareBps int, commissionBps int) (SplitDiscountResult, error) {
	if basePriceCents < 0 {
		return SplitDiscountResult{}, fmt.Errorf("base price cannot be negative: %d", basePriceCents)
	}
	if sellerDiscountBps < 0 || sellerDiscountBps > 10000 {
		return SplitDiscountResult{}, fmt.Errorf("seller discount bps out of range [0, 10000]: %d", sellerDiscountBps)
	}
	if zamkShareBps < 0 || zamkShareBps > 2500 {
		return SplitDiscountResult{}, fmt.Errorf("ZAMK share bps out of range [0, 2500]: %d", zamkShareBps)
	}
	if commissionBps < 0 || commissionBps > 10000 {
		return SplitDiscountResult{}, fmt.Errorf("commission bps out of range [0, 10000]: %d", commissionBps)
	}

	// Boundary check on base price
	if basePriceCents > math.MaxInt64/10000 {
		return SplitDiscountResult{}, ErrIntegerOverflow
	}

	sellerDiscount := RoundBps(basePriceCents, sellerDiscountBps)
	zamkSubsidy := RoundBps(basePriceCents, zamkShareBps)

	commissionBase := basePriceCents - sellerDiscount
	if commissionBase < 0 {
		commissionBase = 0
	}

	customerPaid := basePriceCents - sellerDiscount - zamkSubsidy
	if customerPaid < 0 {
		return SplitDiscountResult{}, ErrCustomerPriceNegative
	}

	commissionCharged := RoundBps(commissionBase, commissionBps)
	sellerEarning := commissionBase - commissionCharged

	return SplitDiscountResult{
		BasePriceCents:         basePriceCents,
		SellerDiscountCents:    sellerDiscount,
		ZamkSubsidyCents:       zamkSubsidy,
		CustomerPaidCents:      customerPaid,
		CommissionBaseCents:    commissionBase,
		CommissionChargedCents: commissionCharged,
		SellerEarningCents:     sellerEarning,
	}, nil
}

// CalculateLineSplitDiscount computes both unit and total monetary values for an order item line.
func CalculateLineSplitDiscount(baseUnitPriceCents int64, quantity int, sellerDiscountBps int, zamkShareBps int, commissionBps int) (LineSplitDiscountResult, error) {
	if quantity <= 0 {
		return LineSplitDiscountResult{}, fmt.Errorf("quantity must be positive: %d", quantity)
	}

	unitRes, err := CalculateSplitDiscount(baseUnitPriceCents, sellerDiscountBps, zamkShareBps, commissionBps)
	if err != nil {
		return LineSplitDiscountResult{}, err
	}

	q := int64(quantity)
	if baseUnitPriceCents > 0 && q > math.MaxInt64/baseUnitPriceCents {
		return LineSplitDiscountResult{}, ErrIntegerOverflow
	}

	totalSellerDiscount := unitRes.SellerDiscountCents * q
	totalZamkSubsidy := unitRes.ZamkSubsidyCents * q
	totalCustomerPaid := unitRes.CustomerPaidCents * q
	totalCommissionBase := unitRes.CommissionBaseCents * q
	totalCommissionCharged := RoundBps(totalCommissionBase, commissionBps)
	sellerNetTotal := totalCommissionBase - totalCommissionCharged

	return LineSplitDiscountResult{
		BaseUnitPriceCents:          unitRes.BasePriceCents,
		SellerDiscountUnitCents:     unitRes.SellerDiscountCents,
		ZamkSubsidyUnitCents:        unitRes.ZamkSubsidyCents,
		CustomerPaidUnitPriceCents:  unitRes.CustomerPaidCents,
		CommissionBaseUnitCents:     unitRes.CommissionBaseCents,
		Quantity:                    quantity,
		TotalSellerDiscountCents:    totalSellerDiscount,
		TotalZamkSubsidyCents:       totalZamkSubsidy,
		TotalCustomerPaidCents:      totalCustomerPaid,
		TotalCommissionBaseCents:    totalCommissionBase,
		CommissionRateBps:           commissionBps,
		TotalCommissionChargedCents: totalCommissionCharged,
		SellerNetEarningTotalCents:  sellerNetTotal,
	}, nil
}

// CalculateFixedDiscount computes financial split for fixed-amount seller-funded promotions.
// Only valid for funding_mode = 'seller' in V1.
func CalculateFixedDiscount(basePriceCents int64, sellerDiscountFixedCents int64, commissionBps int) (SplitDiscountResult, error) {
	if basePriceCents < 0 {
		return SplitDiscountResult{}, fmt.Errorf("base price cannot be negative: %d", basePriceCents)
	}
	if sellerDiscountFixedCents < 0 {
		return SplitDiscountResult{}, fmt.Errorf("seller discount cannot be negative: %d", sellerDiscountFixedCents)
	}
	if commissionBps < 0 || commissionBps > 10000 {
		return SplitDiscountResult{}, fmt.Errorf("commission bps out of range [0, 10000]: %d", commissionBps)
	}

	sellerDiscount := sellerDiscountFixedCents
	if sellerDiscount > basePriceCents {
		sellerDiscount = basePriceCents
	}

	commissionBase := basePriceCents - sellerDiscount
	customerPaid := basePriceCents - sellerDiscount
	commissionCharged := RoundBps(commissionBase, commissionBps)
	sellerEarning := commissionBase - commissionCharged

	return SplitDiscountResult{
		BasePriceCents:         basePriceCents,
		SellerDiscountCents:    sellerDiscount,
		ZamkSubsidyCents:       0,
		CustomerPaidCents:      customerPaid,
		CommissionBaseCents:    commissionBase,
		CommissionChargedCents: commissionCharged,
		SellerEarningCents:     sellerEarning,
	}, nil
}
