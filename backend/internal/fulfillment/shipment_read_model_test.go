package fulfillment_test

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/eirfefrggrvgrgrtbg/shop-zamk/backend/internal/fulfillment"
	"github.com/eirfefrggrvgrgrtbg/shop-zamk/backend/internal/orders"
	"github.com/eirfefrggrvgrgrtbg/shop-zamk/backend/internal/platform/postgres"
	"github.com/eirfefrggrvgrgrtbg/shop-zamk/backend/internal/testutil"
)

type shipmentReadFixture struct {
	db       *pgxpool.Pool
	pgClient *postgres.Client
	repo     *fulfillment.Repository
	svc      *fulfillment.Service
	handler  *fulfillment.Handler

	adminID    uuid.UUID
	customerID uuid.UUID
	categoryID uuid.UUID

	createdOrderIDs    []uuid.UUID
	createdProductIDs  []uuid.UUID
	createdSellerIDs   []uuid.UUID
	createdUserIDs     []uuid.UUID
	createdCategoryIDs []uuid.UUID
	createdColorIDs    []uuid.UUID
	createdSizeSysIDs  []uuid.UUID
}

func setupShipmentReadFixture(t *testing.T) *shipmentReadFixture {
	t.Helper()
	ctx := context.Background()

	dsn := testutil.GetTestDatabaseURL()
	require.Equal(t, testutil.CanonicalTestDatabaseDSN, dsn, "must use exact canonical test database DSN")

	pgClient, err := postgres.NewClient(ctx, dsn)
	require.NoError(t, err)
	testutil.AssertTestDatabase(t, pgClient.Pool)

	db := pgClient.Pool
	repo := fulfillment.NewRepository(db)
	ordersRepo := orders.NewRepository(db)
	svc := fulfillment.NewService(repo, ordersRepo, pgClient, &mockPayoutsService{}, nil, nil)
	handler := fulfillment.NewHandler(svc)

	f := &shipmentReadFixture{
		db:       db,
		pgClient: pgClient,
		repo:     repo,
		svc:      svc,
		handler:  handler,
	}

	t.Cleanup(func() {
		cleanupCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		defer pgClient.Close()

		testutil.AssertTestDatabase(t, db)

		if len(f.createdOrderIDs) > 0 {
			if _, err := db.Exec(cleanupCtx, `DELETE FROM orders WHERE id = ANY($1)`, f.createdOrderIDs); err != nil {
				t.Errorf("cleanup orders failed: %v", err)
			}
		}
		if len(f.createdProductIDs) > 0 {
			if _, err := db.Exec(cleanupCtx, `DELETE FROM products WHERE id = ANY($1)`, f.createdProductIDs); err != nil {
				t.Errorf("cleanup products failed: %v", err)
			}
		}
		if len(f.createdColorIDs) > 0 {
			if _, err := db.Exec(cleanupCtx, `DELETE FROM colors WHERE id = ANY($1)`, f.createdColorIDs); err != nil {
				t.Errorf("cleanup colors failed: %v", err)
			}
		}
		if len(f.createdSizeSysIDs) > 0 {
			if _, err := db.Exec(cleanupCtx, `DELETE FROM size_systems WHERE id = ANY($1)`, f.createdSizeSysIDs); err != nil {
				t.Errorf("cleanup size_systems failed: %v", err)
			}
		}
		if len(f.createdCategoryIDs) > 0 {
			if _, err := db.Exec(cleanupCtx, `DELETE FROM categories WHERE id = ANY($1)`, f.createdCategoryIDs); err != nil {
				t.Errorf("cleanup categories failed: %v", err)
			}
		}
		if len(f.createdSellerIDs) > 0 {
			if _, err := db.Exec(cleanupCtx, `DELETE FROM sellers WHERE id = ANY($1)`, f.createdSellerIDs); err != nil {
				t.Errorf("cleanup sellers failed: %v", err)
			}
		}
		if len(f.createdUserIDs) > 0 {
			if _, err := db.Exec(cleanupCtx, `DELETE FROM users WHERE id = ANY($1)`, f.createdUserIDs); err != nil {
				t.Errorf("cleanup users failed: %v", err)
			}
		}
	})

	f.adminID = uuid.New()
	_, err = db.Exec(ctx, `
		INSERT INTO users (id, name, email, password_hash, role, status, created_at, updated_at)
		VALUES ($1, 'Shipment Admin', $2, 'hash', 'admin', 'active', now(), now())
	`, f.adminID, "admin-"+f.adminID.String()+"@zamk.test")
	require.NoError(t, err)
	f.createdUserIDs = append(f.createdUserIDs, f.adminID)

	f.customerID = uuid.New()
	_, err = db.Exec(ctx, `
		INSERT INTO users (id, name, email, password_hash, role, status, created_at, updated_at)
		VALUES ($1, 'Buyer Ivan', $2, 'hash', 'customer', 'active', now(), now())
	`, f.customerID, "buyer-"+f.customerID.String()+"@zamk.test")
	require.NoError(t, err)
	f.createdUserIDs = append(f.createdUserIDs, f.customerID)

	f.categoryID = uuid.New()
	_, err = db.Exec(ctx, `
		INSERT INTO categories (id, name, slug, created_at, updated_at)
		VALUES ($1, 'Shipment Test Category', $2, now(), now())
	`, f.categoryID, "cat-"+f.categoryID.String())
	require.NoError(t, err)
	f.createdCategoryIDs = append(f.createdCategoryIDs, f.categoryID)

	return f
}

func (f *shipmentReadFixture) createSeller(t *testing.T, brandName string) uuid.UUID {
	t.Helper()
	ctx := context.Background()
	sellerID := uuid.New()
	_, err := f.db.Exec(ctx, `
		INSERT INTO sellers (id, brand_name, slug, contact_email, status, created_at, updated_at)
		VALUES ($1, $2, $3, $4, 'active', now(), now())
	`, sellerID, brandName, "seller-"+sellerID.String(), "seller-"+sellerID.String()+"@zamk.test")
	require.NoError(t, err)
	f.createdSellerIDs = append(f.createdSellerIDs, sellerID)
	return sellerID
}

type itemFixtureSpec struct {
	title            string
	quantity         int
	orderItemColor   *string
	orderItemSize    *string
	orderItemSKU     *string
	orderItemImg     *string
	variantColorText *string
	variantSizeText  *string
	dictColorNameRU  *string
	dictSizeValue    *string
	productMainImg   *string
}

func (f *shipmentReadFixture) createOrderItemWithSpec(
	t *testing.T,
	orderID, fulfillmentID, sellerID uuid.UUID,
	spec itemFixtureSpec,
) (orderItemID, productID, variantID uuid.UUID) {
	t.Helper()
	ctx := context.Background()

	productID = uuid.New()
	_, err := f.db.Exec(ctx, `
		INSERT INTO products (id, seller_id, category_id, title, slug, price_cents, main_image_url, status, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, 250000, $6, 'published', now(), now())
	`, productID, sellerID, f.categoryID, spec.title, "prod-"+productID.String(), spec.productMainImg)
	require.NoError(t, err)
	f.createdProductIDs = append(f.createdProductIDs, productID)

	var colorID *uuid.UUID
	if spec.dictColorNameRU != nil {
		cid := uuid.New()
		_, err := f.db.Exec(ctx, `
			INSERT INTO colors (id, code, name_ru, is_active)
			VALUES ($1, $2, $3, true)
		`, cid, "col-"+cid.String()[:8], *spec.dictColorNameRU)
		require.NoError(t, err)
		f.createdColorIDs = append(f.createdColorIDs, cid)
		colorID = &cid
	}

	var sizeValueID *uuid.UUID
	if spec.dictSizeValue != nil {
		sysID := uuid.New()
		_, err := f.db.Exec(ctx, `
			INSERT INTO size_systems (id, code, name, is_active)
			VALUES ($1, $2, 'INT', true)
		`, sysID, "sys-"+sysID.String()[:8])
		require.NoError(t, err)
		f.createdSizeSysIDs = append(f.createdSizeSysIDs, sysID)

		svID := uuid.New()
		_, err = f.db.Exec(ctx, `
			INSERT INTO size_values (id, size_system_id, value, sort_order, is_active)
			VALUES ($1, $2, $3, 1, true)
		`, svID, sysID, *spec.dictSizeValue)
		require.NoError(t, err)
		sizeValueID = &svID
	}

	variantID = uuid.New()
	sku := fmt.Sprintf("SKU-%s", variantID.String()[:8])
	if spec.orderItemSKU != nil {
		sku = *spec.orderItemSKU
	}
	barcode := fmt.Sprintf("46%011d", time.Now().UnixNano()%90000000000+10000000000)
	_, err = f.db.Exec(ctx, `
		INSERT INTO product_variants (id, product_id, sku, seller_sku, barcode, size, color, color_id, size_value_id, price_cents, is_active, created_at, updated_at)
		VALUES ($1, $2, $3, $3, $4, $5, $6, $7, $8, 250000, true, now(), now())
	`, variantID, productID, sku, barcode, spec.variantSizeText, spec.variantColorText, colorID, sizeValueID)
	require.NoError(t, err)

	orderItemID = uuid.New()
	_, err = f.db.Exec(ctx, `
		INSERT INTO order_items (
			id, order_id, order_fulfillment_id, product_id, product_variant_id, seller_id,
			title, product_slug, variant_size, variant_color, sku, image_url,
			price_cents, quantity, subtotal_price_cents, picked_quantity, created_at
		) VALUES (
			$1, $2, $3, $4, $5, $6,
			$7, $8, $9, $10, $11, $12,
			250000, $13, $14, $13, now()
		)
	`,
		orderItemID, orderID, fulfillmentID, productID, variantID, sellerID,
		spec.title, "prod-"+productID.String(), spec.orderItemSize, spec.orderItemColor, spec.orderItemSKU, spec.orderItemImg,
		spec.quantity, int64(spec.quantity)*250000,
	)
	require.NoError(t, err)
	return orderItemID, productID, variantID
}

func strPtr(s string) *string {
	return &s
}

func TestShipmentReadModel_ListAndDetail(t *testing.T) {
	f := setupShipmentReadFixture(t)
	ctx := context.Background()

	sellerAlphaID := f.createSeller(t, "Atelier Alpha")
	sellerBetaID := f.createSeller(t, "Dom Beta")

	// Create a split order with two fulfillments (one per seller)
	orderID := uuid.New()
	orderNumber := "ORD-" + orderID.String()[:8]
	deliveryMethod := "Курьер 1-2 дня"
	customerName := "Анна Каренина"
	customerPhone := "+79991112233"
	deliveryAddress := "г. Москва, ул. Тверская, д. 10, кв. 5"

	_, err := f.db.Exec(ctx, `
		INSERT INTO orders (
			id, user_id, order_number, status, total_price_cents, currency,
			customer_name, customer_phone, customer_email, delivery_address, delivery_method_name,
			created_at, updated_at
		) VALUES (
			$1, $2, $3, 'shipped', 750000, 'RUB',
			$4, $5, 'anna@zamk.test', $6, $7,
			now(), now()
		)
	`, orderID, f.customerID, orderNumber, customerName, customerPhone, deliveryAddress, deliveryMethod)
	require.NoError(t, err)
	f.createdOrderIDs = append(f.createdOrderIDs, orderID)

	packedAtAlpha := time.Now().UTC().Add(-3 * time.Hour).Truncate(time.Microsecond)
	packedAtBeta := time.Now().UTC().Add(-2 * time.Hour).Truncate(time.Microsecond)

	fulfillmentAlphaID := uuid.New()
	_, err = f.db.Exec(ctx, `
		INSERT INTO order_fulfillments (id, order_id, seller_id, status, subtotal_cents, commission_bps, seller_amount_cents, packed_at, created_at, updated_at)
		VALUES ($1, $2, $3, 'shipped', 500000, 900, 455000, $4, now(), now())
	`, fulfillmentAlphaID, orderID, sellerAlphaID, packedAtAlpha)
	require.NoError(t, err)

	fulfillmentBetaID := uuid.New()
	_, err = f.db.Exec(ctx, `
		INSERT INTO order_fulfillments (id, order_id, seller_id, status, subtotal_cents, commission_bps, seller_amount_cents, packed_at, created_at, updated_at)
		VALUES ($1, $2, $3, 'delivered', 250000, 900, 227500, $4, now(), now())
	`, fulfillmentBetaID, orderID, sellerBetaID, packedAtBeta)
	require.NoError(t, err)

	// Alpha fulfillment has 2 item positions: quantity 2 + quantity 3 => itemsCount = 2, unitsCount = 5
	alphaItem1ID, alphaProd1ID, _ := f.createOrderItemWithSpec(t, orderID, fulfillmentAlphaID, sellerAlphaID, itemFixtureSpec{
		title:          "Пальто кашемировое Alpha",
		quantity:       2,
		orderItemColor: strPtr("Графит"),
		orderItemSize:  strPtr("M"),
		orderItemSKU:   strPtr("ALPHA-COAT-M"),
		orderItemImg:   strPtr("https://cdn.zamk.test/alpha-coat.jpg"),
	})
	alphaItem2ID, alphaProd2ID, _ := f.createOrderItemWithSpec(t, orderID, fulfillmentAlphaID, sellerAlphaID, itemFixtureSpec{
		title:           "Шарф шерстяной Alpha",
		quantity:        3,
		dictColorNameRU: strPtr("Молочный"),
		dictSizeValue:   strPtr("ONE SIZE"),
		productMainImg:  strPtr("https://cdn.zamk.test/alpha-scarf.jpg"),
	})

	// Beta fulfillment has 1 item position: quantity 1 => itemsCount = 1, unitsCount = 1, with NO color/size
	betaItem1ID, betaProd1ID, _ := f.createOrderItemWithSpec(t, orderID, fulfillmentBetaID, sellerBetaID, itemFixtureSpec{
		title:    "Сумка кожаная Beta",
		quantity: 1,
	})

	// Create Shipment Alpha: shipped, with carrier & tracking
	shipmentAlphaID := uuid.New()
	shippedAtAlpha := time.Now().UTC().Add(-90 * time.Minute).Truncate(time.Microsecond)
	carrierAlpha := "СДЭК"
	trackingAlpha := "CDEK-ALPHA-777"
	trackingURLAlpha := "https://cdek.test/track/CDEK-ALPHA-777"
	_, err = f.db.Exec(ctx, `
		INSERT INTO shipments (id, order_id, fulfillment_id, status, carrier, tracking_number, tracking_url, shipped_at, created_at, updated_at)
		VALUES ($1, $2, $3, 'shipped', $4, $5, $6, $7, $7, $7)
	`, shipmentAlphaID, orderID, fulfillmentAlphaID, carrierAlpha, trackingAlpha, trackingURLAlpha, shippedAtAlpha)
	require.NoError(t, err)

	// Create Shipment Beta: delivered, with nil optional carrier/tracking and preserved deliveredAt
	shipmentBetaID := uuid.New()
	shippedAtBeta := time.Now().UTC().Add(-80 * time.Minute).Truncate(time.Microsecond)
	deliveredAtBeta := time.Now().UTC().Add(-15 * time.Minute).Truncate(time.Microsecond)
	_, err = f.db.Exec(ctx, `
		INSERT INTO shipments (id, order_id, fulfillment_id, status, carrier, tracking_number, tracking_url, shipped_at, delivered_at, created_at, updated_at)
		VALUES ($1, $2, $3, 'delivered', NULL, NULL, NULL, $4, $5, $4, $5)
	`, shipmentBetaID, orderID, fulfillmentBetaID, shippedAtBeta, deliveredAtBeta)
	require.NoError(t, err)

	// -------------------------------------------------------------------------
	// Test List Read Model (A, B, C, D, E, F, K, L, N)
	// -------------------------------------------------------------------------
	r := chi.NewRouter()
	r.Get("/api/admin/shipments", f.handler.ListAdminShipments)
	r.Get("/api/admin/shipments/{id}", f.handler.GetAdminShipment)
	r.Patch("/api/admin/shipments/{id}/status", func(w http.ResponseWriter, req *http.Request) {
		ctxWithAdmin := context.WithValue(req.Context(), "userID", f.adminID)
		f.handler.UpdateShipmentStatus(w, req.WithContext(ctxWithAdmin))
	})

	reqList := httptest.NewRequest(http.MethodGet, "/api/admin/shipments?limit=100", nil)
	rrList := httptest.NewRecorder()
	r.ServeHTTP(rrList, reqList)
	require.Equal(t, http.StatusOK, rrList.Code)

	rawListBytes := rrList.Body.Bytes()

	// F. List does not expose unnecessary full recipient PII
	rawListStr := string(rawListBytes)
	assert.NotContains(t, rawListStr, "customerName")
	assert.NotContains(t, rawListStr, "customerPhone")
	assert.NotContains(t, rawListStr, "customerEmail")
	assert.NotContains(t, rawListStr, "deliveryAddress")
	assert.NotContains(t, rawListStr, customerName)
	assert.NotContains(t, rawListStr, customerPhone)
	assert.NotContains(t, rawListStr, deliveryAddress)

	// N. Existing admin shipment API consumers remain compatible (unmarshal into []fulfillment.Shipment)
	var legacyList []fulfillment.Shipment
	require.NoError(t, json.Unmarshal(rawListBytes, &legacyList))

	var listItems []fulfillment.AdminShipmentListItem
	require.NoError(t, json.Unmarshal(rawListBytes, &listItems))

	byID := make(map[uuid.UUID]fulfillment.AdminShipmentListItem)
	for _, item := range listItems {
		byID[item.ID] = item
	}

	require.Contains(t, byID, shipmentAlphaID)
	require.Contains(t, byID, shipmentBetaID)

	// Verify Shipment Alpha in list (A, B, C, D, E)
	alphaRow := byID[shipmentAlphaID]
	assert.Equal(t, shipmentAlphaID, alphaRow.ID)
	assert.Equal(t, shipmentAlphaID, alphaRow.ShipmentID)
	assert.Equal(t, orderID, alphaRow.OrderID)
	require.NotNil(t, alphaRow.OrderNumber)
	assert.Equal(t, orderNumber, *alphaRow.OrderNumber) // A. orderNumber
	require.NotNil(t, alphaRow.FulfillmentID)
	assert.Equal(t, fulfillmentAlphaID, *alphaRow.FulfillmentID) // E. split order maps to Alpha fulfillment
	require.NotNil(t, alphaRow.SellerID)
	assert.Equal(t, sellerAlphaID, *alphaRow.SellerID)
	require.NotNil(t, alphaRow.SellerName)
	assert.Equal(t, "Atelier Alpha", *alphaRow.SellerName) // B. canonical seller display field
	assert.Equal(t, "shipped", alphaRow.Status)
	require.NotNil(t, alphaRow.Carrier)
	assert.Equal(t, carrierAlpha, *alphaRow.Carrier)
	require.NotNil(t, alphaRow.TrackingNumber)
	assert.Equal(t, trackingAlpha, *alphaRow.TrackingNumber)
	require.NotNil(t, alphaRow.TrackingUrl)
	assert.Equal(t, trackingURLAlpha, *alphaRow.TrackingUrl)
	require.NotNil(t, alphaRow.DeliveryMethodName)
	assert.Equal(t, deliveryMethod, *alphaRow.DeliveryMethodName)
	assert.Equal(t, 2, alphaRow.ItemsCount) // C. itemsCount correct (2 positions in Alpha)
	assert.Equal(t, 5, alphaRow.UnitsCount) // D. unitsCount sums quantities (2 + 3 = 5)
	require.NotNil(t, alphaRow.ShippedAt)
	assert.WithinDuration(t, shippedAtAlpha, *alphaRow.ShippedAt, time.Second)
	assert.Nil(t, alphaRow.DeliveredAt)

	// Verify Shipment Beta in list (E, K, L)
	betaRow := byID[shipmentBetaID]
	assert.Equal(t, shipmentBetaID, betaRow.ShipmentID)
	require.NotNil(t, betaRow.OrderNumber)
	assert.Equal(t, orderNumber, *betaRow.OrderNumber)
	require.NotNil(t, betaRow.FulfillmentID)
	assert.Equal(t, fulfillmentBetaID, *betaRow.FulfillmentID) // E. split order maps to Beta fulfillment
	require.NotNil(t, betaRow.SellerID)
	assert.Equal(t, sellerBetaID, *betaRow.SellerID)
	require.NotNil(t, betaRow.SellerName)
	assert.Equal(t, "Dom Beta", *betaRow.SellerName)
	assert.Equal(t, "delivered", betaRow.Status)
	assert.Nil(t, betaRow.Carrier)        // K. missing optional carrier is nil
	assert.Nil(t, betaRow.TrackingNumber) // K. missing optional trackingNumber is nil
	assert.Nil(t, betaRow.TrackingUrl)    // K. missing optional trackingUrl is nil
	assert.Equal(t, 1, betaRow.ItemsCount)
	assert.Equal(t, 1, betaRow.UnitsCount)
	require.NotNil(t, betaRow.ShippedAt)
	require.NotNil(t, betaRow.DeliveredAt) // L. delivered timestamps preserved
	assert.WithinDuration(t, shippedAtBeta, *betaRow.ShippedAt, time.Second)
	assert.WithinDuration(t, deliveredAtBeta, *betaRow.DeliveredAt, time.Second)

	// -------------------------------------------------------------------------
	// Test Detail Read Model (G, H, I, J, K, L, N)
	// -------------------------------------------------------------------------
	reqDetailAlpha := httptest.NewRequest(http.MethodGet, "/api/admin/shipments/"+shipmentAlphaID.String(), nil)
	rrDetailAlpha := httptest.NewRecorder()
	r.ServeHTTP(rrDetailAlpha, reqDetailAlpha)
	require.Equal(t, http.StatusOK, rrDetailAlpha.Code)

	// N. Legacy consumer compatibility on detail endpoint
	var legacyDetail fulfillment.Shipment
	require.NoError(t, json.Unmarshal(rrDetailAlpha.Body.Bytes(), &legacyDetail))
	assert.Equal(t, shipmentAlphaID, legacyDetail.ID)
	assert.Equal(t, "shipped", legacyDetail.Status)

	var detailAlpha fulfillment.AdminShipmentDetail
	require.NoError(t, json.Unmarshal(rrDetailAlpha.Body.Bytes(), &detailAlpha))

	// G. Detail includes shipment/order/fulfillment/seller/recipient context
	assert.Equal(t, shipmentAlphaID, detailAlpha.ID)
	assert.Equal(t, shipmentAlphaID, detailAlpha.ShipmentID)
	assert.Equal(t, orderID, detailAlpha.OrderID)
	require.NotNil(t, detailAlpha.OrderNumber)
	assert.Equal(t, orderNumber, *detailAlpha.OrderNumber)
	require.NotNil(t, detailAlpha.FulfillmentID)
	assert.Equal(t, fulfillmentAlphaID, *detailAlpha.FulfillmentID)
	require.NotNil(t, detailAlpha.FulfillmentStatus)
	assert.Equal(t, "shipped", *detailAlpha.FulfillmentStatus)
	require.NotNil(t, detailAlpha.SellerID)
	assert.Equal(t, sellerAlphaID, *detailAlpha.SellerID)
	require.NotNil(t, detailAlpha.SellerName)
	assert.Equal(t, "Atelier Alpha", *detailAlpha.SellerName)
	require.NotNil(t, detailAlpha.DeliveryMethodName)
	assert.Equal(t, deliveryMethod, *detailAlpha.DeliveryMethodName)
	require.NotNil(t, detailAlpha.CustomerName)
	assert.Equal(t, customerName, *detailAlpha.CustomerName)
	require.NotNil(t, detailAlpha.CustomerPhone)
	assert.Equal(t, customerPhone, *detailAlpha.CustomerPhone)
	require.NotNil(t, detailAlpha.DeliveryAddress)
	assert.Equal(t, deliveryAddress, *detailAlpha.DeliveryAddress)
	require.NotNil(t, detailAlpha.PackedAt)
	assert.WithinDuration(t, packedAtAlpha, *detailAlpha.PackedAt, time.Second)

	// H & I. Detail items belong ONLY to Alpha fulfillment and quantities are exact
	assert.Equal(t, 2, detailAlpha.ItemsCount)
	assert.Equal(t, 5, detailAlpha.UnitsCount)
	require.Len(t, detailAlpha.Items, 2)

	assert.Equal(t, alphaItem1ID, detailAlpha.Items[0].OrderItemID)
	assert.Equal(t, alphaProd1ID, detailAlpha.Items[0].ProductID)
	assert.Equal(t, "Пальто кашемировое Alpha", detailAlpha.Items[0].ProductTitle)
	assert.Equal(t, 2, detailAlpha.Items[0].Quantity)
	require.NotNil(t, detailAlpha.Items[0].VariantColor)
	assert.Equal(t, "Графит", *detailAlpha.Items[0].VariantColor) // J. from order_items snapshot
	require.NotNil(t, detailAlpha.Items[0].VariantSize)
	assert.Equal(t, "M", *detailAlpha.Items[0].VariantSize)
	require.NotNil(t, detailAlpha.Items[0].ImageURL)
	assert.Equal(t, "https://cdn.zamk.test/alpha-coat.jpg", *detailAlpha.Items[0].ImageURL)

	assert.Equal(t, alphaItem2ID, detailAlpha.Items[1].OrderItemID)
	assert.Equal(t, alphaProd2ID, detailAlpha.Items[1].ProductID)
	assert.Equal(t, "Шарф шерстяной Alpha", detailAlpha.Items[1].ProductTitle)
	assert.Equal(t, 3, detailAlpha.Items[1].Quantity)
	require.NotNil(t, detailAlpha.Items[1].VariantColor)
	assert.Equal(t, "Молочный", *detailAlpha.Items[1].VariantColor) // J. from canonical variant colors dictionary
	require.NotNil(t, detailAlpha.Items[1].VariantSize)
	assert.Equal(t, "ONE SIZE", *detailAlpha.Items[1].VariantSize) // J. from canonical variant size_values dictionary
	require.NotNil(t, detailAlpha.Items[1].ImageURL)
	assert.Equal(t, "https://cdn.zamk.test/alpha-scarf.jpg", *detailAlpha.Items[1].ImageURL)

	// Check Beta Detail: only Beta item, nil variant color/size when absent in DB
	reqDetailBeta := httptest.NewRequest(http.MethodGet, "/api/admin/shipments/"+shipmentBetaID.String(), nil)
	rrDetailBeta := httptest.NewRecorder()
	r.ServeHTTP(rrDetailBeta, reqDetailBeta)
	require.Equal(t, http.StatusOK, rrDetailBeta.Code)

	var detailBeta fulfillment.AdminShipmentDetail
	require.NoError(t, json.Unmarshal(rrDetailBeta.Body.Bytes(), &detailBeta))
	assert.Equal(t, 1, detailBeta.ItemsCount)
	assert.Equal(t, 1, detailBeta.UnitsCount)
	require.Len(t, detailBeta.Items, 1)
	assert.Equal(t, betaItem1ID, detailBeta.Items[0].OrderItemID)
	assert.Equal(t, betaProd1ID, detailBeta.Items[0].ProductID)
	assert.Equal(t, "Сумка кожаная Beta", detailBeta.Items[0].ProductTitle)
	assert.Equal(t, 1, detailBeta.Items[0].Quantity)
	assert.Nil(t, detailBeta.Items[0].VariantColor) // J. not invented when absent
	assert.Nil(t, detailBeta.Items[0].VariantSize)  // J. not invented when absent
	assert.Nil(t, detailBeta.Carrier)               // K. nil carrier preserved
	assert.Nil(t, detailBeta.TrackingNumber)        // K. nil trackingNumber preserved
	require.NotNil(t, detailBeta.DeliveredAt)       // L. deliveredAt preserved
	assert.WithinDuration(t, deliveredAtBeta, *detailBeta.DeliveredAt, time.Second)

	// -------------------------------------------------------------------------
	// M. Existing status/action semantics unchanged (update tracking on shipped)
	// -------------------------------------------------------------------------
	patchBody, err := json.Marshal(fulfillment.UpdateShipmentStatusRequest{
		Status:         "shipped",
		Carrier:        strPtr("Boxberry"),
		TrackingNumber: strPtr("BXB-999"),
	})
	require.NoError(t, err)
	reqPatch := httptest.NewRequest(http.MethodPatch, "/api/admin/shipments/"+shipmentAlphaID.String()+"/status", bytes.NewReader(patchBody))
	rrPatch := httptest.NewRecorder()
	r.ServeHTTP(rrPatch, reqPatch)
	require.Equal(t, http.StatusOK, rrPatch.Code)

	updatedAlpha, err := f.svc.GetAdminShipment(ctx, shipmentAlphaID)
	require.NoError(t, err)
	require.NotNil(t, updatedAlpha.Carrier)
	assert.Equal(t, "Boxberry", *updatedAlpha.Carrier)
	require.NotNil(t, updatedAlpha.TrackingNumber)
	assert.Equal(t, "BXB-999", *updatedAlpha.TrackingNumber)
}

func TestPackedFulfillmentReadSources_IncludeSellerAndCounts(t *testing.T) {
	f := setupShipmentReadFixture(t)
	ctx := context.Background()

	sellerID := f.createSeller(t, "Maison Packed")
	orderID := uuid.New()
	orderNumber := "ORD-PACK-" + orderID.String()[:6]
	packedAt := time.Now().UTC().Add(-45 * time.Minute).Truncate(time.Microsecond)

	_, err := f.db.Exec(ctx, `
		INSERT INTO orders (
			id, user_id, order_number, status, total_price_cents, currency,
			customer_name, customer_phone, customer_email, delivery_address, delivery_method_name,
			created_at, updated_at
		) VALUES (
			$1, $2, $3, 'packed', 750000, 'RUB',
			'Пётр Гринёв', '+79990001122', 'petr@zamk.test', 'г. Оренбург, Крепостная 1', 'Экспресс 3 часа',
			now(), now()
		)
	`, orderID, f.customerID, orderNumber)
	require.NoError(t, err)
	f.createdOrderIDs = append(f.createdOrderIDs, orderID)

	fulfillmentID := uuid.New()
	_, err = f.db.Exec(ctx, `
		INSERT INTO order_fulfillments (id, order_id, seller_id, status, subtotal_cents, commission_bps, seller_amount_cents, packed_at, created_at, updated_at)
		VALUES ($1, $2, $3, 'packed', 750000, 900, 682500, $4, now(), now())
	`, fulfillmentID, orderID, sellerID, packedAt)
	require.NoError(t, err)

	f.createOrderItemWithSpec(t, orderID, fulfillmentID, sellerID, itemFixtureSpec{
		title:    "Рубашка льняная",
		quantity: 2,
	})
	f.createOrderItemWithSpec(t, orderID, fulfillmentID, sellerID, itemFixtureSpec{
		title:    "Брюки льняные",
		quantity: 1,
	})

	// 1. Check ListAdminFulfillments(status='packed') (current AdminShipments.tsx packed queue source)
	packedStatus := "packed"
	fulfillments, err := f.svc.ListAdminFulfillments(ctx, 50, 0, &packedStatus)
	require.NoError(t, err)

	var foundFulfillment *fulfillment.Fulfillment
	for i := range fulfillments {
		if fulfillments[i].ID == fulfillmentID {
			foundFulfillment = &fulfillments[i]
			break
		}
	}
	require.NotNil(t, foundFulfillment, "packed fulfillment must be returned by ListAdminFulfillments")
	assert.Equal(t, fulfillmentID, foundFulfillment.FulfillmentID)
	require.NotNil(t, foundFulfillment.OrderNumber)
	assert.Equal(t, orderNumber, *foundFulfillment.OrderNumber)
	require.NotNil(t, foundFulfillment.SellerName)
	assert.Equal(t, "Maison Packed", *foundFulfillment.SellerName)
	assert.Equal(t, 2, foundFulfillment.ItemsCount)
	assert.Equal(t, 3, foundFulfillment.UnitsCount)
	require.NotNil(t, foundFulfillment.PackedAt)
	assert.WithinDuration(t, packedAt, *foundFulfillment.PackedAt, time.Second)
	require.NotNil(t, foundFulfillment.DeliveryMethodName)
	assert.Equal(t, "Экспресс 3 часа", *foundFulfillment.DeliveryMethodName)

	// 2. Check GetDispatchQueue (GET /api/admin/fulfillments/dispatch)
	dispatchQueue, err := f.svc.GetDispatchQueue(ctx)
	require.NoError(t, err)

	var foundQueueItem *fulfillment.DispatchQueueItem
	for i := range dispatchQueue {
		if dispatchQueue[i].FulfillmentID == fulfillmentID {
			foundQueueItem = &dispatchQueue[i]
			break
		}
	}
	require.NotNil(t, foundQueueItem, "packed fulfillment must be returned by GetDispatchQueue")
	assert.Equal(t, orderNumber, foundQueueItem.OrderNumber)
	assert.Equal(t, sellerID, foundQueueItem.SellerID)
	require.NotNil(t, foundQueueItem.SellerName)
	assert.Equal(t, "Maison Packed", *foundQueueItem.SellerName)
	assert.Equal(t, 2, foundQueueItem.ItemsCount)
	assert.Equal(t, 3, foundQueueItem.UnitsCount)
	assert.Equal(t, 3, foundQueueItem.TotalQuantity)
	require.NotNil(t, foundQueueItem.PackedAt)
	assert.WithinDuration(t, packedAt, *foundQueueItem.PackedAt, time.Second)
	require.NotNil(t, foundQueueItem.DeliveryMethodName)
	assert.Equal(t, "Экспресс 3 часа", *foundQueueItem.DeliveryMethodName)
}
