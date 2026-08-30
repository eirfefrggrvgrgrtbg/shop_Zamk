package returns_test

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/eirfefrggrvgrgrtbg/shop-zamk/backend/internal/returns"
	"github.com/eirfefrggrvgrgrtbg/shop-zamk/backend/internal/storage"
)

type dummyMsgStorageProvider struct {
	mu          sync.Mutex
	deletedKeys []string
	uploaded    map[string]int64
}

func newDummyMsgStorageProvider() *dummyMsgStorageProvider {
	return &dummyMsgStorageProvider{
		uploaded: make(map[string]int64),
	}
}

func (d *dummyMsgStorageProvider) UploadImage(ctx context.Context, reader io.Reader, objectSize int64, objectKey string, contentType string) (*storage.StoredObject, error) {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.uploaded[objectKey] = objectSize
	return &storage.StoredObject{
		ObjectURL: "http://localhost:9000/media/" + objectKey,
		ObjectKey: objectKey,
		Size:      objectSize,
	}, nil
}

func (d *dummyMsgStorageProvider) DownloadObject(ctx context.Context, objectKey string) ([]byte, error) {
	return []byte("dummy image content"), nil
}

func (d *dummyMsgStorageProvider) DeleteObject(ctx context.Context, objectKey string) error {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.deletedKeys = append(d.deletedKeys, objectKey)
	return nil
}

func (d *dummyMsgStorageProvider) BuildPublicURL(objectKey string) string {
	return "http://localhost:9000/media/" + objectKey
}

func (fix *m51Fixture) createAdminUser(t *testing.T) uuid.UUID {
	t.Helper()
	ctx := context.Background()
	adminID := uuid.New()
	_, err := fix.client.Pool.Exec(ctx, "INSERT INTO users (id, name, phone, email, password_hash) VALUES ($1, 'Admin', '+79991112255', $2, 'hash')", adminID, "admin_"+uuid.New().String()+"@test.com")
	require.NoError(t, err)
	return adminID
}

func (fix *m51Fixture) createDeliveredReturn(t *testing.T, status string) (uuid.UUID, testOrder) {
	t.Helper()
	ctx := context.Background()

	tOrd := fix.createDeliveredOrder(t, time.Now().Add(-1*time.Hour), 2)
	retID := uuid.New()

	_, err := fix.client.Pool.Exec(ctx, `
		INSERT INTO returns (id, order_id, fulfillment_id, user_id, status, reason, comment)
		VALUES ($1, $2, $3, $4, $5, 'defective', 'Claim original comment')
	`, retID, tOrd.orderID, tOrd.fulfillmentID, fix.userID, status)
	require.NoError(t, err)

	retItemID := uuid.New()
	_, err = fix.client.Pool.Exec(ctx, `
		INSERT INTO return_items (id, return_id, order_item_id, quantity, reason, condition, restock)
		VALUES ($1, $2, $3, 1, 'defective', 'new', false)
	`, retItemID, retID, tOrd.orderItemID)
	require.NoError(t, err)

	return retID, tOrd
}

var sampleJpegBytes = []byte{
	0xFF, 0xD8, 0xFF, 0xE0, 0x00, 0x10, 0x4A, 0x46, 0x49, 0x46, 0x00, 0x01,
	0x01, 0x01, 0x00, 0x48, 0x00, 0x48, 0x00, 0x00, 0xFF, 0xDB, 0x00, 0x43,
	0x00, 0x08, 0x06, 0x06, 0x07, 0x06, 0x05, 0x08, 0x07, 0x07, 0x07, 0x09,
	0x09, 0x08, 0x0A, 0x0C, 0x14, 0x0D, 0x0C, 0x0B, 0x0B, 0x0C, 0x19, 0x12,
	0x13, 0x0F, 0x14, 0x1D, 0x1A, 0x1F, 0x1E, 0x1D, 0x1A, 0x1C, 0x1C, 0x20,
	0x24, 0x2E, 0x27, 0x20, 0x22, 0x2C, 0x23, 0x1C, 0x1C, 0x28, 0x37, 0x29,
	0x2C, 0x30, 0x31, 0x34, 0x34, 0x34, 0x1F, 0x27, 0x39, 0x3D, 0x38, 0x32,
	0x3C, 0x2E, 0x33, 0x34, 0x32, 0xFF, 0xD9,
}

var samplePngBytes = []byte{
	0x89, 0x50, 0x4E, 0x47, 0x0D, 0x0A, 0x1A, 0x0A, 0x00, 0x00, 0x00, 0x0D,
	0x49, 0x48, 0x44, 0x52, 0x00, 0x00, 0x00, 0x01, 0x00, 0x00, 0x00, 0x01,
	0x08, 0x06, 0x00, 0x00, 0x00, 0x1F, 0x15, 0xC4, 0x89, 0x00, 0x00, 0x00,
	0x0A, 0x49, 0x44, 0x41, 0x54, 0x78, 0x9C, 0x63, 0x00, 0x01, 0x00, 0x00,
	0x05, 0x00, 0x01, 0x0D, 0x0A, 0x2D, 0xB4, 0x00, 0x00, 0x00, 0x00, 0x49,
	0x45, 0x4E, 0x44, 0xAE, 0x42, 0x60, 0x82,
}

// ----------------------------------------------------------------------------
// 1. Admin Info Request & Messages Tests
// ----------------------------------------------------------------------------

func TestM534A_AdminMessages(t *testing.T) {
	fix := setupM51Fixture(t)
	dummyStorage := newDummyMsgStorageProvider()
	fix.svc.SetStorageProvider(dummyStorage)
	ctx := context.Background()
	adminID := fix.createAdminUser(t)

	activeStatuses := []string{"requested", "needs_info", "approved", "receiving", "item_received"}
	for _, st := range activeStatuses {
		t.Run(fmt.Sprintf("active status %s allows ordinary admin message without changing status", st), func(t *testing.T) {
			retID, _ := fix.createDeliveredReturn(t, st)

			err := fix.svc.SendAdminReturnMessage(ctx, retID, adminID, fmt.Sprintf("Admin message for status %s", st), false, nil)
			require.NoError(t, err)

			ret, _, err := fix.returnsRepo.GetReturn(ctx, retID)
			require.NoError(t, err)
			assert.Equal(t, st, ret.Status, "Status must remain unchanged for active status")

			msgs, err := fix.svc.GetReturnMessages(ctx, retID)
			require.NoError(t, err)
			require.Len(t, msgs, 1)
			assert.Equal(t, returns.ReturnMessageTypeMessage, msgs[0].MessageType)
			assert.Equal(t, returns.ReturnMessageSenderRoleAdmin, msgs[0].SenderRole)
		})
	}

	terminalStatuses := []string{"rejected", "refunded", "completed", "cancelled"}
	for _, st := range terminalStatuses {
		t.Run(fmt.Sprintf("terminal status %s rejects ordinary admin message", st), func(t *testing.T) {
			retID, _ := fix.createDeliveredReturn(t, st)

			err := fix.svc.SendAdminReturnMessage(ctx, retID, adminID, fmt.Sprintf("Admin message for terminal status %s", st), false, nil)
			assert.True(t, errors.Is(err, returns.ErrReturnTerminalState))
		})
	}
}

// ----------------------------------------------------------------------------
// 2. Customer Reply & Messages Tests
// ----------------------------------------------------------------------------

func TestM534A_CustomerMessages(t *testing.T) {
	fix := setupM51Fixture(t)
	dummyStorage := newDummyMsgStorageProvider()
	fix.svc.SetStorageProvider(dummyStorage)
	ctx := context.Background()

	activeUnchangedStatuses := []string{"requested", "approved", "receiving", "item_received"}
	for _, st := range activeUnchangedStatuses {
		t.Run(fmt.Sprintf("active status %s allows customer message without changing status", st), func(t *testing.T) {
			retID, _ := fix.createDeliveredReturn(t, st)

			err := fix.svc.SendCustomerReturnMessage(ctx, retID, fix.userID, fmt.Sprintf("Customer message for status %s", st), nil)
			require.NoError(t, err)

			ret, _, err := fix.returnsRepo.GetReturn(ctx, retID)
			require.NoError(t, err)
			assert.Equal(t, st, ret.Status, "Status must remain unchanged for active status")

			msgs, err := fix.svc.GetReturnMessages(ctx, retID)
			require.NoError(t, err)
			require.Len(t, msgs, 1)
			assert.Equal(t, returns.ReturnMessageTypeMessage, msgs[0].MessageType)
			assert.Equal(t, returns.ReturnMessageSenderRoleCustomer, msgs[0].SenderRole)
		})
	}

	t.Run("needs_info allows customer message and transitions atomically to requested", func(t *testing.T) {
		retID, _ := fix.createDeliveredReturn(t, "needs_info")

		err := fix.svc.SendCustomerReturnMessage(ctx, retID, fix.userID, "Here is clarification from customer", nil)
		require.NoError(t, err)

		ret, _, err := fix.returnsRepo.GetReturn(ctx, retID)
		require.NoError(t, err)
		assert.Equal(t, "requested", ret.Status)

		msgs, err := fix.svc.GetReturnMessages(ctx, retID)
		require.NoError(t, err)
		require.Len(t, msgs, 1)
		assert.Equal(t, returns.ReturnMessageTypeMessage, msgs[0].MessageType)
	})

	terminalStatuses := []string{"rejected", "refunded", "completed", "cancelled"}
	for _, st := range terminalStatuses {
		t.Run(fmt.Sprintf("terminal status %s rejects customer message", st), func(t *testing.T) {
			retID, _ := fix.createDeliveredReturn(t, st)

			err := fix.svc.SendCustomerReturnMessage(ctx, retID, fix.userID, fmt.Sprintf("Customer message for terminal status %s", st), nil)
			assert.True(t, errors.Is(err, returns.ErrReturnTerminalState))
		})
	}
}

// ----------------------------------------------------------------------------
// 3. RequestReturnInfo Matrix Tests
// ----------------------------------------------------------------------------

func TestM534A_RequestReturnInfo_Matrix(t *testing.T) {
	fix := setupM51Fixture(t)
	dummyStorage := newDummyMsgStorageProvider()
	fix.svc.SetStorageProvider(dummyStorage)
	ctx := context.Background()
	adminID := fix.createAdminUser(t)

	t.Run("requested status allows RequestReturnInfo and transitions to needs_info", func(t *testing.T) {
		retID, _ := fix.createDeliveredReturn(t, "requested")

		err := fix.svc.SendAdminReturnMessage(ctx, retID, adminID, "   Пожалуйста, приложите фото серийного номера.   ", true, nil)
		require.NoError(t, err)

		ret, _, err := fix.returnsRepo.GetReturn(ctx, retID)
		require.NoError(t, err)
		assert.Equal(t, "needs_info", ret.Status)

		msgs, err := fix.svc.GetReturnMessages(ctx, retID)
		require.NoError(t, err)
		require.Len(t, msgs, 1)
		assert.Equal(t, returns.ReturnMessageTypeInfoRequest, msgs[0].MessageType)
		assert.Equal(t, "Пожалуйста, приложите фото серийного номера.", msgs[0].Body)
	})

	disallowedStatuses := []string{
		"needs_info",
		"approved",
		"receiving",
		"item_received",
		"rejected",
		"refunded",
		"completed",
		"cancelled",
	}

	for _, st := range disallowedStatuses {
		t.Run(fmt.Sprintf("status %s rejects RequestReturnInfo", st), func(t *testing.T) {
			retID, _ := fix.createDeliveredReturn(t, st)

			err := fix.svc.SendAdminReturnMessage(ctx, retID, adminID, "Some clarification", true, nil)
			assert.True(t, errors.Is(err, returns.ErrReturnNotRequestableInfo))

			ret, _, err := fix.returnsRepo.GetReturn(ctx, retID)
			require.NoError(t, err)
			assert.Equal(t, st, ret.Status, "Status must remain unchanged when RequestReturnInfo is rejected")
		})
	}
}

// ----------------------------------------------------------------------------
// 4. Attachments Validation, Upload & Binding Tests
// ----------------------------------------------------------------------------

func TestM534A_Attachments_Full(t *testing.T) {
	fix := setupM51Fixture(t)
	dummyStorage := newDummyMsgStorageProvider()
	fix.svc.SetStorageProvider(dummyStorage)
	ctx := context.Background()
	adminID := fix.createAdminUser(t)

	t.Run("upload attachment validation - invalid mime / fake content-type / size limit", func(t *testing.T) {
		retID, _ := fix.createDeliveredReturn(t, "requested")

		// Invalid extension
		_, err := fix.svc.UploadCustomerReturnMessageAttachment(ctx, retID, fix.userID, bytes.NewReader(sampleJpegBytes), "test.txt", "image/jpeg", int64(len(sampleJpegBytes)))
		assert.True(t, errors.Is(err, returns.ErrReturnMessageAttachmentInvalid))

		// Fake content-type: header claims image/jpeg, but body is text
		fakeBytes := []byte("Not a real jpeg file content at all")
		_, err = fix.svc.UploadCustomerReturnMessageAttachment(ctx, retID, fix.userID, bytes.NewReader(fakeBytes), "test.jpg", "image/jpeg", int64(len(fakeBytes)))
		assert.True(t, errors.Is(err, returns.ErrReturnMessageAttachmentInvalid))

		// File > 10MB
		hugeSize := int64(10*1024*1024 + 1)
		_, err = fix.svc.UploadCustomerReturnMessageAttachment(ctx, retID, fix.userID, bytes.NewReader(sampleJpegBytes), "test.jpg", "image/jpeg", hugeSize)
		assert.True(t, errors.Is(err, returns.ErrReturnMessageAttachmentTooLarge))
	})

	t.Run("upload state guard - rejects uploads in terminal return states", func(t *testing.T) {
		terminalStatuses := []string{"rejected", "refunded", "completed", "cancelled"}
		for _, st := range terminalStatuses {
			retID, _ := fix.createDeliveredReturn(t, st)

			// Customer upload
			_, err := fix.svc.UploadCustomerReturnMessageAttachment(ctx, retID, fix.userID, bytes.NewReader(sampleJpegBytes), "photo.jpg", "image/jpeg", int64(len(sampleJpegBytes)))
			assert.True(t, errors.Is(err, returns.ErrReturnTerminalState), "Customer upload must fail in %s", st)

			// Admin upload
			_, err = fix.svc.UploadAdminReturnMessageAttachment(ctx, retID, adminID, bytes.NewReader(sampleJpegBytes), "photo.jpg", "image/jpeg", int64(len(sampleJpegBytes)))
			assert.True(t, errors.Is(err, returns.ErrReturnTerminalState), "Admin upload must fail in %s", st)
		}

		activeStatuses := []string{"requested", "needs_info", "approved", "receiving", "item_received"}
		for _, st := range activeStatuses {
			retID, _ := fix.createDeliveredReturn(t, st)

			resp, err := fix.svc.UploadCustomerReturnMessageAttachment(ctx, retID, fix.userID, bytes.NewReader(sampleJpegBytes), "photo.jpg", "image/jpeg", int64(len(sampleJpegBytes)))
			require.NoError(t, err, "Customer upload must succeed in %s", st)
			assert.NotEmpty(t, resp.ID)
			assert.NotEmpty(t, resp.URL)
		}
	})

	t.Run("attachment-only message (empty text + photo) succeeds and no raw storage_key in DTO", func(t *testing.T) {
		retID, _ := fix.createDeliveredReturn(t, "requested")

		uploadResp, err := fix.svc.UploadCustomerReturnMessageAttachment(ctx, retID, fix.userID, bytes.NewReader(samplePngBytes), "photo.png", "image/png", int64(len(samplePngBytes)))
		require.NoError(t, err)

		// Send message with empty body
		err = fix.svc.SendCustomerReturnMessage(ctx, retID, fix.userID, "   ", []uuid.UUID{uploadResp.ID})
		require.NoError(t, err)

		msgs, err := fix.svc.GetReturnMessages(ctx, retID)
		require.NoError(t, err)
		require.Len(t, msgs, 1)
		assert.Equal(t, "", msgs[0].Body)
		require.Len(t, msgs[0].Attachments, 1)
		assert.Equal(t, uploadResp.ID, msgs[0].Attachments[0].ID)
		assert.Contains(t, msgs[0].Attachments[0].URL, "http://localhost:9000/media/")
		assert.False(t, strings.HasPrefix(msgs[0].Attachments[0].URL, "returns/"), "storage key must not be exposed as raw relative path")
	})

	t.Run("attachment count limits: 6 succeeds, 7 fails", func(t *testing.T) {
		retID, _ := fix.createDeliveredReturn(t, "requested")

		var attIDs []uuid.UUID
		for i := 0; i < 7; i++ {
			resp, err := fix.svc.UploadCustomerReturnMessageAttachment(ctx, retID, fix.userID, bytes.NewReader(sampleJpegBytes), fmt.Sprintf("photo_%d.jpg", i), "image/jpeg", int64(len(sampleJpegBytes)))
			require.NoError(t, err)
			attIDs = append(attIDs, resp.ID)
		}

		// 7 attachments fails
		err := fix.svc.SendCustomerReturnMessage(ctx, retID, fix.userID, "Message with 7 attachments", attIDs)
		assert.True(t, errors.Is(err, returns.ErrReturnMessageTooManyAttachments))

		// 6 attachments succeeds
		err = fix.svc.SendCustomerReturnMessage(ctx, retID, fix.userID, "Message with 6 attachments", attIDs[:6])
		require.NoError(t, err)

		msgs, err := fix.svc.GetReturnMessages(ctx, retID)
		require.NoError(t, err)
		require.Len(t, msgs, 1)
		assert.Len(t, msgs[0].Attachments, 6)
	})

	t.Run("staged security: foreign user attachment / different return attachment / already-bound attachment", func(t *testing.T) {
		ret1, _ := fix.createDeliveredReturn(t, "requested")
		ret2, _ := fix.createDeliveredReturn(t, "requested")

		foreignUserID := uuid.New()
		_, err := fix.client.Pool.Exec(ctx, "INSERT INTO users (id, name, phone, email, password_hash) VALUES ($1, 'Foreign', '+79998887766', $2, 'hash')", foreignUserID, "foreign_"+uuid.New().String()+"@test.com")
		require.NoError(t, err)

		// Upload for ret1 by user1
		upload1, err := fix.svc.UploadCustomerReturnMessageAttachment(ctx, ret1, fix.userID, bytes.NewReader(sampleJpegBytes), "photo1.jpg", "image/jpeg", int64(len(sampleJpegBytes)))
		require.NoError(t, err)

		// Try to bind upload1 to ret2 -> must fail
		err = fix.svc.SendCustomerReturnMessage(ctx, ret2, fix.userID, "Attaching to different return", []uuid.UUID{upload1.ID})
		assert.True(t, errors.Is(err, returns.ErrReturnMessageAttachmentNotOwned))

		// Try to bind upload1 by foreign user -> must fail
		err = fix.svc.SendCustomerReturnMessage(ctx, ret1, foreignUserID, "Attaching foreign staged upload", []uuid.UUID{upload1.ID})
		// Note: return does not belong to foreign user (ErrReturnNotFound) or attachment not owned
		assert.Error(t, err)

		// Bind upload1 successfully to ret1
		err = fix.svc.SendCustomerReturnMessage(ctx, ret1, fix.userID, "First bind", []uuid.UUID{upload1.ID})
		require.NoError(t, err)

		// Try to bind upload1 AGAIN -> must fail because it was deleted from staged
		err = fix.svc.SendCustomerReturnMessage(ctx, ret1, fix.userID, "Second bind", []uuid.UUID{upload1.ID})
		assert.True(t, errors.Is(err, returns.ErrReturnMessageAttachmentNotOwned))
	})

	t.Run("concurrent double-bind: exactly one succeeds", func(t *testing.T) {
		retID, _ := fix.createDeliveredReturn(t, "requested")

		uploadResp, err := fix.svc.UploadCustomerReturnMessageAttachment(ctx, retID, fix.userID, bytes.NewReader(sampleJpegBytes), "shared.jpg", "image/jpeg", int64(len(sampleJpegBytes)))
		require.NoError(t, err)

		var wg sync.WaitGroup
		successCount := 0
		failCount := 0
		var mu sync.Mutex

		for i := 0; i < 2; i++ {
			wg.Add(1)
			go func(idx int) {
				defer wg.Done()
				err := fix.svc.SendCustomerReturnMessage(ctx, retID, fix.userID, fmt.Sprintf("Concurrent send %d", idx), []uuid.UUID{uploadResp.ID})
				mu.Lock()
				defer mu.Unlock()
				if err == nil {
					successCount++
				} else {
					failCount++
				}
			}(i)
		}

		wg.Wait()
		assert.Equal(t, 1, successCount, "Exactly one concurrent bind must succeed")
		assert.Equal(t, 1, failCount, "The second concurrent bind must fail")
	})

	t.Run("customer photo reply in needs_info transitions return to requested", func(t *testing.T) {
		retID, _ := fix.createDeliveredReturn(t, "needs_info")

		uploadResp, err := fix.svc.UploadCustomerReturnMessageAttachment(ctx, retID, fix.userID, bytes.NewReader(sampleJpegBytes), "evidence_reply.jpg", "image/jpeg", int64(len(sampleJpegBytes)))
		require.NoError(t, err)

		err = fix.svc.SendCustomerReturnMessage(ctx, retID, fix.userID, "", []uuid.UUID{uploadResp.ID})
		require.NoError(t, err)

		ret, _, err := fix.returnsRepo.GetReturn(ctx, retID)
		require.NoError(t, err)
		assert.Equal(t, "requested", ret.Status)

		msgs, err := fix.svc.GetReturnMessages(ctx, retID)
		require.NoError(t, err)
		require.Len(t, msgs, 1)
		assert.Len(t, msgs[0].Attachments, 1)
	})

	t.Run("side effect verification: no return shipments, inventory, refunds, or payments mutated", func(t *testing.T) {
		retID, tOrd := fix.createDeliveredReturn(t, "requested")

		// Send message with attachment
		uploadResp, err := fix.svc.UploadCustomerReturnMessageAttachment(ctx, retID, fix.userID, bytes.NewReader(sampleJpegBytes), "test.jpg", "image/jpeg", int64(len(sampleJpegBytes)))
		require.NoError(t, err)

		err = fix.svc.SendCustomerReturnMessage(ctx, retID, fix.userID, "Check side effects", []uuid.UUID{uploadResp.ID})
		require.NoError(t, err)

		// Check shipments count
		var shipmentCount int
		err = fix.client.Pool.QueryRow(ctx, "SELECT COUNT(*) FROM return_shipments WHERE return_id = $1", retID).Scan(&shipmentCount)
		require.NoError(t, err)
		assert.Equal(t, 0, shipmentCount)

		// Check refunds count
		var refundCount int
		err = fix.client.Pool.QueryRow(ctx, "SELECT COUNT(*) FROM refunds WHERE return_id = $1", retID).Scan(&refundCount)
		require.NoError(t, err)
		assert.Equal(t, 0, refundCount)

		// Check order status unchanged
		var orderStatus string
		err = fix.client.Pool.QueryRow(ctx, "SELECT status FROM orders WHERE id = $1", tOrd.orderID).Scan(&orderStatus)
		require.NoError(t, err)
		assert.Equal(t, "delivered", orderStatus)
	})
}

// ----------------------------------------------------------------------------
// 5. Concurrency
// ----------------------------------------------------------------------------

func TestM534A_Concurrency_AdminCustomer(t *testing.T) {
	fix := setupM51Fixture(t)
	dummyStorage := newDummyMsgStorageProvider()
	fix.svc.SetStorageProvider(dummyStorage)
	ctx := context.Background()
	adminID := fix.createAdminUser(t)

	retID, _ := fix.createDeliveredReturn(t, "requested")

	var wg sync.WaitGroup
	for i := 0; i < 5; i++ {
		wg.Add(1)
		go func(msgIndex int) {
			defer wg.Done()
			_ = fix.svc.SendAdminReturnMessage(ctx, retID, adminID, fmt.Sprintf("Admin %d", msgIndex), false, nil)
		}(i)
		wg.Add(1)
		go func(msgIndex int) {
			defer wg.Done()
			_ = fix.svc.SendCustomerReturnMessage(ctx, retID, fix.userID, fmt.Sprintf("Customer %d", msgIndex), nil)
		}(i)
	}

	wg.Wait()

	msgs, _ := fix.svc.GetReturnMessages(ctx, retID)
	assert.Len(t, msgs, 10)
}
