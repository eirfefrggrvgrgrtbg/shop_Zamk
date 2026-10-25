package payments

import (
	"bytes"
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

	"github.com/eirfefrggrvgrgrtbg/shop-zamk/backend/internal/addresses"
	"github.com/eirfefrggrvgrgrtbg/shop-zamk/backend/internal/platform/postgres"
)

// SHOP.DELIVERY.DETAILS.1: the order keeps the postal code and the courier comment of its checkout as its own
// snapshot, next to the address, name and phone. Disposable database, real handlers.

type deliveryOrder struct {
	Status int
	Body   map[string]any
	Raw    string
	ID     uuid.UUID
}

func (f *payFixture) place(u uuid.UUID, key string, over map[string]any) deliveryOrder {
	f.t.Helper()
	body := map[string]any{
		"customerName": "Анна Покупатель", "customerPhone": "+79990000001", "customerEmail": "anna@buyer.test",
		"deliveryAddress": "г. Москва, ул. Тверская, д. 1, кв. 12", "deliveryMethodId": f.dm,
	}
	for k, v := range over {
		body[k] = v
	}
	b, _ := json.Marshal(body)
	r := httptest.NewRequest(http.MethodPost, "/api/customer/orders", bytes.NewReader(b))
	r = r.WithContext(context.WithValue(r.Context(), "userID", u))
	if key != "" {
		r.Header.Set("Idempotency-Key", key)
	}
	w := httptest.NewRecorder()
	f.orderH.CreateOrder(w, r)
	out := deliveryOrder{Status: w.Code, Raw: w.Body.String()}
	_ = json.Unmarshal(w.Body.Bytes(), &out.Body)
	if id, ok := out.Body["id"].(string); ok {
		out.ID, _ = uuid.Parse(id)
	}
	return out
}

// read is what the customer gets from GET /customer/orders/{id}.
func (f *payFixture) read(u, orderID uuid.UUID) deliveryOrder {
	f.t.Helper()
	r := httptest.NewRequest(http.MethodGet, "/api/customer/orders/"+orderID.String(), nil)
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("id", orderID.String())
	r = r.WithContext(context.WithValue(context.WithValue(r.Context(), chi.RouteCtxKey, rctx), "userID", u))
	w := httptest.NewRecorder()
	f.orderH.GetCustomerOrder(w, r)
	out := deliveryOrder{Status: w.Code, Raw: w.Body.String(), ID: orderID}
	_ = json.Unmarshal(w.Body.Bytes(), &out.Body)
	return out
}

func (f *payFixture) cartOf(u uuid.UUID) (uuid.UUID, uuid.UUID) {
	p, v := f.variant(500000, 20)
	f.addToCart(u, p, v, 1)
	return p, v
}

func TestOrderDeliveryDetails(t *testing.T) {
	f := newPayFixture(t, "", "")
	ctx := context.Background()
	book := addresses.NewService(addresses.NewRepository(&postgres.Client{Pool: f.db}))

	t.Run("A: a saved address with postal code and comment — the order gets its own copy, and keeps it", func(t *testing.T) {
		u := f.buyer()
		f.cartOf(u)
		saved, err := book.CreateAddress(ctx, u, addresses.CreateAddressRequest{
			RecipientName: "Анна Иванова", Phone: "+7 999 000-11-22", City: "Москва", Street: "Тверская", House: "1", Apartment: ptr("12"),
			PostalCode: ptr("101000"), Comment: ptr("Домофон 12, позвонить за час"), IsDefault: true,
		})
		require.NoError(t, err)
		placed := f.place(u, uuid.NewString(), map[string]any{"customerName": saved.RecipientName, "customerPhone": saved.Phone, "deliveryPostalCode": *saved.PostalCode, "deliveryCourierComment": *saved.Comment})
		require.Equal(t, http.StatusCreated, placed.Status, placed.Raw)
		got := f.read(u, placed.ID)
		require.Equal(t, http.StatusOK, got.Status)
		assert.Equal(t, "101000", got.Body["deliveryPostalCode"])
		assert.Equal(t, "Домофон 12, позвонить за час", got.Body["deliveryCourierComment"])
		assert.Equal(t, "г. Москва, ул. Тверская, д. 1, кв. 12", got.Body["deliveryAddress"])
		before := got.Raw

		// D: edit the saved address (postal code, comment, everything)
		_, err = book.UpdateAddress(ctx, saved.ID, u, addresses.UpdateAddressRequest{City: ptr("Казань"), PostalCode: ptr("420111"), Comment: ptr("Другой комментарий"), RecipientName: ptr("Другой")})
		require.NoError(t, err)
		assert.JSONEq(t, before, f.read(u, placed.ID).Raw, "D: editing the saved address leaves the order untouched")
		// E: delete it
		require.NoError(t, book.DeleteAddress(ctx, saved.ID, u))
		assert.JSONEq(t, before, f.read(u, placed.ID).Raw, "E: deleting the saved address leaves the order untouched")
		f.mustPay(u, placed.ID) // the order is still payable
	})

	t.Run("B: manual entry with postal code and comment (trimmed)", func(t *testing.T) {
		u := f.buyer()
		f.cartOf(u)
		placed := f.place(u, uuid.NewString(), map[string]any{"deliveryPostalCode": "  101000 ", "deliveryCourierComment": "  Оставить у консьержа  "})
		require.Equal(t, http.StatusCreated, placed.Status, placed.Raw)
		assert.Equal(t, "101000", placed.Body["deliveryPostalCode"])
		got := f.read(u, placed.ID)
		assert.Equal(t, "Оставить у консьержа", got.Body["deliveryCourierComment"])
		var dbPostal, dbComment string
		require.NoError(t, f.db.QueryRow(ctx, `SELECT delivery_postal_code, delivery_courier_comment FROM orders WHERE id=$1`, placed.ID).Scan(&dbPostal, &dbComment))
		assert.Equal(t, []string{"101000", "Оставить у консьержа"}, []string{dbPostal, dbComment})
	})

	t.Run("C: neither given (or blank): checkout works, nothing is invented, the keys are absent", func(t *testing.T) {
		for i, over := range []map[string]any{nil, {"deliveryPostalCode": "   ", "deliveryCourierComment": ""}} {
			u := f.buyer()
			f.cartOf(u)
			placed := f.place(u, uuid.NewString(), over)
			require.Equal(t, http.StatusCreated, placed.Status, "case %d: %s", i, placed.Raw)
			got := f.read(u, placed.ID)
			assert.NotContains(t, got.Body, "deliveryPostalCode")
			assert.NotContains(t, got.Body, "deliveryCourierComment")
			assert.Equal(t, 1, f.count(`SELECT count(*) FROM orders WHERE id=$1 AND delivery_postal_code IS NULL AND delivery_courier_comment IS NULL`, placed.ID))
		}
	})

	t.Run("G: an old order (placed before the columns existed) is read as before", func(t *testing.T) {
		u := f.buyer()
		old := uuid.New()
		f.exec(`INSERT INTO orders (id, user_id, status, total_price_cents, currency, customer_name, customer_phone, customer_email, delivery_address)
			VALUES ($1,$2,'awaiting_payment',100000,'RUB','Старый Заказ','+79990000009','old@buyer.test','г. Тула, ул. Старая, д. 3')`, old, u)
		got := f.read(u, old)
		require.Equal(t, http.StatusOK, got.Status, got.Raw)
		assert.Equal(t, "г. Тула, ул. Старая, д. 3", got.Body["deliveryAddress"])
		assert.NotContains(t, got.Body, "deliveryPostalCode")
		assert.NotContains(t, got.Body, "deliveryCourierComment")
	})

	t.Run("validation: too long, bad characters, control characters — refused before anything is reserved", func(t *testing.T) {
		u := f.buyer()
		p, v := f.cartOf(u)
		reservedBefore := f.reserved(v)
		_ = p
		for _, c := range []struct {
			field string
			value string
		}{
			{"deliveryCourierComment", strings.Repeat("я", 501)}, {"deliveryCourierComment", "строка\x00с нулём"}, {"deliveryCourierComment", "перенос\nстроки"},
			{"deliveryPostalCode", strings.Repeat("1", 13)}, {"deliveryPostalCode", "10<1>"},
		} {
			placed := f.place(u, uuid.NewString(), map[string]any{c.field: c.value})
			require.Equal(t, http.StatusBadRequest, placed.Status, "%s=%q %s", c.field, c.value, placed.Raw)
			errBody, _ := placed.Body["error"].(map[string]any)
			assert.Equal(t, "validation_error", errBody["code"])
			assert.Equal(t, c.field, errBody["field"])
			assert.Regexp(t, `[А-Яа-я]`, errBody["message"])
		}
		assert.Equal(t, 0, f.ordersOf(u))
		assert.Equal(t, reservedBefore, f.reserved(v), "nothing was reserved")
		assert.Equal(t, 1, f.cartLines(u), "the cart is intact")
		// the limit itself is allowed
		ok := f.place(u, uuid.NewString(), map[string]any{"deliveryCourierComment": strings.Repeat("я", 500), "deliveryPostalCode": "123456789012"})
		require.Equal(t, http.StatusCreated, ok.Status, ok.Raw)
	})

	t.Run("HTML in the comment is stored as text and never comes back as markup", func(t *testing.T) {
		u := f.buyer()
		f.cartOf(u)
		placed := f.place(u, uuid.NewString(), map[string]any{"deliveryCourierComment": `<script>alert(1)</script> & <b>x</b>`})
		require.Equal(t, http.StatusCreated, placed.Status, placed.Raw)
		got := f.read(u, placed.ID)
		assert.NotContains(t, got.Raw, "<script>", "the JSON escapes angle brackets")
		assert.Equal(t, `<script>alert(1)</script> & <b>x</b>`, got.Body["deliveryCourierComment"], "the text itself is unchanged")
	})

	t.Run("F: a foreign address id gives nothing: checkout takes no address id, only what the customer sends", func(t *testing.T) {
		a, b := f.buyer(), f.buyer()
		secret, err := book.CreateAddress(ctx, a, addresses.CreateAddressRequest{RecipientName: "Секретный Получатель", Phone: "+7 999 111-22-33", City: "Тайноград", Street: "Секретная", House: "7", PostalCode: ptr("999999"), Comment: ptr("секретный комментарий")})
		require.NoError(t, err)
		f.cartOf(b)
		placed := f.place(b, uuid.NewString(), map[string]any{"deliveryAddressId": secret.ID, "addressId": secret.ID})
		require.Equal(t, http.StatusCreated, placed.Status, placed.Raw)
		for _, leak := range []string{"Секретн", "Тайноград", "999999", "секретный"} {
			assert.NotContains(t, placed.Raw, leak)
		}
		assert.NotContains(t, f.read(b, placed.ID).Body, "deliveryPostalCode")
		// and the owner's order is not readable by the other buyer
		f.cartOf(a)
		own := f.place(a, uuid.NewString(), map[string]any{"deliveryPostalCode": "101000", "deliveryCourierComment": "для А"})
		require.Equal(t, http.StatusCreated, own.Status)
		stranger := f.read(b, own.ID)
		assert.Equal(t, http.StatusNotFound, stranger.Status)
		assert.NotContains(t, stranger.Raw, "для А")
	})

	t.Run("the same checkout key with a changed comment is a conflict (the snapshot cannot be swapped silently)", func(t *testing.T) {
		u := f.buyer()
		f.cartOf(u)
		key := uuid.NewString()
		first := f.place(u, key, map[string]any{"deliveryCourierComment": "первый"})
		require.Equal(t, http.StatusCreated, first.Status, first.Raw)
		again := f.place(u, key, map[string]any{"deliveryCourierComment": "первый"})
		assert.Equal(t, first.ID, again.ID, "the identical repeat returns the same order")
		changed := f.place(u, key, map[string]any{"deliveryCourierComment": "второй"})
		assert.Equal(t, http.StatusConflict, changed.Status, changed.Raw)
		assert.Equal(t, 1, f.ordersOf(u))
		assert.Equal(t, "первый", f.read(u, first.ID).Body["deliveryCourierComment"])
	})

	t.Run("staff: the admin order detail carries the snapshot; money and delivery method are untouched", func(t *testing.T) {
		u := f.buyer()
		f.cartOf(u)
		with := f.place(u, uuid.NewString(), map[string]any{"deliveryPostalCode": "101000", "deliveryCourierComment": "звонок"})
		require.Equal(t, http.StatusCreated, with.Status)
		d, err := f.ordersSvc.GetAdminOrder(ctx, with.ID)
		require.NoError(t, err)
		require.NotNil(t, d.DeliveryPostalCode)
		require.NotNil(t, d.DeliveryCourierComment)
		assert.Equal(t, "101000", *d.DeliveryPostalCode)
		assert.Equal(t, "звонок", *d.DeliveryCourierComment)
		// K: pricing and delivery method as before: same totals with and without the optional fields
		u2 := f.buyer()
		f.cartOf(u2)
		without := f.place(u2, uuid.NewString(), nil)
		require.Equal(t, http.StatusCreated, without.Status)
		assert.Equal(t, with.Body["totalPriceCents"], without.Body["totalPriceCents"])
		assert.Equal(t, with.Body["deliveryPriceCents"], without.Body["deliveryPriceCents"])
		assert.Equal(t, with.Body["deliveryMethodId"], without.Body["deliveryMethodId"])
	})
}
