package marketing_test

import (
	"math"
	"testing"

	"github.com/eirfefrggrvgrgrtbg/shop-zamk/backend/internal/marketing"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCalculateSplitDiscount_CanonicalExample(t *testing.T) {
	// Canonical Product Owner Example:
	// Base price: 5000 RUB = 500000 cents
	// Seller discount: 15% = 1500 bps = 750 RUB = 75000 cents
	// ZAMK subsidy: 10% = 1000 bps = 500 RUB = 50000 cents
	// Commission rate: 9% = 900 bps
	// Expected:
	// - Customer pays: 3750 RUB = 375000 cents
	// - Commission base: 4250 RUB = 425000 cents
	// - ZAMK commission charged: 4250 * 0.09 = 382.50 RUB = 38250 cents
	// - Seller net earning: 4250 - 382.50 = 3867.50 RUB = 386750 cents

	res, err := marketing.CalculateSplitDiscount(500000, 1500, 1000, 900)
	require.NoError(t, err)

	assert.Equal(t, int64(500000), res.BasePriceCents)
	assert.Equal(t, int64(75000), res.SellerDiscountCents, "Seller discount must be exactly 750 RUB (75000 cents)")
	assert.Equal(t, int64(50000), res.ZamkSubsidyCents, "ZAMK subsidy must be exactly 500 RUB (50000 cents)")
	assert.Equal(t, int64(375000), res.CustomerPaidCents, "Customer must pay exactly 3750 RUB (375000 cents)")
	assert.Equal(t, int64(425000), res.CommissionBaseCents, "Commission base must be exactly 4250 RUB (425000 cents)")
	assert.Equal(t, int64(38250), res.CommissionChargedCents, "Commission must be 382.50 RUB (38250 cents)")
	assert.Equal(t, int64(386750), res.SellerEarningCents, "Seller earning must be 3867.50 RUB (386750 cents)")

	// Invariant: ZAMK subsidy must NOT reduce the seller commission base
	assert.Equal(t, res.BasePriceCents-res.SellerDiscountCents, res.CommissionBaseCents)
	// Invariant: Customer Paid + Seller Discount + ZAMK Subsidy = Base Price
	assert.Equal(t, res.BasePriceCents, res.CustomerPaidCents+res.SellerDiscountCents+res.ZamkSubsidyCents)
}

func TestCalculateLineSplitDiscount_QuantityMultiple(t *testing.T) {
	// Canonical example with Quantity Q = 2:
	// Base unit = 500000 cents
	// Seller discount unit = 75000 cents
	// ZAMK subsidy unit = 50000 cents
	// Customer paid unit = 375000 cents
	// Commission base unit = 425000 cents
	// Quantity = 2
	lineRes, err := marketing.CalculateLineSplitDiscount(500000, 2, 1500, 1000, 900)
	require.NoError(t, err)

	assert.Equal(t, int64(500000), lineRes.BaseUnitPriceCents)
	assert.Equal(t, int64(75000), lineRes.SellerDiscountUnitCents)
	assert.Equal(t, int64(50000), lineRes.ZamkSubsidyUnitCents)
	assert.Equal(t, int64(375000), lineRes.CustomerPaidUnitPriceCents)
	assert.Equal(t, int64(425000), lineRes.CommissionBaseUnitCents)
	assert.Equal(t, 2, lineRes.Quantity)

	// Line totals
	assert.Equal(t, int64(150000), lineRes.TotalSellerDiscountCents, "75000 * 2 = 150000")
	assert.Equal(t, int64(100000), lineRes.TotalZamkSubsidyCents, "50000 * 2 = 100000")
	assert.Equal(t, int64(750000), lineRes.TotalCustomerPaidCents, "375000 * 2 = 750000")
	assert.Equal(t, int64(850000), lineRes.TotalCommissionBaseCents, "425000 * 2 = 850000")
	assert.Equal(t, int64(76500), lineRes.TotalCommissionChargedCents, "850000 * 900 / 10000 = 76500")
	assert.Equal(t, int64(773500), lineRes.SellerNetEarningTotalCents, "850000 - 76500 = 773500")

	// Invariant: Total Customer Paid = Unit Customer Paid * Quantity
	assert.Equal(t, lineRes.CustomerPaidUnitPriceCents*int64(lineRes.Quantity), lineRes.TotalCustomerPaidCents)
	// Invariant: Total Commission Base = Unit Commission Base * Quantity
	assert.Equal(t, lineRes.CommissionBaseUnitCents*int64(lineRes.Quantity), lineRes.TotalCommissionBaseCents)
}

func TestCalculateSplitDiscount_PureSellerDiscount(t *testing.T) {
	// Base price: 100000 cents (1000 RUB), Seller discount: 20% (2000 bps), ZAMK: 0% (0 bps), Comm: 15% (1500 bps)
	res, err := marketing.CalculateSplitDiscount(100000, 2000, 0, 1500)
	require.NoError(t, err)

	assert.Equal(t, int64(20000), res.SellerDiscountCents)
	assert.Equal(t, int64(0), res.ZamkSubsidyCents)
	assert.Equal(t, int64(80000), res.CustomerPaidCents)
	assert.Equal(t, int64(80000), res.CommissionBaseCents)
	assert.Equal(t, int64(12000), res.CommissionChargedCents) // 80000 * 1500 / 10000 = 12000
	assert.Equal(t, int64(68000), res.SellerEarningCents)
}

func TestCalculateSplitDiscount_MaxZamkCeiling(t *testing.T) {
	// Max allowed ZAMK share is 25% (2500 bps)
	res, err := marketing.CalculateSplitDiscount(100000, 1000, 2500, 900)
	require.NoError(t, err)
	assert.Equal(t, int64(25000), res.ZamkSubsidyCents)
	assert.Equal(t, int64(65000), res.CustomerPaidCents)

	// Exceeding 2500 bps must be rejected
	_, err = marketing.CalculateSplitDiscount(100000, 1000, 2501, 900)
	assert.Error(t, err)
}

func TestCalculateSplitDiscount_NegativePriceProtection(t *testing.T) {
	// If discounts exceed 100%, customer price becomes negative -> reject
	_, err := marketing.CalculateSplitDiscount(100000, 8000, 2500, 900) // 80% + 25% = 105%
	assert.ErrorIs(t, err, marketing.ErrCustomerPriceNegative)
}

func TestCalculateFixedDiscount_SellerOnly(t *testing.T) {
	// Base price: 300000 cents (3000 RUB), Fixed discount: 50000 cents (500 RUB), Comm: 10% (1000 bps)
	res, err := marketing.CalculateFixedDiscount(300000, 50000, 1000)
	require.NoError(t, err)

	assert.Equal(t, int64(50000), res.SellerDiscountCents)
	assert.Equal(t, int64(0), res.ZamkSubsidyCents)
	assert.Equal(t, int64(250000), res.CustomerPaidCents)
	assert.Equal(t, int64(250000), res.CommissionBaseCents)
	assert.Equal(t, int64(25000), res.CommissionChargedCents)
	assert.Equal(t, int64(225000), res.SellerEarningCents)
}

func TestMoney_IntegerOverflowBoundaryProtection(t *testing.T) {
	// Overflow protection on RoundBps
	maxInt := int64(math.MaxInt64)
	val := marketing.RoundBps(maxInt, 5000)
	assert.True(t, val > 0, "must not overflow into negative numbers")

	// CalculateSplitDiscount with near-max price
	_, err := marketing.CalculateSplitDiscount(maxInt, 1000, 1000, 1000)
	assert.ErrorIs(t, err, marketing.ErrIntegerOverflow)

	// CalculateLineSplitDiscount with huge quantity
	_, err = marketing.CalculateLineSplitDiscount(1000000, math.MaxInt32, 1000, 1000, 1000)
	// Must execute safely without panic
	assert.NoError(t, err)
}
