package marketing

import (
	"errors"
	"fmt"
	"math"
	"math/big"
	"sort"
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

// PromotedItemInput contains inputs for distributing seller fixed discounts across order items.
type PromotedItemInput struct {
	ID                 string
	BaseUnitPriceCents int64
	Quantity           int
	CommissionRateBps  int
}

// AllocateSellerFixedDiscount deterministically allocates a fixed seller discount across multiple eligible items.
// Uses proportional allocation with the largest-remainder rule (Hare-Niemeyer method).
// Invariants enforced:
// 1. Sum of line discounts exactly equals min(fixedDiscountCents, eligibleSubtotal). Zero cents lost.
// 2. Deterministic tie-breaking on largest fractional remainder, subtotal descending, ID ascending.
// 3. No item customer payable becomes negative.
// 4. Zero ZAMK subsidy for SELLER promotions.
func AllocateSellerFixedDiscount(fixedDiscountCents int64, items []PromotedItemInput) ([]LineSplitDiscountResult, error) {
	if len(items) == 0 {
		return nil, nil
	}
	if fixedDiscountCents < 0 {
		return nil, fmt.Errorf("fixed discount cannot be negative: %d", fixedDiscountCents)
	}

	type itemMeta struct {
		index     int
		id        string
		subtotal  int64
		baseUnit  int64
		quantity  int
		commBps   int
		initialD  int64
		remainder int64
		finalD    int64
	}

	var eligibleTotal int64
	metaList := make([]itemMeta, len(items))
	for i, it := range items {
		if it.BaseUnitPriceCents < 0 {
			return nil, fmt.Errorf("item base unit price cannot be negative: %d", it.BaseUnitPriceCents)
		}
		if it.Quantity <= 0 {
			return nil, fmt.Errorf("item quantity must be positive: %d", it.Quantity)
		}
		if it.CommissionRateBps < 0 || it.CommissionRateBps > 10000 {
			return nil, fmt.Errorf("commission bps out of range: %d", it.CommissionRateBps)
		}
		subtotal := it.BaseUnitPriceCents * int64(it.Quantity)
		eligibleTotal += subtotal
		metaList[i] = itemMeta{
			index:    i,
			id:       it.ID,
			subtotal: subtotal,
			baseUnit: it.BaseUnitPriceCents,
			quantity: it.Quantity,
			commBps:  it.CommissionRateBps,
		}
	}

	actualDiscount := fixedDiscountCents
	if actualDiscount > eligibleTotal {
		actualDiscount = eligibleTotal
	}

	if eligibleTotal > 0 && actualDiscount > 0 {
		var sumInitial int64
		for i := range metaList {
			metaList[i].initialD = (actualDiscount * metaList[i].subtotal) / eligibleTotal
			metaList[i].remainder = (actualDiscount * metaList[i].subtotal) % eligibleTotal
			metaList[i].finalD = metaList[i].initialD
			sumInitial += metaList[i].initialD
		}

		leftoverCents := int(actualDiscount - sumInitial)
		if leftoverCents > 0 {
			// Sort helper slice for remainder distribution
			type remItem struct {
				idx       int
				remainder int64
				subtotal  int64
				id        string
			}
			remList := make([]remItem, len(metaList))
			for i, m := range metaList {
				remList[i] = remItem{
					idx:       m.index,
					remainder: m.remainder,
					subtotal:  m.subtotal,
					id:        m.id,
				}
			}

			// Deterministic sort:
			// 1. Remainder descending
			// 2. Subtotal descending
			// 3. ID ascending
			for a := 0; a < len(remList)-1; a++ {
				for b := a + 1; b < len(remList); b++ {
					shouldSwap := false
					if remList[b].remainder > remList[a].remainder {
						shouldSwap = true
					} else if remList[b].remainder == remList[a].remainder {
						if remList[b].subtotal > remList[a].subtotal {
							shouldSwap = true
						} else if remList[b].subtotal == remList[a].subtotal {
							if remList[b].id < remList[a].id {
								shouldSwap = true
							}
						}
					}
					if shouldSwap {
						remList[a], remList[b] = remList[b], remList[a]
					}
				}
			}

			for c := 0; c < leftoverCents && c < len(remList); c++ {
				metaList[remList[c].idx].finalD++
			}
		}
	}

	results := make([]LineSplitDiscountResult, len(items))
	for i, m := range metaList {
		totalPaid := m.subtotal - m.finalD
		totalCommBase := m.subtotal - m.finalD
		totalComm := RoundBps(totalCommBase, m.commBps)
		sellerNet := totalCommBase - totalComm

		unitDiscount := m.finalD / int64(m.quantity)
		unitPaid := m.baseUnit - unitDiscount
		unitCommBase := m.baseUnit - unitDiscount

		results[i] = LineSplitDiscountResult{
			BaseUnitPriceCents:          m.baseUnit,
			SellerDiscountUnitCents:     unitDiscount,
			ZamkSubsidyUnitCents:        0,
			CustomerPaidUnitPriceCents:  unitPaid,
			CommissionBaseUnitCents:     unitCommBase,
			Quantity:                    m.quantity,
			TotalSellerDiscountCents:    m.finalD,
			TotalZamkSubsidyCents:       0,
			TotalCustomerPaidCents:      totalPaid,
			TotalCommissionBaseCents:    totalCommBase,
			CommissionRateBps:           m.commBps,
			TotalCommissionChargedCents: totalComm,
			SellerNetEarningTotalCents:  sellerNet,
		}
	}

	return results, nil
}

// mulDivFloor returns floor(a*b/c) for non-negative operands without int64 overflow.
func mulDivFloor(a, b, c int64) int64 {
	if c <= 0 {
		return 0
	}
	r := new(big.Int).Mul(big.NewInt(a), big.NewInt(b))
	return r.Quo(r, big.NewInt(c)).Int64()
}

func mulMod(a, b, c int64) int64 {
	if c <= 0 {
		return 0
	}
	r := new(big.Int).Mul(big.NewInt(a), big.NewInt(b))
	return r.Rem(r, big.NewInt(c)).Int64()
}

// AllocateCappedCofundedDiscount applies an order-level cap (max_discount_cents) to co-funded percent
// lines. The capped total is split between seller and ZAMK in proportion to their uncapped totals:
//
//	seller_target = floor(cap * S / T), zamk_target = cap - seller_target   (T = S + Z)
//
// so the rounding remainder of the split goes to ZAMK and the seller never pays more than its share.
// ZAMK subsidy is stored per unit (order_item_promotions requires total = unit * quantity), so it is
// assigned in whole units per line, never above the line's uncapped unit subsidy; a remainder smaller
// than one unit step is carried by the seller only while the seller stays within its uncapped share.
// Seller discount is spread over lines in proportion to the uncapped seller discount (largest
// remainder, then larger line, then id), never above the uncapped line amount.
// Each line keeps: commission_base = base - seller, customer_paid = commission_base - zamk.
func AllocateCappedCofundedDiscount(capCents int64, lines []LineSplitDiscountResult, ids []string) ([]LineSplitDiscountResult, error) {
	if capCents <= 0 {
		return nil, fmt.Errorf("discount cap must be positive: %d", capCents)
	}
	if len(lines) != len(ids) {
		return nil, fmt.Errorf("lines and ids length mismatch")
	}
	var S, Z int64
	for _, l := range lines {
		if l.Quantity <= 0 {
			return nil, fmt.Errorf("item quantity must be positive: %d", l.Quantity)
		}
		S += l.TotalSellerDiscountCents
		Z += l.TotalZamkSubsidyCents
	}
	T := S + Z
	out := make([]LineSplitDiscountResult, len(lines))
	copy(out, lines)
	if T <= capCents {
		return out, nil
	}

	sellerTarget := mulDivFloor(capCents, S, T)
	zamkTarget := capCents - sellerTarget

	type ord struct {
		idx  int
		rem  int64
		size int64
		id   string
	}
	sortOrd := func(list []ord) {
		sort.SliceStable(list, func(a, b int) bool {
			if list[a].rem != list[b].rem {
				return list[a].rem > list[b].rem
			}
			if list[a].size != list[b].size {
				return list[a].size > list[b].size
			}
			return list[a].id < list[b].id
		})
	}

	// ZAMK: whole units per line, proportional, never above the uncapped unit subsidy.
	zamkUnit := make([]int64, len(lines))
	var zamkAssigned int64
	zOrder := make([]ord, len(lines))
	for i, l := range lines {
		zamkUnit[i] = mulDivFloor(l.ZamkSubsidyUnitCents, capCents, T)
		zamkAssigned += zamkUnit[i] * int64(l.Quantity)
		zOrder[i] = ord{idx: i, rem: mulMod(l.ZamkSubsidyUnitCents, capCents, T), size: l.TotalZamkSubsidyCents, id: ids[i]}
	}
	sortOrd(zOrder)
	for progress := true; progress; {
		progress = false
		for _, o := range zOrder {
			q := int64(lines[o.idx].Quantity)
			if zamkUnit[o.idx] < lines[o.idx].ZamkSubsidyUnitCents && zamkAssigned+q <= zamkTarget {
				zamkUnit[o.idx]++
				zamkAssigned += q
				progress = true
			}
		}
	}

	// Seller: the rest of the cap, never above its uncapped total.
	sellerTotal := capCents - zamkAssigned
	if sellerTotal > S {
		sellerTotal = S
	}
	sellerLine := make([]int64, len(lines))
	var sellerAssigned int64
	sOrder := make([]ord, len(lines))
	for i, l := range lines {
		sellerLine[i] = mulDivFloor(sellerTotal, l.TotalSellerDiscountCents, S)
		sellerAssigned += sellerLine[i]
		sOrder[i] = ord{idx: i, rem: mulMod(sellerTotal, l.TotalSellerDiscountCents, S), size: l.TotalSellerDiscountCents, id: ids[i]}
	}
	sortOrd(sOrder)
	for progress := true; progress && sellerAssigned < sellerTotal; {
		progress = false
		for _, o := range sOrder {
			if sellerAssigned >= sellerTotal {
				break
			}
			if sellerLine[o.idx] < lines[o.idx].TotalSellerDiscountCents {
				sellerLine[o.idx]++
				sellerAssigned++
				progress = true
			}
		}
	}

	for i, l := range lines {
		q := int64(l.Quantity)
		gross := l.BaseUnitPriceCents * q
		sellerUnit := sellerLine[i] / q
		commBase := gross - sellerLine[i]
		comm := RoundBps(commBase, l.CommissionRateBps)
		out[i] = LineSplitDiscountResult{
			BaseUnitPriceCents:          l.BaseUnitPriceCents,
			SellerDiscountUnitCents:     sellerUnit,
			ZamkSubsidyUnitCents:        zamkUnit[i],
			CustomerPaidUnitPriceCents:  l.BaseUnitPriceCents - sellerUnit - zamkUnit[i],
			CommissionBaseUnitCents:     l.BaseUnitPriceCents - sellerUnit,
			Quantity:                    l.Quantity,
			TotalSellerDiscountCents:    sellerLine[i],
			TotalZamkSubsidyCents:       zamkUnit[i] * q,
			TotalCustomerPaidCents:      commBase - zamkUnit[i]*q,
			TotalCommissionBaseCents:    commBase,
			CommissionRateBps:           l.CommissionRateBps,
			TotalCommissionChargedCents: comm,
			SellerNetEarningTotalCents:  commBase - comm,
		}
	}
	return out, nil
}
