package fulfillment

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/eirfefrggrvgrgrtbg/shop-zamk/backend/internal/orders"
	"github.com/eirfefrggrvgrgrtbg/shop-zamk/backend/internal/platform/postgres"
	"github.com/eirfefrggrvgrgrtbg/shop-zamk/backend/internal/testutil"
)

func TestCustomerMultiFulfillmentTracking(t *testing.T) {
	dsn := testutil.GetTestDatabaseURL()
	if dsn == "" {
		t.Skip("TEST_DATABASE_URL not set")
	}

	ctx := context.Background()
	postgresClient, err := postgres.NewClient(ctx, dsn)
	if err != nil {
		t.Fatalf("failed to connect: %v", err)
	}
	defer postgresClient.Close()

	// 1. Mandatory hard test database guard BEFORE ANY mutation
	testutil.AssertTestDatabase(t, postgresClient.Pool)

	db := postgresClient.Pool
	repo := NewRepository(db)

	t.Run("Repository_TransactionScopedIsolation", func(t *testing.T) {
		// Defensive guard verification inside subtest
		testutil.AssertTestDatabase(t, db)

		tx, err := db.Begin(ctx)
		require.NoError(t, err)

		// Register cleanup immediately after beginning tx, before first mutation
		t.Cleanup(func() {
			if err := tx.Rollback(ctx); err != nil && !errors.Is(err, pgx.ErrTxClosed) {
				t.Errorf("cleanup rollback failed: %v", err)
			}
		})

		customerID := uuid.New()
		orderID := uuid.New()
		seller1ID := uuid.New()
		seller2ID := uuid.New()
		seller3ID := uuid.New()
		seller4ID := uuid.New()

		f1ID := uuid.New()
		f2ID := uuid.New()
		f3ID := uuid.New()
		f4ID := uuid.New()

		s1ID := uuid.New()
		s2ID := uuid.New()
		s4ID := uuid.New()

		customerEmail := fmt.Sprintf("cust_%s@example.com", customerID)
		seller1Email := fmt.Sprintf("s1_%s@example.com", seller1ID)
		seller2Email := fmt.Sprintf("s2_%s@example.com", seller2ID)
		seller3Email := fmt.Sprintf("s3_%s@example.com", seller3ID)
		seller4Email := fmt.Sprintf("s4_%s@example.com", seller4ID)

		seller1Slug := fmt.Sprintf("s1-%s", seller1ID)
		seller2Slug := fmt.Sprintf("s2-%s", seller2ID)
		seller3Slug := fmt.Sprintf("s3-%s", seller3ID)
		seller4Slug := fmt.Sprintf("s4-%s", seller4ID)

		// Users
		_, err = tx.Exec(ctx, "INSERT INTO users (id, name, email, password_hash) VALUES ($1, $2, $3, $4)", customerID, "Customer", customerEmail, "hash")
		require.NoError(t, err)

		// Sellers
		_, err = tx.Exec(ctx, "INSERT INTO sellers (id, brand_name, slug, contact_email) VALUES ($1, $2, $3, $4)", seller1ID, "Brand A", seller1Slug, seller1Email)
		require.NoError(t, err)
		_, err = tx.Exec(ctx, "INSERT INTO sellers (id, brand_name, slug, contact_email) VALUES ($1, $2, $3, $4)", seller2ID, "Brand B", seller2Slug, seller2Email)
		require.NoError(t, err)
		_, err = tx.Exec(ctx, "INSERT INTO sellers (id, brand_name, slug, contact_email) VALUES ($1, $2, $3, $4)", seller3ID, "Brand C", seller3Slug, seller3Email)
		require.NoError(t, err)
		_, err = tx.Exec(ctx, "INSERT INTO sellers (id, brand_name, slug, contact_email) VALUES ($1, $2, $3, $4)", seller4ID, "Brand D", seller4Slug, seller4Email)
		require.NoError(t, err)

		// Order
		_, err = tx.Exec(ctx, "INSERT INTO orders (id, user_id, status, total_price_cents, customer_name, customer_phone, customer_email, delivery_address) VALUES ($1, $2, 'shipped', 4000, 'Customer', '+1000', $3, 'Address')", orderID, customerID, customerEmail)
		require.NoError(t, err)

		// Fulfillments
		// A: shipped, has shipment A
		_, err = tx.Exec(ctx, "INSERT INTO order_fulfillments (id, order_id, seller_id, status) VALUES ($1, $2, $3, 'shipped')", f1ID, orderID, seller1ID)
		require.NoError(t, err)
		// B: shipped, has shipment B
		_, err = tx.Exec(ctx, "INSERT INTO order_fulfillments (id, order_id, seller_id, status) VALUES ($1, $2, $3, 'shipped')", f2ID, orderID, seller2ID)
		require.NoError(t, err)
		// C: packed, has NO shipment row (NULL shipment case)
		_, err = tx.Exec(ctx, "INSERT INTO order_fulfillments (id, order_id, seller_id, status) VALUES ($1, $2, $3, 'packed')", f3ID, orderID, seller3ID)
		require.NoError(t, err)
		// D: shipped, has shipment row D where tracking fields are NULL
		_, err = tx.Exec(ctx, "INSERT INTO order_fulfillments (id, order_id, seller_id, status) VALUES ($1, $2, $3, 'shipped')", f4ID, orderID, seller4ID)
		require.NoError(t, err)

		shippedAtA := time.Date(2026, 9, 25, 10, 0, 0, 0, time.UTC)
		shippedAtB := time.Date(2026, 9, 25, 12, 0, 0, 0, time.UTC)
		deliveredAtB := time.Date(2026, 9, 26, 15, 30, 0, 0, time.UTC)

		// Shipment A: carrier, trackingNumber, trackingUrl, shippedAt, deliveredAt = NULL
		_, err = tx.Exec(ctx, `
			INSERT INTO shipments (id, order_id, fulfillment_id, status, carrier, tracking_number, tracking_url, shipped_at, delivered_at)
			VALUES ($1, $2, $3, 'shipped', 'CarrierA', 'TRK-A', 'https://a.example/track/TRK-A', $4, NULL)
		`, s1ID, orderID, f1ID, shippedAtA)
		require.NoError(t, err)

		// Shipment B: carrier, trackingNumber, trackingUrl, shippedAt, deliveredAt = distinct value
		_, err = tx.Exec(ctx, `
			INSERT INTO shipments (id, order_id, fulfillment_id, status, carrier, tracking_number, tracking_url, shipped_at, delivered_at)
			VALUES ($1, $2, $3, 'delivered', 'CarrierB', 'TRK-B', 'https://b.example/track/TRK-B', $4, $5)
		`, s2ID, orderID, f2ID, shippedAtB, deliveredAtB)
		require.NoError(t, err)

		// Shipment D: shipment row exists, but all tracking fields are NULL
		_, err = tx.Exec(ctx, `
			INSERT INTO shipments (id, order_id, fulfillment_id, status, carrier, tracking_number, tracking_url, shipped_at, delivered_at)
			VALUES ($1, $2, $3, 'shipped', NULL, NULL, NULL, NULL, NULL)
		`, s4ID, orderID, f4ID)
		require.NoError(t, err)

		// Read fulfillments via repository with transaction
		fulfillments, err := repo.GetOrderFulfillmentsTx(ctx, tx, orderID)
		require.NoError(t, err)
		require.Len(t, fulfillments, 4)

		fulfillmentMap := make(map[uuid.UUID]Fulfillment)
		for _, f := range fulfillments {
			fulfillmentMap[f.ID] = f
		}

		// 1. Assert Fulfillment A
		fA, existsA := fulfillmentMap[f1ID]
		require.True(t, existsA, "fulfillment A must exist")
		require.NotNil(t, fA.ShipmentID)
		assert.Equal(t, s1ID, *fA.ShipmentID)
		require.NotNil(t, fA.ShipmentStatus)
		assert.Equal(t, "shipped", *fA.ShipmentStatus)
		require.NotNil(t, fA.Carrier)
		assert.Equal(t, "CarrierA", *fA.Carrier)
		require.NotNil(t, fA.TrackingNumber)
		assert.Equal(t, "TRK-A", *fA.TrackingNumber)
		require.NotNil(t, fA.TrackingUrl)
		assert.Equal(t, "https://a.example/track/TRK-A", *fA.TrackingUrl)
		require.NotNil(t, fA.ShippedAt)
		assert.Equal(t, shippedAtA.Unix(), fA.ShippedAt.Unix())
		assert.Nil(t, fA.DeliveredAt, "deliveredAt for A must be nil")

		// 2. Assert Fulfillment B
		fB, existsB := fulfillmentMap[f2ID]
		require.True(t, existsB, "fulfillment B must exist")
		require.NotNil(t, fB.ShipmentID)
		assert.Equal(t, s2ID, *fB.ShipmentID)
		require.NotNil(t, fB.ShipmentStatus)
		assert.Equal(t, "delivered", *fB.ShipmentStatus)
		require.NotNil(t, fB.Carrier)
		assert.Equal(t, "CarrierB", *fB.Carrier)
		require.NotNil(t, fB.TrackingNumber)
		assert.Equal(t, "TRK-B", *fB.TrackingNumber)
		require.NotNil(t, fB.TrackingUrl)
		assert.Equal(t, "https://b.example/track/TRK-B", *fB.TrackingUrl)
		require.NotNil(t, fB.ShippedAt)
		assert.Equal(t, shippedAtB.Unix(), fB.ShippedAt.Unix())
		require.NotNil(t, fB.DeliveredAt)
		assert.Equal(t, deliveredAtB.Unix(), fB.DeliveredAt.Unix())

		// 3. Assert Fulfillment C (No shipment row -> pure LEFT JOIN NULL)
		fC, existsC := fulfillmentMap[f3ID]
		require.True(t, existsC, "fulfillment C must exist")
		assert.Nil(t, fC.ShipmentID, "shipmentId for C must be nil")
		assert.Nil(t, fC.ShipmentStatus, "shipmentStatus for C must be nil")
		assert.Nil(t, fC.Carrier, "carrier for C must be nil (no leak from A or B)")
		assert.Nil(t, fC.TrackingNumber, "trackingNumber for C must be nil")
		assert.Nil(t, fC.TrackingUrl, "trackingUrl for C must be nil")
		assert.Nil(t, fC.ShippedAt, "shippedAt for C must be nil")
		assert.Nil(t, fC.DeliveredAt, "deliveredAt for C must be nil")

		// 4. Assert Fulfillment D (Shipment row exists, but tracking fields are NULL)
		fD, existsD := fulfillmentMap[f4ID]
		require.True(t, existsD, "fulfillment D must exist")
		require.NotNil(t, fD.ShipmentID)
		assert.Equal(t, s4ID, *fD.ShipmentID)
		require.NotNil(t, fD.ShipmentStatus)
		assert.Equal(t, "shipped", *fD.ShipmentStatus)
		assert.Nil(t, fD.Carrier, "carrier for D must be nil")
		assert.Nil(t, fD.TrackingNumber, "trackingNumber for D must be nil")
		assert.Nil(t, fD.TrackingUrl, "trackingUrl for D must be nil")
		assert.Nil(t, fD.ShippedAt, "shippedAt for D must be nil")
		assert.Nil(t, fD.DeliveredAt, "deliveredAt for D must be nil")

		// Verify no cross-assignment between A and B
		assert.NotEqual(t, *fA.Carrier, *fB.Carrier)
		assert.NotEqual(t, *fA.TrackingNumber, *fB.TrackingNumber)
		assert.NotEqual(t, *fA.TrackingUrl, *fB.TrackingUrl)
		assert.NotEqual(t, fA.ShippedAt.Unix(), fB.ShippedAt.Unix())
	})

	t.Run("Handler_CustomerDTOContractAndJSONSemantics", func(t *testing.T) {
		// Defensive guard verification inside subtest
		testutil.AssertTestDatabase(t, db)

		customerID := uuid.New()
		orderID := uuid.New()
		seller1ID := uuid.New()
		seller2ID := uuid.New()
		seller3ID := uuid.New()

		f1ID := uuid.New()
		f2ID := uuid.New()
		f3ID := uuid.New()

		s1ID := uuid.New()
		s2ID := uuid.New()

		customerEmail := fmt.Sprintf("cust_dto_%s@example.com", customerID)
		seller1Email := fmt.Sprintf("s1_dto_%s@example.com", seller1ID)
		seller2Email := fmt.Sprintf("s2_dto_%s@example.com", seller2ID)
		seller3Email := fmt.Sprintf("s3_dto_%s@example.com", seller3ID)

		seller1Slug := fmt.Sprintf("s1-dto-%s", seller1ID)
		seller2Slug := fmt.Sprintf("s2-dto-%s", seller2ID)
		seller3Slug := fmt.Sprintf("s3-dto-%s", seller3ID)

		// Register cleanup BEFORE any mutation, with exact UUID scoping
		t.Cleanup(func() {
			cleanCtx := context.Background()
			_, _ = db.Exec(cleanCtx, "DELETE FROM shipments WHERE order_id = $1", orderID)
			_, _ = db.Exec(cleanCtx, "DELETE FROM order_fulfillments WHERE order_id = $1", orderID)
			_, _ = db.Exec(cleanCtx, "DELETE FROM orders WHERE id = $1", orderID)
			_, _ = db.Exec(cleanCtx, "DELETE FROM sellers WHERE id IN ($1, $2, $3)", seller1ID, seller2ID, seller3ID)
			_, _ = db.Exec(cleanCtx, "DELETE FROM users WHERE id IN ($1, $2, $3, $4)", customerID, seller1ID, seller2ID, seller3ID)
		})

		// Users
		_, err := db.Exec(ctx, "INSERT INTO users (id, name, email, password_hash) VALUES ($1, $2, $3, $4)", customerID, "Customer DTO", customerEmail, "hash")
		require.NoError(t, err)

		// Sellers
		_, err = db.Exec(ctx, "INSERT INTO sellers (id, brand_name, slug, contact_email) VALUES ($1, $2, $3, $4)", seller1ID, "Brand DTO 1", seller1Slug, seller1Email)
		require.NoError(t, err)
		_, err = db.Exec(ctx, "INSERT INTO sellers (id, brand_name, slug, contact_email) VALUES ($1, $2, $3, $4)", seller2ID, "Brand DTO 2", seller2Slug, seller2Email)
		require.NoError(t, err)
		_, err = db.Exec(ctx, "INSERT INTO sellers (id, brand_name, slug, contact_email) VALUES ($1, $2, $3, $4)", seller3ID, "Brand DTO 3", seller3Slug, seller3Email)
		require.NoError(t, err)

		// Order
		_, err = db.Exec(ctx, "INSERT INTO orders (id, user_id, status, total_price_cents, customer_name, customer_phone, customer_email, delivery_address) VALUES ($1, $2, 'shipped', 3000, 'Customer DTO', '+1000', $3, 'Address')", orderID, customerID, customerEmail)
		require.NoError(t, err)

		// Fulfillments
		_, err = db.Exec(ctx, "INSERT INTO order_fulfillments (id, order_id, seller_id, status) VALUES ($1, $2, $3, 'shipped')", f1ID, orderID, seller1ID)
		require.NoError(t, err)
		_, err = db.Exec(ctx, "INSERT INTO order_fulfillments (id, order_id, seller_id, status) VALUES ($1, $2, $3, 'shipped')", f2ID, orderID, seller2ID)
		require.NoError(t, err)
		_, err = db.Exec(ctx, "INSERT INTO order_fulfillments (id, order_id, seller_id, status) VALUES ($1, $2, $3, 'packed')", f3ID, orderID, seller3ID)
		require.NoError(t, err)

		shippedAtA := time.Date(2026, 9, 25, 10, 0, 0, 0, time.UTC)
		shippedAtB := time.Date(2026, 9, 25, 12, 0, 0, 0, time.UTC)
		deliveredAtB := time.Date(2026, 9, 26, 15, 30, 0, 0, time.UTC)

		// Shipments
		_, err = db.Exec(ctx, `
			INSERT INTO shipments (id, order_id, fulfillment_id, status, carrier, tracking_number, tracking_url, shipped_at, delivered_at)
			VALUES ($1, $2, $3, 'shipped', 'CarrierA', 'TRK-A', 'https://a.example/track/TRK-A', $4, NULL)
		`, s1ID, orderID, f1ID, shippedAtA)
		require.NoError(t, err)

		_, err = db.Exec(ctx, `
			INSERT INTO shipments (id, order_id, fulfillment_id, status, carrier, tracking_number, tracking_url, shipped_at, delivered_at)
			VALUES ($1, $2, $3, 'delivered', 'CarrierB', 'TRK-B', 'https://b.example/track/TRK-B', $4, $5)
		`, s2ID, orderID, f2ID, shippedAtB, deliveredAtB)
		require.NoError(t, err)

		ordersRepo := orders.NewRepository(db)
		svc := NewService(repo, ordersRepo, postgresClient, nil, nil)
		handler := NewHandler(svc)

		req := httptest.NewRequest("GET", "/customer/orders/"+orderID.String()+"/fulfillments", nil)
		rCtx := chi.NewRouteContext()
		rCtx.URLParams.Add("orderId", orderID.String())
		req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rCtx))
		req = req.WithContext(context.WithValue(req.Context(), "userID", customerID))

		rec := httptest.NewRecorder()
		handler.GetCustomerOrderFulfillments(rec, req)

		require.Equal(t, http.StatusOK, rec.Code, "handler response must be 200 OK")

		// 1. Unmarshal into typed DTO
		var dtoList []CustomerFulfillmentResponse
		err = json.Unmarshal(rec.Body.Bytes(), &dtoList)
		require.NoError(t, err, "response body must deserialize into []CustomerFulfillmentResponse")
		require.Len(t, dtoList, 3)

		dtoMap := make(map[string]CustomerFulfillmentResponse)
		for _, d := range dtoList {
			dtoMap[d.ID] = d
		}

		// DTO checks for Fulfillment A
		dtoA, hasA := dtoMap[f1ID.String()]
		require.True(t, hasA)
		require.NotNil(t, dtoA.ShipmentStatus)
		assert.Equal(t, "shipped", *dtoA.ShipmentStatus)
		require.NotNil(t, dtoA.Carrier)
		assert.Equal(t, "CarrierA", *dtoA.Carrier)
		require.NotNil(t, dtoA.TrackingNumber)
		assert.Equal(t, "TRK-A", *dtoA.TrackingNumber)
		require.NotNil(t, dtoA.TrackingUrl)
		assert.Equal(t, "https://a.example/track/TRK-A", *dtoA.TrackingUrl)
		require.NotNil(t, dtoA.ShippedAt)
		parsedShippedA, err := time.Parse(time.RFC3339, *dtoA.ShippedAt)
		require.NoError(t, err, "shippedAt must be valid RFC3339")
		assert.Equal(t, shippedAtA.Unix(), parsedShippedA.Unix())
		assert.Nil(t, dtoA.DeliveredAt)

		// DTO checks for Fulfillment B
		dtoB, hasB := dtoMap[f2ID.String()]
		require.True(t, hasB)
		require.NotNil(t, dtoB.ShipmentStatus)
		assert.Equal(t, "delivered", *dtoB.ShipmentStatus)
		require.NotNil(t, dtoB.Carrier)
		assert.Equal(t, "CarrierB", *dtoB.Carrier)
		require.NotNil(t, dtoB.TrackingNumber)
		assert.Equal(t, "TRK-B", *dtoB.TrackingNumber)
		require.NotNil(t, dtoB.TrackingUrl)
		assert.Equal(t, "https://b.example/track/TRK-B", *dtoB.TrackingUrl)
		require.NotNil(t, dtoB.ShippedAt)
		parsedShippedB, err := time.Parse(time.RFC3339, *dtoB.ShippedAt)
		require.NoError(t, err, "shippedAt must be valid RFC3339")
		assert.Equal(t, shippedAtB.Unix(), parsedShippedB.Unix())
		require.NotNil(t, dtoB.DeliveredAt)
		parsedDeliveredB, err := time.Parse(time.RFC3339, *dtoB.DeliveredAt)
		require.NoError(t, err, "deliveredAt must be valid RFC3339")
		assert.Equal(t, deliveredAtB.Unix(), parsedDeliveredB.Unix())

		// DTO checks for Fulfillment C (NULL shipment)
		dtoC, hasC := dtoMap[f3ID.String()]
		require.True(t, hasC)
		assert.Nil(t, dtoC.ShipmentStatus)
		assert.Nil(t, dtoC.Carrier)
		assert.Nil(t, dtoC.TrackingNumber)
		assert.Nil(t, dtoC.TrackingUrl)
		assert.Nil(t, dtoC.ShippedAt)
		assert.Nil(t, dtoC.DeliveredAt)

		// 2. Unmarshal into raw JSON map to prove wire semantics (omitempty / keys presence)
		var jsonList []map[string]interface{}
		err = json.Unmarshal(rec.Body.Bytes(), &jsonList)
		require.NoError(t, err)
		require.Len(t, jsonList, 3)

		rawMap := make(map[string]map[string]interface{})
		for _, m := range jsonList {
			rawMap[m["id"].(string)] = m
		}

		rawA := rawMap[f1ID.String()]
		assert.Equal(t, "CarrierA", rawA["carrier"])
		assert.Equal(t, "TRK-A", rawA["trackingNumber"])
		assert.Equal(t, "https://a.example/track/TRK-A", rawA["trackingUrl"])
		parsedRawShippedA, err := time.Parse(time.RFC3339, rawA["shippedAt"].(string))
		require.NoError(t, err)
		assert.Equal(t, shippedAtA.Unix(), parsedRawShippedA.Unix())
		assert.Nil(t, rawA["deliveredAt"], "deliveredAt must be null/omitted in raw JSON")

		rawB := rawMap[f2ID.String()]
		assert.Equal(t, "CarrierB", rawB["carrier"])
		assert.Equal(t, "TRK-B", rawB["trackingNumber"])
		assert.Equal(t, "https://b.example/track/TRK-B", rawB["trackingUrl"])
		parsedRawShippedB, err := time.Parse(time.RFC3339, rawB["shippedAt"].(string))
		require.NoError(t, err)
		assert.Equal(t, shippedAtB.Unix(), parsedRawShippedB.Unix())
		parsedRawDeliveredB, err := time.Parse(time.RFC3339, rawB["deliveredAt"].(string))
		require.NoError(t, err)
		assert.Equal(t, deliveredAtB.Unix(), parsedRawDeliveredB.Unix())

		rawC := rawMap[f3ID.String()]
		assert.Nil(t, rawC["carrier"], "carrier must not leak into fulfillment C JSON")
		assert.Nil(t, rawC["trackingNumber"], "trackingNumber must not leak into fulfillment C JSON")
		assert.Nil(t, rawC["trackingUrl"], "trackingUrl must not leak into fulfillment C JSON")
		assert.Nil(t, rawC["shippedAt"], "shippedAt must not leak into fulfillment C JSON")
		assert.Nil(t, rawC["deliveredAt"], "deliveredAt must not leak into fulfillment C JSON")
	})
}
