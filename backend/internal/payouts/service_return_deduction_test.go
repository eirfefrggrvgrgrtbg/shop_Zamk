package payouts

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/eirfefrggrvgrgrtbg/shop-zamk/backend/internal/orders"
)

// Mock orders repository that can return specific order items
type mockOrdersRepo struct {
	items []orders.OrderItem
}

func (m *mockOrdersRepo) GetOrderItems(ctx context.Context, orderID uuid.UUID) ([]orders.OrderItem, error) {
	var res []orders.OrderItem
	for _, item := range m.items {
		if item.OrderID == orderID {
			res = append(res, item)
		}
	}
	return res, nil
}

func (m *mockOrdersRepo) GetOrder(ctx context.Context, orderID uuid.UUID) (*orders.Order, error) {
	return nil, nil
}

func (m *mockOrdersRepo) GetSellerIDByUserID(ctx context.Context, userID uuid.UUID) (uuid.UUID, error) {
	return uuid.Nil, nil
}

func TestProcessReturnDeduction(t *testing.T) {
	client := setupTestDB(t)
	defer client.Close()
	
	ctx := context.Background()
	tx, err := client.Pool.Begin(ctx)
	require.NoError(t, err)
	defer tx.Rollback(ctx)

	repo := NewRepository(client.Pool)

	// We need an order and order item
	suffix := uuid.NewString()[:8]
	sellerID := uuid.New()
	userID := uuid.New()
	_, err = client.Pool.Exec(ctx, "INSERT INTO users (id, name, phone, email, password_hash, role, created_at) VALUES ($1, 'Test', '+123', $2, 'hash', 'seller', now())", userID, fmt.Sprintf("test-%s@example.com", suffix))
	require.NoError(t, err)
	_, err = client.Pool.Exec(ctx, "INSERT INTO sellers (id, brand_name, slug, contact_email, status, created_at) VALUES ($1, 'test store', $2, $3, 'active', now())", sellerID, fmt.Sprintf("test-store-%s", suffix), fmt.Sprintf("test-%s@store.com", suffix))
	require.NoError(t, err)

	orderID := uuid.New()
	_, err = client.Pool.Exec(ctx, "INSERT INTO orders (id, user_id, status, total_price_cents, currency, customer_name, customer_phone, customer_email, delivery_address, created_at) VALUES ($1, $2, 'paid', 100000, 'RUB', 'Test', '+1', 'a@b.c', 'Addr', now())", orderID, userID)
	require.NoError(t, err)
	orderItemID := uuid.New()
	
	// Create a product variant first for the order item foreign key
	productID := uuid.New()
	categoryID := uuid.New()
	_, err = client.Pool.Exec(ctx, "INSERT INTO categories (id, name, slug) VALUES ($1, 'cat', $2)", categoryID, fmt.Sprintf("cat-%s", suffix))
	require.NoError(t, err)
	_, err = client.Pool.Exec(ctx, "INSERT INTO products (id, seller_id, category_id, title, slug, description, status, price_cents) VALUES ($1, $2, $3, 'P', $4, 'desc', 'published', 50000)", productID, sellerID, categoryID, fmt.Sprintf("p-%s", suffix))
	require.NoError(t, err)
	variantID := uuid.New()
	_, err = client.Pool.Exec(ctx, "INSERT INTO product_variants (id, product_id, sku, price_cents) VALUES ($1, $2, $3, 50000)", variantID, productID, fmt.Sprintf("sku-%s", suffix))
	require.NoError(t, err)

	fulfillmentID := uuid.New()
	_, err = client.Pool.Exec(ctx, "INSERT INTO order_fulfillments (id, order_id, seller_id, status) VALUES ($1, $2, $3, 'paid')", fulfillmentID, orderID, sellerID)
	require.NoError(t, err)

	_, err = client.Pool.Exec(ctx, "INSERT INTO order_items (id, order_id, order_fulfillment_id, product_id, product_variant_id, seller_id, title, product_slug, quantity, price_cents, subtotal_price_cents) VALUES ($1, $2, $3, $4, $5, $6, 'Title', 'slug', 2, 50000, 100000)", orderItemID, orderID, fulfillmentID, productID, variantID, sellerID)
	require.NoError(t, err)

	mockOrders := &mockOrdersRepo{
		items: []orders.OrderItem{
			{
				ID:       orderItemID,
				OrderID:  orderID,
				SellerID: sellerID,
				Quantity: 2,
			},
		},
	}
	
	svc := NewService(repo, client, nil, mockOrders, nil, nil)

	// Insert an initial seller_earning for 1000 RUB, available in 14 days
	availableAt := time.Now().AddDate(0, 0, 14).Round(time.Millisecond).UTC()
	err = repo.CreateLedgerEntryTx(ctx, tx, &SellerLedgerEntry{
		ID:          uuid.New(),
		SellerID:    sellerID,
		OrderID:     &orderID,
		OrderItemID: &orderItemID,
		Type:        "seller_earning",
		AmountCents: 100000, // 1000 RUB
		Currency:    "RUB",
		AvailableAt: &availableAt,
		CreatedAt:   time.Now(),
	})
	require.NoError(t, err)

	// Process a return for 1 unit out of 2. It should deduct 500 RUB.
	returnID := uuid.New()
	deductionItems := []ReturnItemDeduction{
		{
			OrderItemID: orderItemID,
			Quantity:    1,
		},
	}

	err = svc.ProcessReturnDeduction(ctx, tx, returnID, orderID, deductionItems)
	require.NoError(t, err)

	// Verify the negative earning was inserted
	rows, err := tx.Query(ctx, "SELECT amount_cents, type, available_at FROM seller_ledger_entries WHERE seller_id = $1 ORDER BY created_at ASC", sellerID)
	require.NoError(t, err)
	defer rows.Close()

	var amountCents []int64
	var types []string
	var availableAts []time.Time
	for rows.Next() {
		var a int64
		var tStr string
		var av *time.Time
		err := rows.Scan(&a, &tStr, &av)
		require.NoError(t, err)
		amountCents = append(amountCents, a)
		types = append(types, tStr)
		if av != nil {
			availableAts = append(availableAts, *av)
		} else {
			availableAts = append(availableAts, time.Time{})
		}
	}

	assert.Len(t, amountCents, 2)
	
	deductionIdx := -1
	for i, a := range amountCents {
		if a < 0 {
			deductionIdx = i
			break
		}
	}
	
	assert.NotEqual(t, -1, deductionIdx)
	assert.Equal(t, "adjustment", types[deductionIdx])
	assert.Equal(t, int64(-50000), amountCents[deductionIdx]) // 500 RUB deducted
	// Assert AvailableAt is exactly the same
	assert.Equal(t, availableAt.Unix(), availableAts[deductionIdx].Unix())
}
