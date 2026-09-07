package returns_test

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/eirfefrggrvgrgrtbg/shop-zamk/backend/internal/returns"
)

func TestM531B_AdminClaimReadModelAndEvidence(t *testing.T) {
	fix := setupM51Fixture(t)
	ctx := context.Background()
	adminID := uuid.New()

	// 1. Create delivered order with rich customer/product data
	orderID := uuid.New()
	fID := uuid.New()
	shipmentID := uuid.New()
	oiID := uuid.New()
	orderNumber := fmt.Sprintf("ORD-M531B-%s", uuid.New().String()[:6])
	customerPhone := "+79991234567"
	customerName := "Test Customer"
	customerEmail := "test_customer@zamk.local"

	_, err := fix.client.Pool.Exec(ctx, `
		INSERT INTO orders (id, user_id, order_number, status, total_price_cents, currency, delivery_address, delivery_method_name, delivery_price_cents, customer_name, customer_email, customer_phone)
		VALUES ($1, $2, $3, 'delivered', 3500, 'RUB', 'Test St 10', 'Courier', 0, $4, $5, $6)
	`, orderID, fix.userID, orderNumber, customerName, customerEmail, customerPhone)
	require.NoError(t, err)

	_, err = fix.client.Pool.Exec(ctx, `
		INSERT INTO order_fulfillments (id, order_id, seller_id, status)
		VALUES ($1, $2, $3, 'delivered')
	`, fID, orderID, fix.sellerAID)
	require.NoError(t, err)

	delivTime := time.Now().Add(-24 * time.Hour)
	_, err = fix.client.Pool.Exec(ctx, `
		INSERT INTO shipments (id, order_id, fulfillment_id, status, shipped_at, delivered_at)
		VALUES ($1, $2, $3, 'delivered', $4, $5)
	`, shipmentID, orderID, fID, delivTime.Add(-10*time.Hour), delivTime)
	require.NoError(t, err)

	variantSize := "L"
	variantColor := "Charcoal"
	sku := "SKU-COAT-L-CHR"
	imgURL := "https://minio.zamk.local/media/coat.jpg"

	_, err = fix.client.Pool.Exec(ctx, `
		INSERT INTO order_items (id, order_id, order_fulfillment_id, seller_id, product_id, product_variant_id, title, product_slug, price_cents, subtotal_price_cents, quantity, variant_size, variant_color, sku, image_url)
		VALUES ($1, $2, $3, $4, $5, $6, 'Wool Winter Coat', 'wool-winter-coat', 3500, 3500, 1, $7, $8, $9, $10)
	`, oiID, orderID, fID, fix.sellerAID, fix.prodAID, fix.varAID, variantSize, variantColor, sku, imgURL)
	require.NoError(t, err)

	// Create 2 staged evidence photos
	evIDs := fix.createStagedEvidence(t, fix.userID, 2)
	comment := "Right sleeve has torn seam on arrival"

	resp, err := fix.svc.CreateReturn(ctx, fix.userID, orderID, returns.CreateReturnRequest{
		Reason:  "damaged",
		Comment: &comment,
		Items:   []returns.CreateReturnItemRequest{{OrderItemID: oiID, Quantity: 1, EvidenceIDs: evIDs}},
	})
	require.NoError(t, err)
	require.Len(t, resp, 1)
	returnID := resp[0].Return.ID

	// 2. Fetch Admin Return Detail Read Model
	adminDetail, err := fix.svc.GetAdminReturn(ctx, returnID)
	require.NoError(t, err)
	require.NotNil(t, adminDetail)

	// Assert Return fields
	assert.Equal(t, returnID, adminDetail.ID)
	assert.Equal(t, "requested", adminDetail.Status)
	assert.Equal(t, "damaged", adminDetail.Reason)
	require.NotNil(t, adminDetail.Comment)
	assert.Equal(t, comment, *adminDetail.Comment)

	// Assert Order & Customer context
	require.NotNil(t, adminDetail.OrderNumber)
	assert.Equal(t, orderNumber, *adminDetail.OrderNumber)
	require.NotNil(t, adminDetail.CustomerName)
	assert.Equal(t, customerName, *adminDetail.CustomerName)
	require.NotNil(t, adminDetail.CustomerEmail)
	assert.Equal(t, customerEmail, *adminDetail.CustomerEmail)
	require.NotNil(t, adminDetail.CustomerPhone)
	assert.Equal(t, customerPhone, *adminDetail.CustomerPhone)
	require.NotNil(t, adminDetail.DeliveredAt)

	// Assert Seller context
	require.NotNil(t, adminDetail.SellerID)
	assert.Equal(t, fix.sellerAID, *adminDetail.SellerID)
	require.NotNil(t, adminDetail.SellerName)
	assert.NotEmpty(t, *adminDetail.SellerName)

	// Assert Items & Evidence
	require.Len(t, adminDetail.Items, 1)
	it := adminDetail.Items[0]
	assert.Equal(t, "Wool Winter Coat", it.ProductTitle)
	require.NotNil(t, it.ProductImageURL)
	assert.Equal(t, imgURL, *it.ProductImageURL)
	require.NotNil(t, it.VariantSize)
	assert.Equal(t, variantSize, *it.VariantSize)
	require.NotNil(t, it.VariantColor)
	assert.Equal(t, variantColor, *it.VariantColor)
	require.NotNil(t, it.SKU)
	assert.Equal(t, sku, *it.SKU)
	assert.Equal(t, 1, it.Quantity)
	assert.Equal(t, int64(3500), it.PriceCents)
	assert.Equal(t, int64(3500), it.SubtotalPriceCents)

	// Assert Evidence in sort order with public URL
	assert.Equal(t, 2, adminDetail.EvidenceCount)
	require.Len(t, it.Evidence, 2)
	assert.Equal(t, evIDs[0], it.Evidence[0].ID)
	assert.Equal(t, evIDs[1], it.Evidence[1].ID)
	assert.NotEmpty(t, it.Evidence[0].URL)
	assert.NotEmpty(t, it.Evidence[1].URL)
	assert.Contains(t, it.Evidence[0].URL, "photo.jpg")

	// 3. Assert ListAdminReturns includes return with items and evidence count
	list, total, err := fix.svc.ListAdminReturns(ctx, 10, 0, false)
	require.NoError(t, err)
	assert.GreaterOrEqual(t, total, 1)
	require.NotEmpty(t, list)

	var foundInList *returns.AdminReturnResponse
	for i := range list {
		if list[i].ID == returnID {
			foundInList = &list[i]
			break
		}
	}
	require.NotNil(t, foundInList, "Created return must be in admin list")
	assert.Equal(t, orderNumber, *foundInList.OrderNumber)
	assert.Equal(t, 2, foundInList.EvidenceCount)
	require.NotEmpty(t, foundInList.Items)
	assert.Equal(t, "Wool Winter Coat", foundInList.Items[0].ProductTitle)

	// 4. Test Historical Return with ZERO evidence loads normally
	histOrd := fix.createDeliveredOrder(t, time.Now().Add(-2*time.Hour), 1)
	histComment := "Changed mind historical"
	histResp, err := fix.svc.CreateReturn(ctx, fix.userID, histOrd.orderID, returns.CreateReturnRequest{
		Reason:  "changed_mind",
		Comment: &histComment,
		Items:   []returns.CreateReturnItemRequest{{OrderItemID: histOrd.orderItemID, Quantity: 1}},
	})
	require.NoError(t, err)
	require.Len(t, histResp, 1)

	histDetail, err := fix.svc.GetAdminReturn(ctx, histResp[0].Return.ID)
	require.NoError(t, err)
	require.NotNil(t, histDetail)
	assert.Equal(t, 0, histDetail.EvidenceCount)
	require.Len(t, histDetail.Items, 1)
	assert.Empty(t, histDetail.Items[0].Evidence, "Evidence array should be empty for historical return without evidence")

	// 5. Test Approve Moderation Flow
	err = fix.svc.UpdateReturnStatus(ctx, adminID, returnID, returns.UpdateReturnStatusRequest{
		Status: "approved",
	})
	require.NoError(t, err)

	approvedDetail, err := fix.svc.GetAdminReturn(ctx, returnID)
	require.NoError(t, err)
	assert.Equal(t, "approved", approvedDetail.Status)
	assert.NotNil(t, approvedDetail.ApprovedAt)

	// 6. Test Reject Moderation Flow with required comment on fresh claim
	rejOrd := fix.createDeliveredOrder(t, time.Now().Add(-1*time.Hour), 1)
	rejComment := "Customer claim to reject"
	rejResp, err := fix.svc.CreateReturn(ctx, fix.userID, rejOrd.orderID, returns.CreateReturnRequest{
		Reason:  "size_fit",
		Comment: &rejComment,
		Items:   []returns.CreateReturnItemRequest{{OrderItemID: rejOrd.orderItemID, Quantity: 1}},
	})
	require.NoError(t, err)
	rejReturnID := rejResp[0].Return.ID

	// Reject without comment -> ErrRejectReasonRequired
	err = fix.svc.UpdateReturnStatus(ctx, adminID, rejReturnID, returns.UpdateReturnStatusRequest{
		Status:       "rejected",
		AdminComment: nil,
	})
	assert.ErrorIs(t, err, returns.ErrRejectReasonRequired)

	// Reject with whitespace-only comment -> ErrRejectReasonRequired
	whitespace := "   \t\n  "
	err = fix.svc.UpdateReturnStatus(ctx, adminID, rejReturnID, returns.UpdateReturnStatusRequest{
		Status:       "rejected",
		AdminComment: &whitespace,
	})
	assert.ErrorIs(t, err, returns.ErrRejectReasonRequired)

	// Reject with valid comment -> SUCCESS
	rejectReason := "Tags removed and item shows signs of wear"
	err = fix.svc.UpdateReturnStatus(ctx, adminID, rejReturnID, returns.UpdateReturnStatusRequest{
		Status:       "rejected",
		AdminComment: &rejectReason,
	})
	require.NoError(t, err)

	rejectedDetail, err := fix.svc.GetAdminReturn(ctx, rejReturnID)
	require.NoError(t, err)
	assert.Equal(t, "rejected", rejectedDetail.Status)
	assert.NotNil(t, rejectedDetail.RejectedAt)
	require.NotNil(t, rejectedDetail.AdminComment)
	assert.Equal(t, rejectReason, *rejectedDetail.AdminComment)

	// Verify rejected status released quantity so customer can create new return
	newComment := "Re-applying with fresh claim"
	_, err = fix.svc.CreateReturn(ctx, fix.userID, rejOrd.orderID, returns.CreateReturnRequest{
		Reason:  "size_fit",
		Comment: &newComment,
		Items:   []returns.CreateReturnItemRequest{{OrderItemID: rejOrd.orderItemID, Quantity: 1}},
	})
	require.NoError(t, err, "Rejected return must release returnable quantity")

	// 7. Verify Zero Side Effects: no refund, no stock movements created by approve/reject
	var refundsCount, movementsCount int
	_ = fix.client.Pool.QueryRow(ctx, "SELECT COUNT(*) FROM refunds WHERE order_id IN ($1, $2)", orderID, rejOrd.orderID).Scan(&refundsCount)
	_ = fix.client.Pool.QueryRow(ctx, "SELECT COUNT(*) FROM stock_movements WHERE order_id IN ($1, $2)", orderID, rejOrd.orderID).Scan(&movementsCount)
	assert.Equal(t, 0, refundsCount, "Moderation decisions must not create refunds")
	assert.Equal(t, 0, movementsCount, "Moderation decisions must not create stock movements")
}
