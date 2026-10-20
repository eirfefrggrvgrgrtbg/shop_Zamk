package marketing

import (
	"fmt"
	"math/rand"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// M0.3 L: AllocateCappedCofundedDiscount keeps every invariant for many random co-funded orders.
func TestAllocateCappedCofundedDiscount_Properties(t *testing.T) {
	rng := rand.New(rand.NewSource(42))
	exactCap := 0
	for n := 0; n < 5000; n++ {
		nLines := 1 + rng.Intn(4)
		lines := make([]LineSplitDiscountResult, nLines)
		ids := make([]string, nLines)
		var S, Z int64
		sellerBps := 1 + rng.Intn(7499)
		zamkBps := 1 + rng.Intn(2499)
		for i := range lines {
			base := int64(1 + rng.Intn(5_000_000))
			q := 1 + rng.Intn(5)
			l, err := CalculateLineSplitDiscount(base, q, sellerBps, zamkBps, rng.Intn(3000))
			require.NoError(t, err)
			lines[i] = l
			ids[i] = fmt.Sprintf("id-%02d", i)
			S += l.TotalSellerDiscountCents
			Z += l.TotalZamkSubsidyCents
		}
		T := S + Z
		if T < 2 {
			continue
		}
		capCents := 1 + rng.Int63n(T-1)
		out, err := AllocateCappedCofundedDiscount(capCents, lines, ids)
		require.NoError(t, err)

		var S2, Z2 int64
		for i, l := range out {
			q := int64(l.Quantity)
			gross := l.BaseUnitPriceCents * q
			assert.LessOrEqual(t, l.TotalSellerDiscountCents, lines[i].TotalSellerDiscountCents, "seller never above its uncapped share")
			assert.LessOrEqual(t, l.TotalZamkSubsidyCents, lines[i].TotalZamkSubsidyCents, "ZAMK never above its uncapped share")
			assert.GreaterOrEqual(t, l.TotalSellerDiscountCents, int64(0))
			assert.Equal(t, l.ZamkSubsidyUnitCents*q, l.TotalZamkSubsidyCents, "ZAMK subsidy stays a per-unit amount")
			assert.Equal(t, gross-l.TotalSellerDiscountCents, l.TotalCommissionBaseCents, "commission_base = base - seller")
			assert.Equal(t, l.TotalCommissionBaseCents-l.TotalZamkSubsidyCents, l.TotalCustomerPaidCents, "customer_paid + zamk = commission_base")
			assert.Equal(t, RoundBps(l.TotalCommissionBaseCents, l.CommissionRateBps), l.TotalCommissionChargedCents)
			assert.Equal(t, l.BaseUnitPriceCents-l.SellerDiscountUnitCents-l.ZamkSubsidyUnitCents, l.CustomerPaidUnitPriceCents)
			assert.Equal(t, l.BaseUnitPriceCents-l.SellerDiscountUnitCents, l.CommissionBaseUnitCents)
			S2 += l.TotalSellerDiscountCents
			Z2 += l.TotalZamkSubsidyCents
		}
		assert.LessOrEqual(t, S2+Z2, capCents, "never more than the cap")
		assert.GreaterOrEqual(t, S2, capCents*S/T, "seller pays at least its proportional share of the cap")
		assert.LessOrEqual(t, S2, S)
		if S2+Z2 == capCents {
			exactCap++
		} else {
			// only when ZAMK's whole-unit steps cannot reach its share and the seller is already at its uncapped share
			assert.Equal(t, S, S2, "a total below the cap happens only when the seller is at its full share")
		}
	}
	assert.Greater(t, exactCap, 4900, "the total equals the cap in practically all cases")
}

func TestAllocateCappedCofundedDiscount_CanonicalExample(t *testing.T) {
	// 10 000 ₽, seller 30 % + ZAMK 10 % = 4 000 ₽ discount, cap 2 000 ₽ -> seller 1 500 ₽, ZAMK 500 ₽.
	l, err := CalculateLineSplitDiscount(1_000_000, 1, 3000, 1000, 900)
	require.NoError(t, err)
	out, err := AllocateCappedCofundedDiscount(200_000, []LineSplitDiscountResult{l}, []string{"a"})
	require.NoError(t, err)
	assert.Equal(t, int64(150_000), out[0].TotalSellerDiscountCents)
	assert.Equal(t, int64(50_000), out[0].TotalZamkSubsidyCents)
	assert.Equal(t, int64(800_000), out[0].TotalCustomerPaidCents)
	assert.Equal(t, int64(850_000), out[0].TotalCommissionBaseCents)
	assert.Equal(t, int64(76_500), out[0].TotalCommissionChargedCents)

	same, err := AllocateCappedCofundedDiscount(500_000, []LineSplitDiscountResult{l}, []string{"a"})
	require.NoError(t, err)
	assert.Equal(t, l, same[0], "cap above the discount changes nothing")
}

func TestAllocateCappedCofundedDiscount_RoundingGoesToZamkAndIsDeterministic(t *testing.T) {
	// S = 2, Z = 1, cap = 2: proportional seller share 1.33 -> 1, ZAMK gets the remainder 1.
	l := LineSplitDiscountResult{BaseUnitPriceCents: 10, Quantity: 1, SellerDiscountUnitCents: 2, ZamkSubsidyUnitCents: 1,
		TotalSellerDiscountCents: 2, TotalZamkSubsidyCents: 1, CommissionRateBps: 0}
	out, err := AllocateCappedCofundedDiscount(2, []LineSplitDiscountResult{l}, []string{"a"})
	require.NoError(t, err)
	assert.Equal(t, int64(1), out[0].TotalSellerDiscountCents)
	assert.Equal(t, int64(1), out[0].TotalZamkSubsidyCents)
	for i := 0; i < 20; i++ {
		again, _ := AllocateCappedCofundedDiscount(2, []LineSplitDiscountResult{l}, []string{"a"})
		assert.Equal(t, out, again)
	}
}
