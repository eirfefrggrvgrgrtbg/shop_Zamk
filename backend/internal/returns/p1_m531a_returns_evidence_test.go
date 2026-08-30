package returns_test

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	goredis "github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/eirfefrggrvgrgrtbg/shop-zamk/backend/internal/app"
	"github.com/eirfefrggrvgrgrtbg/shop-zamk/backend/internal/auth"
	"github.com/eirfefrggrvgrgrtbg/shop-zamk/backend/internal/config"
	"github.com/eirfefrggrvgrgrtbg/shop-zamk/backend/internal/inventory"
	"github.com/eirfefrggrvgrgrtbg/shop-zamk/backend/internal/orders"
	"github.com/eirfefrggrvgrgrtbg/shop-zamk/backend/internal/payments"
	"github.com/eirfefrggrvgrgrtbg/shop-zamk/backend/internal/payouts"
	"github.com/eirfefrggrvgrgrtbg/shop-zamk/backend/internal/platform/postgres"
	"github.com/eirfefrggrvgrgrtbg/shop-zamk/backend/internal/platform/redis"
	"github.com/eirfefrggrvgrgrtbg/shop-zamk/backend/internal/returns"
	"github.com/eirfefrggrvgrgrtbg/shop-zamk/backend/internal/storage"
)

// Dummy storage provider for testing uploads
type dummyStorageProvider struct {
	mu          sync.Mutex
	deletedKeys []string
}

func (d *dummyStorageProvider) UploadImage(ctx context.Context, reader io.Reader, objectSize int64, objectKey string, contentType string) (*storage.StoredObject, error) {
	return &storage.StoredObject{
		ObjectURL: "http://localhost:9000/media/" + objectKey,
		ObjectKey: objectKey,
		Size:      objectSize,
	}, nil
}

func (d *dummyStorageProvider) DownloadObject(ctx context.Context, objectKey string) ([]byte, error) {
	return []byte("dummy image content"), nil
}

func (d *dummyStorageProvider) DeleteObject(ctx context.Context, objectKey string) error {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.deletedKeys = append(d.deletedKeys, objectKey)
	return nil
}

func (d *dummyStorageProvider) BuildPublicURL(objectKey string) string {
	return "http://localhost:9000/media/" + objectKey
}

func (d *dummyStorageProvider) GetDeletedKeys() []string {
	d.mu.Lock()
	defer d.mu.Unlock()
	copied := make([]string, len(d.deletedKeys))
	copy(copied, d.deletedKeys)
	return copied
}

type failingStorageProvider struct {
	dummyStorageProvider
}

func (f *failingStorageProvider) DeleteObject(ctx context.Context, objectKey string) error {
	return fmt.Errorf("minio connection refused")
}

func TestM531A_EvidenceRequirementMatrix(t *testing.T) {
	fix := setupM51Fixture(t)
	ctx := context.Background()

	tOrd := fix.createDeliveredOrder(t, time.Now().Add(-1*time.Hour), 10)
	validComment := "Description of problem"

	// 1. DEFECTIVE (Required 2-6 photos + mandatory comment)
	// Empty comment -> ErrCommentRequired
	ev2 := fix.createStagedEvidence(t, fix.userID, 2)
	_, err := fix.svc.CreateReturn(ctx, fix.userID, tOrd.orderID, returns.CreateReturnRequest{
		Reason:  "defective",
		Comment: nil,
		Items:   []returns.CreateReturnItemRequest{{OrderItemID: tOrd.orderItemID, Quantity: 1, EvidenceIDs: ev2}},
	})
	assert.ErrorIs(t, err, returns.ErrCommentRequired)

	emptyComment := "   \t\n  "
	_, err = fix.svc.CreateReturn(ctx, fix.userID, tOrd.orderID, returns.CreateReturnRequest{
		Reason:  "defective",
		Comment: &emptyComment,
		Items:   []returns.CreateReturnItemRequest{{OrderItemID: tOrd.orderItemID, Quantity: 1, EvidenceIDs: ev2}},
	})
	assert.ErrorIs(t, err, returns.ErrCommentRequired)

	// 0 photos + valid comment -> ErrEvidenceRequired
	_, err = fix.svc.CreateReturn(ctx, fix.userID, tOrd.orderID, returns.CreateReturnRequest{
		Reason:  "defective",
		Comment: &validComment,
		Items:   []returns.CreateReturnItemRequest{{OrderItemID: tOrd.orderItemID, Quantity: 1}},
	})
	assert.ErrorIs(t, err, returns.ErrEvidenceRequired)

	// 1 photo + valid comment -> ErrEvidenceRequired
	ev1 := fix.createStagedEvidence(t, fix.userID, 1)
	_, err = fix.svc.CreateReturn(ctx, fix.userID, tOrd.orderID, returns.CreateReturnRequest{
		Reason:  "defective",
		Comment: &validComment,
		Items:   []returns.CreateReturnItemRequest{{OrderItemID: tOrd.orderItemID, Quantity: 1, EvidenceIDs: ev1}},
	})
	assert.ErrorIs(t, err, returns.ErrEvidenceRequired)

	// 2 photos + valid comment -> SUCCESS
	resp2, err := fix.svc.CreateReturn(ctx, fix.userID, tOrd.orderID, returns.CreateReturnRequest{
		Reason:  "defective",
		Comment: &validComment,
		Items:   []returns.CreateReturnItemRequest{{OrderItemID: tOrd.orderItemID, Quantity: 1, EvidenceIDs: ev2}},
	})
	require.NoError(t, err)
	assert.Len(t, resp2, 1)

	// 6 photos + valid comment -> SUCCESS
	ev6 := fix.createStagedEvidence(t, fix.userID, 6)
	resp6, err := fix.svc.CreateReturn(ctx, fix.userID, tOrd.orderID, returns.CreateReturnRequest{
		Reason:  "defective",
		Comment: &validComment,
		Items:   []returns.CreateReturnItemRequest{{OrderItemID: tOrd.orderItemID, Quantity: 1, EvidenceIDs: ev6}},
	})
	require.NoError(t, err)
	assert.Len(t, resp6, 1)

	// 7 photos + valid comment -> ErrEvidenceTooMany
	ev7 := fix.createStagedEvidence(t, fix.userID, 7)
	_, err = fix.svc.CreateReturn(ctx, fix.userID, tOrd.orderID, returns.CreateReturnRequest{
		Reason:  "defective",
		Comment: &validComment,
		Items:   []returns.CreateReturnItemRequest{{OrderItemID: tOrd.orderItemID, Quantity: 1, EvidenceIDs: ev7}},
	})
	assert.ErrorIs(t, err, returns.ErrEvidenceTooMany)

	// 2. OTHER REQUIRED REASONS: damaged, wrong_item, not_as_described, incomplete (2 valid photos + comment -> success)
	for _, reqReason := range []string{"damaged", "wrong_item", "not_as_described", "incomplete"} {
		// Empty comment -> ErrCommentRequired
		ev := fix.createStagedEvidence(t, fix.userID, 2)
		_, err := fix.svc.CreateReturn(ctx, fix.userID, tOrd.orderID, returns.CreateReturnRequest{
			Reason:  reqReason,
			Comment: nil,
			Items:   []returns.CreateReturnItemRequest{{OrderItemID: tOrd.orderItemID, Quantity: 1, EvidenceIDs: ev}},
		})
		assert.ErrorIs(t, err, returns.ErrCommentRequired, "Reason %s must require comment", reqReason)

		// Valid comment -> Success
		resp, err := fix.svc.CreateReturn(ctx, fix.userID, tOrd.orderID, returns.CreateReturnRequest{
			Reason:  reqReason,
			Comment: &validComment,
			Items:   []returns.CreateReturnItemRequest{{OrderItemID: tOrd.orderItemID, Quantity: 1, EvidenceIDs: ev}},
		})
		require.NoError(t, err, "Reason %s with 2 photos and comment must succeed", reqReason)
		assert.Len(t, resp, 1)
	}

	// 3. OPTIONAL REASONS: size_fit, changed_mind, other (0 photos + comment -> success, empty comment -> reject)
	for _, optReason := range []string{"size_fit", "changed_mind", "other"} {
		// Empty comment -> ErrCommentRequired
		_, err := fix.svc.CreateReturn(ctx, fix.userID, tOrd.orderID, returns.CreateReturnRequest{
			Reason:  optReason,
			Comment: nil,
			Items:   []returns.CreateReturnItemRequest{{OrderItemID: tOrd.orderItemID, Quantity: 1}},
		})
		assert.ErrorIs(t, err, returns.ErrCommentRequired, "Reason %s must require comment", optReason)

		// Whitespace comment -> ErrCommentRequired
		_, err = fix.svc.CreateReturn(ctx, fix.userID, tOrd.orderID, returns.CreateReturnRequest{
			Reason:  optReason,
			Comment: &emptyComment,
			Items:   []returns.CreateReturnItemRequest{{OrderItemID: tOrd.orderItemID, Quantity: 1}},
		})
		assert.ErrorIs(t, err, returns.ErrCommentRequired, "Reason %s must reject whitespace comment", optReason)

		// Valid comment + 0 photos -> SUCCESS
		resp, err := fix.svc.CreateReturn(ctx, fix.userID, tOrd.orderID, returns.CreateReturnRequest{
			Reason:  optReason,
			Comment: &validComment,
			Items:   []returns.CreateReturnItemRequest{{OrderItemID: tOrd.orderItemID, Quantity: 1}},
		})
		require.NoError(t, err, "Reason %s with comment must succeed", optReason)
		assert.Len(t, resp, 1)
	}

	// size_fit with 7 photos + comment -> ErrEvidenceTooMany
	evSize7 := fix.createStagedEvidence(t, fix.userID, 7)
	_, err = fix.svc.CreateReturn(ctx, fix.userID, tOrd.orderID, returns.CreateReturnRequest{
		Reason:  "size_fit",
		Comment: &validComment,
		Items:   []returns.CreateReturnItemRequest{{OrderItemID: tOrd.orderItemID, Quantity: 1, EvidenceIDs: evSize7}},
	})
	assert.ErrorIs(t, err, returns.ErrEvidenceTooMany)
}

func TestM531A_EvidenceSecurityAndOwnership(t *testing.T) {
	fix := setupM51Fixture(t)
	ctx := context.Background()
	testComment := "Valid comment text"

	tOrd := fix.createDeliveredOrder(t, time.Now().Add(-1*time.Hour), 5)
	otherCustomerID := uuid.New()
	_, err := fix.client.Pool.Exec(ctx, "INSERT INTO users (id, name, phone, email, password_hash) VALUES ($1, 'Other Cust', '+79998887766', $2, 'hash')",
		otherCustomerID, "other_"+uuid.New().String()+"@test.com")
	require.NoError(t, err)

	// A. Stealing another customer's staged evidence -> ErrEvidenceNotFound
	foreignEvidence := fix.createStagedEvidence(t, otherCustomerID, 2)
	_, err = fix.svc.CreateReturn(ctx, fix.userID, tOrd.orderID, returns.CreateReturnRequest{
		Reason:  "defective",
		Comment: &testComment,
		Items:   []returns.CreateReturnItemRequest{{OrderItemID: tOrd.orderItemID, Quantity: 1, EvidenceIDs: foreignEvidence}},
	})
	assert.ErrorIs(t, err, returns.ErrEvidenceNotFound, "Must reject using another customer's staged evidence")

	// B. Nonexistent evidence ID -> ErrEvidenceNotFound
	nonexistentIDs := []uuid.UUID{uuid.New(), uuid.New()}
	_, err = fix.svc.CreateReturn(ctx, fix.userID, tOrd.orderID, returns.CreateReturnRequest{
		Reason:  "defective",
		Comment: &testComment,
		Items:   []returns.CreateReturnItemRequest{{OrderItemID: tOrd.orderItemID, Quantity: 1, EvidenceIDs: nonexistentIDs}},
	})
	assert.ErrorIs(t, err, returns.ErrEvidenceNotFound)

	// C. Duplicate evidence ID in same item -> ErrEvidenceDuplicate
	validEv := fix.createStagedEvidence(t, fix.userID, 1)
	dupIDs := []uuid.UUID{validEv[0], validEv[0]}
	_, err = fix.svc.CreateReturn(ctx, fix.userID, tOrd.orderID, returns.CreateReturnRequest{
		Reason:  "defective",
		Comment: &testComment,
		Items:   []returns.CreateReturnItemRequest{{OrderItemID: tOrd.orderItemID, Quantity: 1, EvidenceIDs: dupIDs}},
	})
	assert.ErrorIs(t, err, returns.ErrEvidenceDuplicate)

	// D. Unsupported media format in DB -> ErrEvidenceInvalidFormat
	badEvID1 := uuid.New()
	badEvID2 := uuid.New()
	_, err = fix.client.Pool.Exec(ctx, `
		INSERT INTO return_item_evidences (id, customer_id, storage_key, content_type, sort_order)
		VALUES ($1, $2, 'key1.pdf', 'application/pdf', 0), ($3, $2, 'key2.pdf', 'application/pdf', 1)
	`, badEvID1, fix.userID, badEvID2)
	require.NoError(t, err)

	_, err = fix.svc.CreateReturn(ctx, fix.userID, tOrd.orderID, returns.CreateReturnRequest{
		Reason:  "defective",
		Comment: &testComment,
		Items:   []returns.CreateReturnItemRequest{{OrderItemID: tOrd.orderItemID, Quantity: 1, EvidenceIDs: []uuid.UUID{badEvID1, badEvID2}}},
	})
	assert.ErrorIs(t, err, returns.ErrEvidenceInvalidFormat)

	// E. Already consumed evidence -> ErrEvidenceAlreadyBound
	usedEv := fix.createStagedEvidence(t, fix.userID, 2)
	respFirst, err := fix.svc.CreateReturn(ctx, fix.userID, tOrd.orderID, returns.CreateReturnRequest{
		Reason:  "defective",
		Comment: &testComment,
		Items:   []returns.CreateReturnItemRequest{{OrderItemID: tOrd.orderItemID, Quantity: 1, EvidenceIDs: usedEv}},
	})
	require.NoError(t, err)
	require.Len(t, respFirst, 1)

	// Attempt using usedEv again in second return
	_, err = fix.svc.CreateReturn(ctx, fix.userID, tOrd.orderID, returns.CreateReturnRequest{
		Reason:  "defective",
		Comment: &testComment,
		Items:   []returns.CreateReturnItemRequest{{OrderItemID: tOrd.orderItemID, Quantity: 1, EvidenceIDs: usedEv}},
	})
	assert.ErrorIs(t, err, returns.ErrEvidenceAlreadyBound, "Must reject already bound evidence")
}

func TestM531A_MultiItemEvidenceLinkage(t *testing.T) {
	fix := setupM51Fixture(t)
	ctx := context.Background()
	testComment := "Multi-item claim comment"

	orderID := uuid.New()
	fID := uuid.New()
	shipmentID := uuid.New()
	oiIDA := uuid.New()
	oiIDB := uuid.New()

	_, err := fix.client.Pool.Exec(ctx, `
		INSERT INTO orders (id, user_id, order_number, status, total_price_cents, currency, delivery_address, delivery_method_name, delivery_price_cents, customer_name, customer_email, customer_phone)
		VALUES ($1, $2, $3, 'delivered', 2000, 'RUB', 'Addr', 'Courier', 0, 'Name', 'email@test.com', '+123')
	`, orderID, fix.userID, fmt.Sprintf("ORD-LINK-%s", uuid.New().String()[:8]))
	require.NoError(t, err)

	_, err = fix.client.Pool.Exec(ctx, `
		INSERT INTO order_fulfillments (id, order_id, seller_id, status)
		VALUES ($1, $2, $3, 'delivered')
	`, fID, orderID, fix.sellerAID)
	require.NoError(t, err)

	_, err = fix.client.Pool.Exec(ctx, `
		INSERT INTO shipments (id, order_id, fulfillment_id, status, shipped_at, delivered_at)
		VALUES ($1, $2, $3, 'delivered', now() - interval '2 days', now() - interval '1 day')
	`, shipmentID, orderID, fID)
	require.NoError(t, err)

	_, err = fix.client.Pool.Exec(ctx, `
		INSERT INTO order_items (id, order_id, order_fulfillment_id, seller_id, product_id, product_variant_id, title, product_slug, price_cents, subtotal_price_cents, quantity)
		VALUES ($1, $2, $3, $4, $5, $6, 'Item A', 'slug-a', 1000, 1000, 1),
		       ($7, $2, $3, $4, $5, $6, 'Item B', 'slug-b', 1000, 1000, 1)
	`, oiIDA, orderID, fID, fix.sellerAID, fix.prodAID, fix.varAID,
		oiIDB)
	require.NoError(t, err)

	evA := fix.createStagedEvidence(t, fix.userID, 2)
	evB := fix.createStagedEvidence(t, fix.userID, 2)

	resp, err := fix.svc.CreateReturn(ctx, fix.userID, orderID, returns.CreateReturnRequest{
		Reason:  "defective",
		Comment: &testComment,
		Items: []returns.CreateReturnItemRequest{
			{OrderItemID: oiIDA, Quantity: 1, EvidenceIDs: evA},
			{OrderItemID: oiIDB, Quantity: 1, EvidenceIDs: evB},
		},
	})
	require.NoError(t, err)
	require.Len(t, resp, 1)
	require.Len(t, resp[0].Items, 2)

	var itemIDA, itemIDB uuid.UUID
	for _, item := range resp[0].Items {
		if item.OrderItemID == oiIDA {
			itemIDA = item.ID
		} else if item.OrderItemID == oiIDB {
			itemIDB = item.ID
		}
	}
	require.NotEqual(t, uuid.Nil, itemIDA)
	require.NotEqual(t, uuid.Nil, itemIDB)

	// Verify exact linkage in DB
	for _, id := range evA {
		var boundItemID *uuid.UUID
		err := fix.client.Pool.QueryRow(ctx, "SELECT return_item_id FROM return_item_evidences WHERE id = $1", id).Scan(&boundItemID)
		require.NoError(t, err)
		require.NotNil(t, boundItemID)
		assert.Equal(t, itemIDA, *boundItemID, "Evidence A must be linked ONLY to ReturnItem A")
	}

	for _, id := range evB {
		var boundItemID *uuid.UUID
		err := fix.client.Pool.QueryRow(ctx, "SELECT return_item_id FROM return_item_evidences WHERE id = $1", id).Scan(&boundItemID)
		require.NoError(t, err)
		require.NotNil(t, boundItemID)
		assert.Equal(t, itemIDB, *boundItemID, "Evidence B must be linked ONLY to ReturnItem B")
	}
}

func TestM531A_MultiFulfillmentAtomicityAndRollback(t *testing.T) {
	fix := setupM51Fixture(t)
	ctx := context.Background()
	testComment := "Rollback test comment"

	orderID := uuid.New()
	fIDA := uuid.New()
	fIDB := uuid.New()
	oiIDA := uuid.New()
	oiIDB := uuid.New()

	_, err := fix.client.Pool.Exec(ctx, `
		INSERT INTO orders (id, user_id, order_number, status, total_price_cents, currency, delivery_address, delivery_method_name, delivery_price_cents, customer_name, customer_email, customer_phone)
		VALUES ($1, $2, $3, 'delivered', 2000, 'RUB', 'Addr', 'Courier', 0, 'Name', 'email@test.com', '+123')
	`, orderID, fix.userID, fmt.Sprintf("ORD-ATOM-%s", uuid.New().String()[:8]))
	require.NoError(t, err)

	_, err = fix.client.Pool.Exec(ctx, `
		INSERT INTO order_fulfillments (id, order_id, seller_id, status)
		VALUES ($1, $2, $3, 'delivered'), ($4, $2, $5, 'delivered')
	`, fIDA, orderID, fix.sellerAID, fIDB, fix.sellerBID)
	require.NoError(t, err)

	_, err = fix.client.Pool.Exec(ctx, `
		INSERT INTO shipments (id, order_id, fulfillment_id, status, shipped_at, delivered_at)
		VALUES ($1, $2, $3, 'delivered', now() - interval '2 days', now() - interval '1 day'),
		       ($4, $2, $5, 'delivered', now() - interval '2 days', now() - interval '1 day')
	`, uuid.New(), orderID, fIDA, uuid.New(), fIDB)
	require.NoError(t, err)

	_, err = fix.client.Pool.Exec(ctx, `
		INSERT INTO order_items (id, order_id, order_fulfillment_id, seller_id, product_id, product_variant_id, title, product_slug, price_cents, subtotal_price_cents, quantity)
		VALUES ($1, $2, $3, $4, $5, $6, 'Item A', 'slug-a', 1000, 1000, 1),
		       ($7, $2, $8, $9, $10, $11, 'Item B', 'slug-b', 1000, 1000, 1)
	`, oiIDA, orderID, fIDA, fix.sellerAID, fix.prodAID, fix.varAID,
		oiIDB, fIDB, fix.sellerBID, fix.prodBID, fix.varBID)
	require.NoError(t, err)

	// Item A has valid 2 photos for defective
	evA := fix.createStagedEvidence(t, fix.userID, 2)
	// Item B has 0 photos for defective (fails validation!)
	_, err = fix.svc.CreateReturn(ctx, fix.userID, orderID, returns.CreateReturnRequest{
		Reason:  "defective",
		Comment: &testComment,
		Items: []returns.CreateReturnItemRequest{
			{OrderItemID: oiIDA, Quantity: 1, EvidenceIDs: evA},
			{OrderItemID: oiIDB, Quantity: 1, EvidenceIDs: nil},
		},
	})
	assert.ErrorIs(t, err, returns.ErrEvidenceRequired)

	// Assert complete atomic rollback
	var returnsCount, returnItemsCount int
	err = fix.client.Pool.QueryRow(ctx, "SELECT COUNT(*) FROM returns WHERE order_id = $1", orderID).Scan(&returnsCount)
	require.NoError(t, err)
	assert.Equal(t, 0, returnsCount, "0 returns must be created on validation failure")

	err = fix.client.Pool.QueryRow(ctx, "SELECT COUNT(*) FROM return_items WHERE order_item_id = ANY($1)", []uuid.UUID{oiIDA, oiIDB}).Scan(&returnItemsCount)
	require.NoError(t, err)
	assert.Equal(t, 0, returnItemsCount, "0 return_items must be created on validation failure")

	// Assert staged evidence for A remained unconsumed
	for _, id := range evA {
		var boundItemID *uuid.UUID
		err = fix.client.Pool.QueryRow(ctx, "SELECT return_item_id FROM return_item_evidences WHERE id = $1", id).Scan(&boundItemID)
		require.NoError(t, err)
		assert.Nil(t, boundItemID, "Staged evidence must remain unbound on rollback")
	}
}

// 5. EVIDENCE FAILURE SIDE EFFECT PROOF (Zero Mutation Snapshot)
func TestM531A_EvidenceFailureSideEffectProof(t *testing.T) {
	fix := setupM51Fixture(t)
	ctx := context.Background()
	testComment := "Side-effect proof comment"

	tOrd := fix.createDeliveredOrder(t, time.Now().Add(-1*time.Hour), 5)

	// Capture initial DB state
	var initReturnsCount, initReturnItemsCount, initRefundsCount, initMovementsCount int
	_ = fix.client.Pool.QueryRow(ctx, "SELECT COUNT(*) FROM returns").Scan(&initReturnsCount)
	_ = fix.client.Pool.QueryRow(ctx, "SELECT COUNT(*) FROM return_items").Scan(&initReturnItemsCount)
	_ = fix.client.Pool.QueryRow(ctx, "SELECT COUNT(*) FROM refunds").Scan(&initRefundsCount)
	_ = fix.client.Pool.QueryRow(ctx, "SELECT COUNT(*) FROM stock_movements").Scan(&initMovementsCount)

	// Create 1 valid staged evidence (insufficient for defective which requires >= 2)
	evIDs := fix.createStagedEvidence(t, fix.userID, 1)

	// Attempt CreateReturn -> Fails validation
	_, err := fix.svc.CreateReturn(ctx, fix.userID, tOrd.orderID, returns.CreateReturnRequest{
		Reason:  "defective",
		Comment: &testComment,
		Items:   []returns.CreateReturnItemRequest{{OrderItemID: tOrd.orderItemID, Quantity: 1, EvidenceIDs: evIDs}},
	})
	assert.ErrorIs(t, err, returns.ErrEvidenceRequired)

	// Assert zero mutations across all operational and financial tables
	var postReturnsCount, postReturnItemsCount, postRefundsCount, postMovementsCount int
	_ = fix.client.Pool.QueryRow(ctx, "SELECT COUNT(*) FROM returns").Scan(&postReturnsCount)
	_ = fix.client.Pool.QueryRow(ctx, "SELECT COUNT(*) FROM return_items").Scan(&postReturnItemsCount)
	_ = fix.client.Pool.QueryRow(ctx, "SELECT COUNT(*) FROM refunds").Scan(&postRefundsCount)
	_ = fix.client.Pool.QueryRow(ctx, "SELECT COUNT(*) FROM stock_movements").Scan(&postMovementsCount)

	assert.Equal(t, initReturnsCount, postReturnsCount, "Zero returns created")
	assert.Equal(t, initReturnItemsCount, postReturnItemsCount, "Zero return_items created")
	assert.Equal(t, initRefundsCount, postRefundsCount, "Zero refunds created")
	assert.Equal(t, initMovementsCount, postMovementsCount, "Zero stock_movements created")

	// Staged evidence remains completely unbound
	var boundItemID *uuid.UUID
	err = fix.client.Pool.QueryRow(ctx, "SELECT return_item_id FROM return_item_evidences WHERE id = $1", evIDs[0]).Scan(&boundItemID)
	require.NoError(t, err)
	assert.Nil(t, boundItemID, "Staged evidence must remain unbound after failed create return")
}

// 6. DELETE STAGED EVIDENCE PROOF & STORAGE CALL VERIFICATION
func TestM531A_DeleteStagedEvidence_SecurityAndAtomicity(t *testing.T) {
	fix := setupM51Fixture(t)
	ctx := context.Background()
	testComment := "Delete staged evidence test comment"

	storageMock := &dummyStorageProvider{}
	ordersRepo := orders.NewRepository(fix.client.Pool)
	returnsRepo := returns.NewRepository(fix.client.Pool)
	invSvc := inventory.NewService(nil, nil, fix.client)
	payRepo := payments.NewRepository(fix.client.Pool)
	cfg := &config.Config{App: config.AppConfig{PaymentStuckPendingMinutes: 30}}
	paySvc := payments.NewService(payRepo, ordersRepo, nil, nil, fix.client, nil, cfg)
	payoutRepo := payouts.NewRepository(fix.client.Pool)
	payoutSvc := payouts.NewService(payoutRepo, fix.client, returnsRepo, ordersRepo, cfg, fix.notifSvc)
	testSvc := returns.NewService(returnsRepo, ordersRepo, invSvc, fix.client, payoutSvc, paySvc, 14, fix.notifSvc, storageMock, returns.NewFakeLogisticsProvider())

	otherCustID := uuid.New()
	_, err := fix.client.Pool.Exec(ctx, "INSERT INTO users (id, name, phone, email, password_hash) VALUES ($1, 'Other Cust 2', '+79991112233', $2, 'hash')",
		otherCustID, "other2_"+uuid.New().String()+"@test.com")
	require.NoError(t, err)

	// A. Owner deletes own staged evidence -> SUCCESS
	ownerEv := fix.createStagedEvidence(t, fix.userID, 1)
	var expectedStorageKey string
	err = fix.client.Pool.QueryRow(ctx, "SELECT storage_key FROM return_item_evidences WHERE id = $1", ownerEv[0]).Scan(&expectedStorageKey)
	require.NoError(t, err)

	err = testSvc.DeleteStagedEvidence(ctx, fix.userID, ownerEv[0])
	require.NoError(t, err)

	// Assert DB row is completely deleted
	var count int
	err = fix.client.Pool.QueryRow(ctx, "SELECT COUNT(*) FROM return_item_evidences WHERE id = $1", ownerEv[0]).Scan(&count)
	require.NoError(t, err)
	assert.Equal(t, 0, count, "Evidence row must be deleted from DB")

	// Assert DeleteObject was called EXACTLY ONCE with the exact storage key
	deletedKeys := storageMock.GetDeletedKeys()
	require.Len(t, deletedKeys, 1, "DeleteObject must be called exactly once for successful delete")
	assert.Equal(t, expectedStorageKey, deletedKeys[0], "Deleted storage key must match evidence.storage_key")

	// B. Other customer attempts to delete staged evidence -> ErrEvidenceNotFound (ownership isolation)
	victimEv := fix.createStagedEvidence(t, fix.userID, 1)
	err = testSvc.DeleteStagedEvidence(ctx, otherCustID, victimEv[0])
	assert.ErrorIs(t, err, returns.ErrEvidenceNotFound, "Other customer cannot delete victim staged evidence")

	// Verify victim evidence still exists in DB
	err = fix.client.Pool.QueryRow(ctx, "SELECT COUNT(*) FROM return_item_evidences WHERE id = $1", victimEv[0]).Scan(&count)
	require.NoError(t, err)
	assert.Equal(t, 1, count, "Victim evidence must remain intact in DB")

	// Assert DeleteObject was NOT called for rejected attempt
	assert.Len(t, storageMock.GetDeletedKeys(), 1, "DeleteObject must not be called when deletion is rejected for other customer")

	// C. Nonexistent evidence ID -> ErrEvidenceNotFound
	err = testSvc.DeleteStagedEvidence(ctx, fix.userID, uuid.New())
	assert.ErrorIs(t, err, returns.ErrEvidenceNotFound)
	assert.Len(t, storageMock.GetDeletedKeys(), 1, "DeleteObject must not be called for nonexistent evidence ID")

	// D. Attempt to delete already-bound evidence -> ErrEvidenceAlreadyBound
	tOrd := fix.createDeliveredOrder(t, time.Now().Add(-1*time.Hour), 1)
	boundEv := fix.createStagedEvidence(t, fix.userID, 2)
	resp, err := testSvc.CreateReturn(ctx, fix.userID, tOrd.orderID, returns.CreateReturnRequest{
		Reason:  "defective",
		Comment: &testComment,
		Items:   []returns.CreateReturnItemRequest{{OrderItemID: tOrd.orderItemID, Quantity: 1, EvidenceIDs: boundEv}},
	})
	require.NoError(t, err)
	require.Len(t, resp, 1)

	err = testSvc.DeleteStagedEvidence(ctx, fix.userID, boundEv[0])
	assert.ErrorIs(t, err, returns.ErrEvidenceAlreadyBound, "Cannot delete evidence already bound to a return item")
	assert.Len(t, storageMock.GetDeletedKeys(), 1, "DeleteObject must not be called when evidence is already bound")

	// E. Storage Provider Failure Semantics: if DeleteObject fails, DB row remains intact
	failingMock := &failingStorageProvider{}
	failingSvc := returns.NewService(returnsRepo, ordersRepo, invSvc, fix.client, payoutSvc, paySvc, 14, fix.notifSvc, failingMock, returns.NewFakeLogisticsProvider())
	stagedEvFail := fix.createStagedEvidence(t, fix.userID, 1)

	err = failingSvc.DeleteStagedEvidence(ctx, fix.userID, stagedEvFail[0])
	assert.Error(t, err, "Must return error when storage delete fails")
	assert.Contains(t, err.Error(), "failed to delete storage object")

	// Assert DB row is NOT deleted so cleanup can be retried safely
	err = fix.client.Pool.QueryRow(ctx, "SELECT COUNT(*) FROM return_item_evidences WHERE id = $1", stagedEvFail[0]).Scan(&count)
	require.NoError(t, err)
	assert.Equal(t, 1, count, "Evidence row must remain in DB when storage delete fails")
}

// 7. CANONICAL REASON & CONDITION CONTRACT PROOF
func TestM531A_CanonicalReasonContractProof(t *testing.T) {
	fix := setupM51Fixture(t)
	ctx := context.Background()
	testComment := "Damaged item description"

	tOrd := fix.createDeliveredOrder(t, time.Now().Add(-1*time.Hour), 1)
	ev := fix.createStagedEvidence(t, fix.userID, 2)

	// Request with top-level canonical reason, comment, NO item reason, NO item condition (matching new Shop contract)
	resp, err := fix.svc.CreateReturn(ctx, fix.userID, tOrd.orderID, returns.CreateReturnRequest{
		Reason:  "damaged",
		Comment: &testComment,
		Items: []returns.CreateReturnItemRequest{
			{
				OrderItemID: tOrd.orderItemID,
				Quantity:    1,
				EvidenceIDs: ev,
				Reason:      nil, // No item-level reason
				Condition:   nil, // No customer-supplied condition
			},
		},
	})
	require.NoError(t, err)
	require.Len(t, resp, 1)
	require.Len(t, resp[0].Items, 1)

	returnItem := resp[0].Items[0]
	// Assert backend copied canonical reason and defaulted condition for backward schema safety
	require.NotNil(t, returnItem.Reason)
	assert.Equal(t, "damaged", *returnItem.Reason, "Backend must populate return_items.reason from canonical return reason")
	require.NotNil(t, returnItem.Condition)
	assert.Equal(t, "new", *returnItem.Condition, "Backend must default condition to 'new' for compatibility")
}

// 8. ROUTER LEVEL AUTH, UPLOAD, SIZE LIMIT & DELETE CONTRACT
func TestM531A_RouterAuthUploadAndSizeLimit(t *testing.T) {
	ctx := context.Background()

	cfg, err := config.Load()
	require.NoError(t, err)
	cfg.RateLimit.Enabled = false
	cfg.Worker.ReturnWindowDays = 14

	pgClient, err := postgres.NewClient(ctx, "postgres://zamk:zamk_password@localhost:5433/zamk_test?sslmode=disable")
	require.NoError(t, err)
	defer pgClient.Close()

	dummyRedis := &redis.Client{Client: goredis.NewClient(&goredis.Options{})}
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelError}))

	r, cancel := app.BuildRouter(ctx, cfg, pgClient, dummyRedis, logger)
	t.Cleanup(cancel)

	tokenSvc := auth.NewTokenService(cfg.JWT.AccessTokenSecret, cfg.JWT.RefreshTokenSecret, cfg.JWT.AccessTokenTTLMinutes)
	customerID := uuid.New()
	customerEmail := fmt.Sprintf("cust_%s@test.com", uuid.New().String()[:8])
	_, err = pgClient.Pool.Exec(ctx, "INSERT INTO users (id, name, phone, email, password_hash, role) VALUES ($1, 'Customer', '+1234567890', $2, 'hash', 'customer')", customerID, customerEmail)
	require.NoError(t, err)

	customerToken, err := tokenSvc.GenerateAccessToken(customerID, customerEmail, "customer")
	require.NoError(t, err)

	sellerID := uuid.New()
	sellerEmail := fmt.Sprintf("seller_%s@test.com", uuid.New().String()[:8])
	_, err = pgClient.Pool.Exec(ctx, "INSERT INTO users (id, name, phone, email, password_hash, role) VALUES ($1, 'Seller', '+1234567891', $2, 'hash', 'seller')", sellerID, sellerEmail)
	require.NoError(t, err)

	sellerToken, err := tokenSvc.GenerateAccessToken(sellerID, sellerEmail, "seller")
	require.NoError(t, err)

	// Helper to create multipart form request
	createUploadReq := func(token string, filename string, contentType string, content []byte) *http.Request {
		body := &bytes.Buffer{}
		writer := multipart.NewWriter(body)
		partHeader := make(map[string][]string)
		partHeader["Content-Disposition"] = []string{fmt.Sprintf(`form-data; name="file"; filename="%s"`, filename)}
		partHeader["Content-Type"] = []string{contentType}
		part, _ := writer.CreatePart(partHeader)
		part.Write(content)
		writer.Close()

		req := httptest.NewRequest("POST", "/api/customer/returns/evidence/upload", body)
		req.Header.Set("Content-Type", writer.FormDataContentType())
		if token != "" {
			req.Header.Set("Authorization", "Bearer "+token)
		}
		return req
	}

	// 1. Unauthenticated upload -> 401 Unauthorized
	reqUnauth := createUploadReq("", "photo.jpg", "image/jpeg", []byte("fake-jpeg-content"))
	recUnauth := httptest.NewRecorder()
	r.ServeHTTP(recUnauth, reqUnauth)
	assert.Equal(t, http.StatusUnauthorized, recUnauth.Code)

	// 2. Seller token -> 403 Forbidden
	reqSeller := createUploadReq(sellerToken, "photo.jpg", "image/jpeg", []byte("fake-jpeg-content"))
	recSeller := httptest.NewRecorder()
	r.ServeHTTP(recSeller, reqSeller)
	assert.Equal(t, http.StatusForbidden, recSeller.Code)

	// 3. Valid customer upload JPEG -> 200 OK
	reqJPEG := createUploadReq(customerToken, "photo.jpg", "image/jpeg", []byte("fake-jpeg-content"))
	recJPEG := httptest.NewRecorder()
	r.ServeHTTP(recJPEG, reqJPEG)
	assert.Equal(t, http.StatusOK, recJPEG.Code)

	var uploadResp returns.UploadEvidenceResponse
	err = json.NewDecoder(recJPEG.Body).Decode(&uploadResp)
	require.NoError(t, err)
	assert.NotEqual(t, uuid.Nil, uploadResp.ID)
	assert.NotEmpty(t, uploadResp.URL)

	// 4. Valid customer upload PNG -> 200 OK
	reqPNG := createUploadReq(customerToken, "photo.png", "image/png", []byte("fake-png-content"))
	recPNG := httptest.NewRecorder()
	r.ServeHTTP(recPNG, reqPNG)
	assert.Equal(t, http.StatusOK, recPNG.Code)

	// 5. Valid customer upload WEBP -> 200 OK
	reqWEBP := createUploadReq(customerToken, "photo.webp", "image/webp", []byte("fake-webp-content"))
	recWEBP := httptest.NewRecorder()
	r.ServeHTTP(recWEBP, reqWEBP)
	assert.Equal(t, http.StatusOK, recWEBP.Code)

	// 6. Unsupported media format (.txt / text/plain) -> 400 Bad Request
	reqBad := createUploadReq(customerToken, "document.txt", "text/plain", []byte("plain text"))
	recBad := httptest.NewRecorder()
	r.ServeHTTP(recBad, reqBad)
	assert.Equal(t, http.StatusBadRequest, recBad.Code)

	// 7. Oversized file (> 10MB limit) -> 400 Bad Request
	oversizedContent := make([]byte, 11*1024*1024) // 11 MB
	reqOversized := createUploadReq(customerToken, "large_photo.jpg", "image/jpeg", oversizedContent)
	recOversized := httptest.NewRecorder()
	r.ServeHTTP(recOversized, reqOversized)
	assert.Equal(t, http.StatusBadRequest, recOversized.Code, "Oversized upload must be rejected with 400")

	// 8. Customer DELETE own staged evidence -> 204 No Content
	reqDel := httptest.NewRequest("DELETE", fmt.Sprintf("/api/customer/returns/evidence/%s", uploadResp.ID), nil)
	reqDel.Header.Set("Authorization", "Bearer "+customerToken)
	recDel := httptest.NewRecorder()
	r.ServeHTTP(recDel, reqDel)
	assert.Equal(t, http.StatusNoContent, recDel.Code)

	// 9. Customer DELETE already-deleted or nonexistent evidence -> 404 Not Found
	reqDelAgain := httptest.NewRequest("DELETE", fmt.Sprintf("/api/customer/returns/evidence/%s", uploadResp.ID), nil)
	reqDelAgain.Header.Set("Authorization", "Bearer "+customerToken)
	recDelAgain := httptest.NewRecorder()
	r.ServeHTTP(recDelAgain, reqDelAgain)
	assert.Equal(t, http.StatusNotFound, recDelAgain.Code)

	// 10. Unauthenticated DELETE -> 401
	reqDelUnauth := httptest.NewRequest("DELETE", fmt.Sprintf("/api/customer/returns/evidence/%s", uuid.New()), nil)
	recDelUnauth := httptest.NewRecorder()
	r.ServeHTTP(recDelUnauth, reqDelUnauth)
	assert.Equal(t, http.StatusUnauthorized, recDelUnauth.Code)

	// 11. Seller DELETE on customer endpoint -> 403
	reqDelSeller := httptest.NewRequest("DELETE", fmt.Sprintf("/api/customer/returns/evidence/%s", uuid.New()), nil)
	reqDelSeller.Header.Set("Authorization", "Bearer "+sellerToken)
	recDelSeller := httptest.NewRecorder()
	r.ServeHTTP(recDelSeller, reqDelSeller)
	assert.Equal(t, http.StatusForbidden, recDelSeller.Code)
}
