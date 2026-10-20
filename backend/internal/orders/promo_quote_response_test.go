package orders

import (
	"encoding/json"
	"errors"
	"fmt"
	"testing"

	"github.com/google/uuid"

	"github.com/eirfefrggrvgrgrtbg/shop-zamk/backend/internal/marketing"
)

func TestPromoQuoteResponseExposesOnlyCustomerFacingFields(t *testing.T) {
	calc := &marketing.CheckoutPromoCalculation{
		PromoCodeID:              uuid.New(),
		CampaignID:               uuid.New(),
		SellerID:                 uuid.New(),
		FundingMode:              "cofunded",
		Code:                     "SAVE10",
		TotalSellerDiscountCents: 700,
		TotalZamkSubsidyCents:    300,
		TotalOrderDiscountCents:  1000,
		TotalCustomerPaidCents:   9000,
		PromotedLines:            []marketing.PromotedLineResult{{SellerID: uuid.New()}},
	}

	raw, err := json.Marshal(newPromoQuoteResponse(calc))
	if err != nil {
		t.Fatal(err)
	}
	var got map[string]any
	if err := json.Unmarshal(raw, &got); err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 || got["code"] != "SAVE10" || got["discountCents"] != float64(1000) {
		t.Fatalf("unexpected quote payload: %s", raw)
	}
}

func TestNormalizePromoLookupErrorMapsUnknownCodeToCustomerRejection(t *testing.T) {
	if !errors.Is(normalizePromoLookupError(fmt.Errorf("lookup: %w", marketing.ErrPromoCodeNotFound)), marketing.ErrPromoNotFound) {
		t.Fatal("unknown code must map to ErrPromoNotFound")
	}
	other := errors.New("boom")
	if normalizePromoLookupError(other) != other {
		t.Fatal("other errors must pass through unchanged")
	}
}
