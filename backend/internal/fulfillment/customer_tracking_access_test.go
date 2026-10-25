package fulfillment_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// SHOP.ORDER.TRACKING.1: a customer reads the fulfillments (tracking) of their own order only. Someone else's order
// and a missing order both answer 404 with no internal text; the answer carries no seller finance or internal fields.

func (k deliveryKit) customerFulfillmentsHTTP(orderID uuid.UUID, customer *uuid.UUID) *httptest.ResponseRecorder {
	req := httptest.NewRequest(http.MethodGet, "/api/customer/orders/"+orderID.String()+"/fulfillments", nil)
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("orderId", orderID.String())
	ctx := context.WithValue(req.Context(), chi.RouteCtxKey, rctx)
	if customer != nil {
		ctx = context.WithValue(ctx, "userID", *customer)
	}
	rec := httptest.NewRecorder()
	k.handler.GetCustomerOrderFulfillments(rec, req.WithContext(ctx))
	return rec
}

func TestCustomerTracking_OwnOrderOnly(t *testing.T) {
	f := newFBOFixture(t)
	k := f.deliveryKit()
	orderID, _, shipID, _, _ := f.shippedSingleVariantOrder()
	stranger := uuid.New()

	t.Run("owner sees the parcel with its real status, no finance or internal fields", func(t *testing.T) {
		rec := k.customerFulfillmentsHTTP(orderID, &f.customerID)
		require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
		var out []map[string]any
		require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &out))
		require.Len(t, out, 1)
		assert.Equal(t, "shipped", out[0]["status"])
		for _, forbidden := range []string{"commissionBps", "sellerAmountCents", "subtotalCents", "commission", "payout", "ledger", "reservation", "allocation", "barcode", "allocatedunits", "unitcode"} {
			assert.NotContains(t, strings.ToLower(rec.Body.String()), strings.ToLower(forbidden), forbidden)
		}
	})

	t.Run("another customer gets 404 and no data", func(t *testing.T) {
		rec := k.customerFulfillmentsHTTP(orderID, &stranger)
		require.Equal(t, http.StatusNotFound, rec.Code, rec.Body.String())
		assert.NotContains(t, rec.Body.String(), shipID.String())
		assert.NotContains(t, strings.ToLower(rec.Body.String()), "unauthorized")
		assert.NotContains(t, rec.Body.String(), "internal")
	})

	t.Run("a missing order answers the same 404 (no existence oracle)", func(t *testing.T) {
		own := k.customerFulfillmentsHTTP(uuid.New(), &f.customerID)
		foreign := k.customerFulfillmentsHTTP(orderID, &stranger)
		require.Equal(t, http.StatusNotFound, own.Code)
		assert.Equal(t, own.Body.String(), foreign.Body.String())
	})

	t.Run("no session: 401", func(t *testing.T) {
		assert.Equal(t, http.StatusUnauthorized, k.customerFulfillmentsHTTP(orderID, nil).Code)
	})
}
