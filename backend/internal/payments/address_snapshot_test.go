package payments

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	"github.com/eirfefrggrvgrgrtbg/shop-zamk/backend/internal/addresses"
	"github.com/eirfefrggrvgrgrtbg/shop-zamk/backend/internal/orders"
	"github.com/eirfefrggrvgrgrtbg/shop-zamk/backend/internal/platform/postgres"
)

// SHOP.ADDRESSES.1: an order carries its own copy of the delivery address (orders.delivery_address, written once at
// checkout). Editing or deleting the saved address afterwards never reaches an existing order.

func TestOrderKeepsItsAddressSnapshot(t *testing.T) {
	f := newPayFixture(t, "", "")
	p, v := f.variant(500000, 5)
	u := f.buyer()
	book := addresses.NewService(addresses.NewRepository(&postgres.Client{Pool: f.db}))

	saved, err := book.CreateAddress(context.Background(), u, addresses.CreateAddressRequest{
		RecipientName: "Анна Иванова", Phone: "+7 999 000-11-22", City: "Москва", Street: "Тверская", House: "1", Apartment: ptr("12"), IsDefault: true,
	})
	require.NoError(t, err)

	// what the Shop sends: the address copied into the order request as text
	snapshot := "г. " + saved.City + ", ул. " + saved.Street + ", д. " + saved.House + ", кв. " + *saved.Apartment
	f.addToCart(u, p, v, 1)
	b, _ := json.Marshal(orders.CreateOrderRequest{CustomerName: saved.RecipientName, CustomerPhone: saved.Phone, CustomerEmail: "anna@buyer.test", DeliveryAddress: snapshot, DeliveryMethodID: f.dm})
	r := httptest.NewRequest(http.MethodPost, "/api/customer/orders", bytes.NewReader(b))
	r = r.WithContext(context.WithValue(r.Context(), "userID", u))
	r.Header.Set("Idempotency-Key", uuid.NewString())
	w := httptest.NewRecorder()
	f.orderH.CreateOrder(w, r)
	require.Equal(t, http.StatusCreated, w.Code, w.Body.String())
	var created struct{ ID uuid.UUID }
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &created))

	read := func() (string, string, string) {
		var addr, name, phone string
		require.NoError(t, f.db.QueryRow(context.Background(), `SELECT delivery_address, customer_name, customer_phone FROM orders WHERE id=$1`, created.ID).Scan(&addr, &name, &phone))
		return addr, name, phone
	}
	addr, name, phone := read()
	require.Equal(t, snapshot, addr)

	// the customer edits, then deletes the saved address
	_, err = book.UpdateAddress(context.Background(), saved.ID, u, addresses.UpdateAddressRequest{City: ptr("Казань"), Street: ptr("Баумана"), RecipientName: ptr("Другой Получатель"), Phone: ptr("+7 900 111-22-33")})
	require.NoError(t, err)
	a2, n2, p2 := read()
	require.Equal(t, []string{addr, name, phone}, []string{a2, n2, p2}, "an edit of the saved address does not change the order")

	require.NoError(t, book.DeleteAddress(context.Background(), saved.ID, u))
	a3, n3, p3 := read()
	require.Equal(t, []string{addr, name, phone}, []string{a3, n3, p3}, "deleting the saved address does not break the order")
	require.Equal(t, 1, f.ordersOf(u))
	// the order is still payable: the delivery data it holds is intact
	f.mustPay(u, created.ID)
}

func ptr(s string) *string { return &s }
