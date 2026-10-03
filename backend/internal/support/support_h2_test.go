package support_test

import (
	"bytes"
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"sync"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/eirfefrggrvgrgrtbg/shop-zamk/backend/internal/support"
)

// -------------------------------------------------------------------------------------------------
// Concurrency Race Tests
// -------------------------------------------------------------------------------------------------

func TestSupport_CaseConcurrent_StaffReadVsRequesterMessageRace(t *testing.T) {
	client, repo, svc := setupTestSupportDB(t)
	custID := createTestUser(t, client, "customer")
	staffID := createTestUser(t, client, "admin")
	defer cleanupFixtures(client, []uuid.UUID{custID, staffID}, nil)

	ctx := context.Background()
	_, err := svc.SendCustomerMessage(ctx, custID, support.SendMessageRequest{TextContent: "Старт диалога"})
	require.NoError(t, err)

	conv, err := svc.GetCustomerConversation(ctx, custID)
	require.NoError(t, err)

	var wg sync.WaitGroup
	numOps := 8
	errChan := make(chan error, numOps*2)

	wg.Add(2)
	go func() {
		defer wg.Done()
		for i := 0; i < numOps; i++ {
			_, err := svc.SendCustomerMessage(ctx, custID, support.SendMessageRequest{
				TextContent: fmt.Sprintf("Гонка сообщение %d", i),
			})
			if err != nil {
				errChan <- fmt.Errorf("customer send message %d: %w", i, err)
			}
			time.Sleep(1 * time.Millisecond)
		}
	}()

	go func() {
		defer wg.Done()
		for i := 0; i < numOps; i++ {
			err := svc.MarkStaffRead(ctx, staffID, conv.Conversation.ID)
			if err != nil {
				errChan <- fmt.Errorf("staff mark read %d: %w", i, err)
			}
			time.Sleep(1 * time.Millisecond)
		}
	}()

	wg.Wait()
	close(errChan)

	for err := range errChan {
		require.NoError(t, err)
	}

	staffUnread, err := repo.GetStaffUnreadCount(ctx, staffID, conv.Conversation.ID)
	require.NoError(t, err)
	assert.GreaterOrEqual(t, staffUnread, 0)
	assert.LessOrEqual(t, staffUnread, numOps+1)

	// Final read mark guarantees zero unread
	err = svc.MarkStaffRead(ctx, staffID, conv.Conversation.ID)
	require.NoError(t, err)

	finalUnread, err := repo.GetStaffUnreadCount(ctx, staffID, conv.Conversation.ID)
	require.NoError(t, err)
	assert.Equal(t, 0, finalUnread, "After final read mark, staff unread MUST be 0")
}

func TestSupport_CaseConcurrent_RequesterReadVsStaffReplyRace(t *testing.T) {
	client, repo, svc := setupTestSupportDB(t)
	custID := createTestUser(t, client, "customer")
	staffID := createTestUser(t, client, "admin")
	defer cleanupFixtures(client, []uuid.UUID{custID, staffID}, nil)

	ctx := context.Background()
	_, err := svc.SendCustomerMessage(ctx, custID, support.SendMessageRequest{TextContent: "Вопрос в поддержку"})
	require.NoError(t, err)

	conv, err := svc.GetCustomerConversation(ctx, custID)
	require.NoError(t, err)

	var wg sync.WaitGroup
	numOps := 8
	errChan := make(chan error, numOps*2)

	wg.Add(2)
	go func() {
		defer wg.Done()
		for i := 0; i < numOps; i++ {
			_, err := svc.SendStaffReply(ctx, conv.Conversation.ID, staffID, support.StaffReplyRequest{
				TextContent: fmt.Sprintf("Ответ сотрудника %d", i),
			})
			if err != nil {
				errChan <- fmt.Errorf("staff reply %d: %w", i, err)
			}
			time.Sleep(1 * time.Millisecond)
		}
	}()

	go func() {
		defer wg.Done()
		for i := 0; i < numOps; i++ {
			err := svc.MarkRequesterRead(ctx, custID, conv.Conversation.ID)
			if err != nil {
				errChan <- fmt.Errorf("customer mark read %d: %w", i, err)
			}
			time.Sleep(1 * time.Millisecond)
		}
	}()

	wg.Wait()
	close(errChan)

	for err := range errChan {
		require.NoError(t, err)
	}

	custUnread, err := repo.GetRequesterUnreadCount(ctx, custID, conv.Conversation.ID)
	require.NoError(t, err)
	assert.GreaterOrEqual(t, custUnread, 0)
	assert.LessOrEqual(t, custUnread, numOps)

	// Final read mark guarantees zero unread
	err = svc.MarkRequesterRead(ctx, custID, conv.Conversation.ID)
	require.NoError(t, err)

	finalUnread, err := repo.GetRequesterUnreadCount(ctx, custID, conv.Conversation.ID)
	require.NoError(t, err)
	assert.Equal(t, 0, finalUnread, "After final read mark, customer unread MUST be 0")
}

// -------------------------------------------------------------------------------------------------
// Tests AK through AP: First-Ever Attachment Flow & Staged Attachment Isolation
// -------------------------------------------------------------------------------------------------

func TestSupport_CaseAK_CustomerFirstEverAttachment(t *testing.T) {
	client, _, svc := setupTestSupportDB(t)
	custID := createTestUser(t, client, "customer")
	defer cleanupFixtures(client, []uuid.UUID{custID}, nil)

	ctx := context.Background()
	fileData := []byte("%PDF-1.4 test dummy pdf content for attachment")
	att, err := svc.UploadCustomerAttachment(ctx, custID, bytes.NewReader(fileData), "test.pdf", "application/pdf", int64(len(fileData)))
	require.NoError(t, err)
	assert.NotEqual(t, uuid.Nil, att.ID)
	assert.Equal(t, custID, att.UploaderUserID)
	assert.Equal(t, "application/pdf", att.ContentType)
}

func TestSupport_CaseAL_SellerFirstEverAttachment(t *testing.T) {
	client, _, svc := setupTestSupportDB(t)
	ownerID := createTestUser(t, client, "seller")
	sellerID := createTestSeller(t, client, ownerID)
	defer cleanupFixtures(client, []uuid.UUID{ownerID}, []uuid.UUID{sellerID})

	ctx := context.Background()
	fileData := []byte("%PDF-1.4 test seller pdf content")
	att, err := svc.UploadSellerAttachment(ctx, sellerID, ownerID, bytes.NewReader(fileData), "seller_invoice.pdf", "application/pdf", int64(len(fileData)))
	require.NoError(t, err)
	assert.NotEqual(t, uuid.Nil, att.ID)
	assert.Equal(t, ownerID, att.UploaderUserID)
}

func TestSupport_CaseAM_UploadCreatesConversationZeroSessions(t *testing.T) {
	client, repo, svc := setupTestSupportDB(t)
	custID := createTestUser(t, client, "customer")
	defer cleanupFixtures(client, []uuid.UUID{custID}, nil)

	ctx := context.Background()
	fileData := []byte("%PDF-1.4 sample content")
	att, err := svc.UploadCustomerAttachment(ctx, custID, bytes.NewReader(fileData), "doc.pdf", "application/pdf", int64(len(fileData)))
	require.NoError(t, err)

	// Conversation exists
	conv, err := repo.GetConversationByID(ctx, att.ConversationID)
	require.NoError(t, err)
	assert.Equal(t, custID, *conv.RequesterUserID)

	// ZERO sessions exist
	var sessionCount int
	err = client.Pool.QueryRow(ctx, `SELECT COUNT(*) FROM support_sessions WHERE conversation_id = $1`, conv.ID).Scan(&sessionCount)
	require.NoError(t, err)
	assert.Equal(t, 0, sessionCount, "Upload MUST create conversation with exactly 0 sessions")

	// GetCustomerConversation returns ActiveSession == nil
	detail, err := svc.GetCustomerConversation(ctx, custID)
	require.NoError(t, err)
	assert.Nil(t, detail.Conversation.ActiveSession, "ActiveSession MUST be nil before first message")
	assert.Empty(t, detail.Messages, "Messages MUST be empty before first message")
}

func TestSupport_CaseAN_FirstMessageBindsStagedAttachmentCreatesActiveSession(t *testing.T) {
	client, _, svc := setupTestSupportDB(t)
	custID := createTestUser(t, client, "customer")
	defer cleanupFixtures(client, []uuid.UUID{custID}, nil)

	ctx := context.Background()
	fileData := []byte("%PDF-1.4 sample content")
	att, err := svc.UploadCustomerAttachment(ctx, custID, bytes.NewReader(fileData), "doc.pdf", "application/pdf", int64(len(fileData)))
	require.NoError(t, err)

	// Send first message referencing the staged attachment
	msg, err := svc.SendCustomerMessage(ctx, custID, support.SendMessageRequest{
		TextContent:   "Вот прикрепленный файл",
		AttachmentIDs: []uuid.UUID{att.ID},
	})
	require.NoError(t, err)
	require.Len(t, msg.Attachments, 1)
	assert.Equal(t, att.ID, msg.Attachments[0].ID)

	// Verify exactly 1 ACTIVE session in DB
	var sessionCount int
	err = client.Pool.QueryRow(ctx, `SELECT COUNT(*) FROM support_sessions WHERE conversation_id = $1 AND status = 'ACTIVE'`, att.ConversationID).Scan(&sessionCount)
	require.NoError(t, err)
	assert.Equal(t, 1, sessionCount, "First message MUST create exactly 1 ACTIVE session")

	// Staged attachment removed from staged table
	var stagedCount int
	err = client.Pool.QueryRow(ctx, `SELECT COUNT(*) FROM support_staged_attachments WHERE id = $1`, att.ID).Scan(&stagedCount)
	require.NoError(t, err)
	assert.Equal(t, 0, stagedCount, "Bound staged attachment MUST be deleted from staged table")

	// Bound to support_attachments table
	var boundCount int
	err = client.Pool.QueryRow(ctx, `SELECT COUNT(*) FROM support_attachments WHERE message_id = $1`, msg.ID).Scan(&boundCount)
	require.NoError(t, err)
	assert.Equal(t, 1, boundCount, "Attachment MUST be stored in support_attachments table")
}

func TestSupport_CaseAO_AbandonedStagedUploadCreatesNoActiveSupportRequest(t *testing.T) {
	client, _, svc := setupTestSupportDB(t)
	custID := createTestUser(t, client, "customer")
	staffID := createTestUser(t, client, "admin")
	defer cleanupFixtures(client, []uuid.UUID{custID, staffID}, nil)

	ctx := context.Background()
	fileData := []byte("%PDF-1.4 abandoned upload")
	att, err := svc.UploadCustomerAttachment(ctx, custID, bytes.NewReader(fileData), "abandoned.pdf", "application/pdf", int64(len(fileData)))
	require.NoError(t, err)

	// Staff lists conversations (active inbox)
	convs, err := svc.ListConversations(ctx, "", staffID)
	require.NoError(t, err)

	for _, c := range convs {
		assert.NotEqual(t, att.ConversationID, c.ID, "Abandoned staged upload with 0 sessions MUST NOT appear in active inbox")
	}
}

func TestSupport_CaseAP_ForeignRequesterCannotBindAnotherRequesterStagedAttachment(t *testing.T) {
	client, _, svc := setupTestSupportDB(t)
	cust1 := createTestUser(t, client, "customer")
	cust2 := createTestUser(t, client, "customer")
	defer cleanupFixtures(client, []uuid.UUID{cust1, cust2}, nil)

	ctx := context.Background()
	fileData := []byte("%PDF-1.4 file of customer 1")
	att1, err := svc.UploadCustomerAttachment(ctx, cust1, bytes.NewReader(fileData), "cust1.pdf", "application/pdf", int64(len(fileData)))
	require.NoError(t, err)

	// Customer 2 attempts to send message referencing Customer 1's attachment ID
	_, err = svc.SendCustomerMessage(ctx, cust2, support.SendMessageRequest{
		TextContent:   "Попытка прикрепить чужой файл",
		AttachmentIDs: []uuid.UUID{att1.ID},
	})
	require.Error(t, err)
	assert.ErrorIs(t, err, support.ErrAttachmentInvalid, "Foreign requester binding another requester's attachment MUST fail")

	// Ensure Customer 1's attachment is still intact in staged table
	var stagedCount int
	err = client.Pool.QueryRow(ctx, `SELECT COUNT(*) FROM support_staged_attachments WHERE id = $1`, att1.ID).Scan(&stagedCount)
	require.NoError(t, err)
	assert.Equal(t, 1, stagedCount, "Foreign attempt MUST NOT alter or consume another requester's staged attachment")
}

// -------------------------------------------------------------------------------------------------
// Tests AQ through AS: Staff Assignment Validation
// -------------------------------------------------------------------------------------------------

func TestSupport_CaseAQ_AR_AS_AssignmentValidation(t *testing.T) {
	client, repo, svc := setupTestSupportDB(t)
	custID := createTestUser(t, client, "customer")
	activeStaff := createTestUser(t, client, "admin")
	normalCust := createTestUser(t, client, "customer")
	defer cleanupFixtures(client, []uuid.UUID{custID, activeStaff, normalCust}, nil)

	ctx := context.Background()
	_, err := svc.SendCustomerMessage(ctx, custID, support.SendMessageRequest{TextContent: "Нужна консультация"})
	require.NoError(t, err)

	conv, err := svc.GetCustomerConversation(ctx, custID)
	require.NoError(t, err)

	// AQ: Assign active staff -> PASS
	err = svc.UpdateSession(ctx, conv.Conversation.ID, support.UpdateSessionRequest{
		AssignedTo: &activeStaff,
	})
	require.NoError(t, err, "Assigning active staff member must succeed")

	sess, err := repo.GetActiveSession(ctx, conv.Conversation.ID)
	require.NoError(t, err)
	require.NotNil(t, sess.AssignedTo)
	assert.Equal(t, activeStaff, *sess.AssignedTo)

	// AR: Assign ordinary customer/non-staff -> REJECT
	err = svc.UpdateSession(ctx, conv.Conversation.ID, support.UpdateSessionRequest{
		AssignedTo: &normalCust,
	})
	require.Error(t, err)
	assert.ErrorIs(t, err, support.ErrInvalidAssignee, "Assigning non-staff user must be rejected")

	// AS: Foreign/nonexistent target -> REJECT
	fakeID := uuid.New()
	err = svc.UpdateSession(ctx, conv.Conversation.ID, support.UpdateSessionRequest{
		AssignedTo: &fakeID,
	})
	require.Error(t, err)
	assert.ErrorIs(t, err, support.ErrInvalidAssignee, "Assigning nonexistent user must be rejected")
}

// -------------------------------------------------------------------------------------------------
// Tests AT through BA: Category Scope & Inactive Assignment Validation
// -------------------------------------------------------------------------------------------------

func TestSupport_CaseAT_AU_AV_AW_AX_AY_AZ_BA_CategoryValidation(t *testing.T) {
	client, repo, svc := setupTestSupportDB(t)
	custID := createTestUser(t, client, "customer")
	ownerID := createTestUser(t, client, "seller")
	sellerID := createTestSeller(t, client, ownerID)
	staffID := createTestUser(t, client, "admin")
	defer cleanupFixtures(client, []uuid.UUID{custID, ownerID, staffID}, []uuid.UUID{sellerID})

	ctx := context.Background()
	_, err := svc.SendCustomerMessage(ctx, custID, support.SendMessageRequest{TextContent: "Вопрос клиента"})
	require.NoError(t, err)
	custConv, err := svc.GetCustomerConversation(ctx, custID)
	require.NoError(t, err)

	_, err = svc.SendSellerMessage(ctx, sellerID, ownerID, support.SendMessageRequest{TextContent: "Вопрос продавца"})
	require.NoError(t, err)
	sellerConv, err := svc.GetSellerConversation(ctx, sellerID, ownerID)
	require.NoError(t, err)

	catCustID := uuid.New()
	catSellerID := uuid.New()
	catBothID := uuid.New()
	catInactiveID := uuid.New()

	for _, c := range []struct {
		id     uuid.UUID
		name   string
		scope  string
		active bool
	}{
		{catCustID, "Только клиент", "CUSTOMER", true},
		{catSellerID, "Только продавец", "SELLER", true},
		{catBothID, "Для обоих", "BOTH", true},
		{catInactiveID, "Архивная категория", "BOTH", false},
	} {
		_, err := client.Pool.Exec(ctx, `
			INSERT INTO support_categories (id, name, requester_scope, active)
			VALUES ($1, $2, $3, $4)
		`, c.id, c.name, c.scope, c.active)
		require.NoError(t, err)
		defer client.Pool.Exec(ctx, `DELETE FROM support_categories WHERE id = $1`, c.id)
	}

	// AT: Customer + CUSTOMER category -> PASS
	err = svc.UpdateSession(ctx, custConv.Conversation.ID, support.UpdateSessionRequest{CategoryID: &catCustID})
	require.NoError(t, err, "Customer conversation with CUSTOMER category must pass")

	// AU: Customer + BOTH category -> PASS
	err = svc.UpdateSession(ctx, custConv.Conversation.ID, support.UpdateSessionRequest{CategoryID: &catBothID})
	require.NoError(t, err, "Customer conversation with BOTH category must pass")

	// AV: Customer + SELLER category -> REJECT
	err = svc.UpdateSession(ctx, custConv.Conversation.ID, support.UpdateSessionRequest{CategoryID: &catSellerID})
	require.Error(t, err)
	assert.ErrorIs(t, err, support.ErrInvalidCategoryScope, "Customer conversation with SELLER category must fail")

	// AW: Seller + SELLER category -> PASS
	err = svc.UpdateSession(ctx, sellerConv.Conversation.ID, support.UpdateSessionRequest{CategoryID: &catSellerID})
	require.NoError(t, err, "Seller conversation with SELLER category must pass")

	// AX: Seller + BOTH category -> PASS
	err = svc.UpdateSession(ctx, sellerConv.Conversation.ID, support.UpdateSessionRequest{CategoryID: &catBothID})
	require.NoError(t, err, "Seller conversation with BOTH category must pass")

	// AY: Seller + CUSTOMER category -> REJECT
	err = svc.UpdateSession(ctx, sellerConv.Conversation.ID, support.UpdateSessionRequest{CategoryID: &catCustID})
	require.Error(t, err)
	assert.ErrorIs(t, err, support.ErrInvalidCategoryScope, "Seller conversation with CUSTOMER category must fail")

	// AZ: Inactive category -> REJECT for new assignment
	err = svc.UpdateSession(ctx, custConv.Conversation.ID, support.UpdateSessionRequest{CategoryID: &catInactiveID})
	require.Error(t, err)
	assert.ErrorIs(t, err, support.ErrInvalidCategoryScope, "Assigning inactive category must fail")

	// BA: Null category -> PASS
	err = svc.UpdateSession(ctx, custConv.Conversation.ID, support.UpdateSessionRequest{ClearCategory: true})
	require.NoError(t, err, "Clearing / setting null category must pass")

	sess, err := repo.GetActiveSession(ctx, custConv.Conversation.ID)
	require.NoError(t, err)
	assert.Nil(t, sess.CategoryID, "Category must now be nil after clearing")

	// Historical integrity check: Session assigned to category which is then deactivated
	err = svc.UpdateSession(ctx, custConv.Conversation.ID, support.UpdateSessionRequest{CategoryID: &catCustID})
	require.NoError(t, err)
	_, err = client.Pool.Exec(ctx, `UPDATE support_categories SET active = false WHERE id = $1`, catCustID)
	require.NoError(t, err)

	histAdminView, err := svc.GetAdminConversation(ctx, custConv.Conversation.ID, staffID)
	require.NoError(t, err)
	require.NotNil(t, histAdminView.Conversation.ActiveSession)
	assert.Equal(t, catCustID, *histAdminView.Conversation.ActiveSession.CategoryID, "Historical deactivated category remains preserved")
	assert.Equal(t, "Только клиент", *histAdminView.Conversation.ActiveSession.CategoryName, "Historical category name remains readable")
}

// -------------------------------------------------------------------------------------------------
// External DTO Serialization: Strict Omission of Operational Metadata
// -------------------------------------------------------------------------------------------------

func TestSupport_ExternalDTOSerialization_OmissionOfOperationalMetadata(t *testing.T) {
	client, _, svc := setupTestSupportDB(t)
	custID := createTestUser(t, client, "customer")
	ownerID := createTestUser(t, client, "seller")
	sellerID := createTestSeller(t, client, ownerID)
	staffID := createTestUser(t, client, "admin")
	defer cleanupFixtures(client, []uuid.UUID{custID, ownerID, staffID}, []uuid.UUID{sellerID})

	ctx := context.Background()

	catID := uuid.New()
	_, err := client.Pool.Exec(ctx, `
		INSERT INTO support_categories (id, name, requester_scope, active)
		VALUES ($1, 'Тест категории', 'BOTH', true)
	`, catID)
	require.NoError(t, err)
	defer client.Pool.Exec(ctx, `DELETE FROM support_categories WHERE id = $1`, catID)

	_, err = svc.SendCustomerMessage(ctx, custID, support.SendMessageRequest{TextContent: "Клиентское сообщение"})
	require.NoError(t, err)
	custConv, err := svc.GetCustomerConversation(ctx, custID)
	require.NoError(t, err)

	prio := support.SessionPriorityUrgent
	err = svc.UpdateSession(ctx, custConv.Conversation.ID, support.UpdateSessionRequest{
		Priority:   &prio,
		CategoryID: &catID,
		AssignedTo: &staffID,
	})
	require.NoError(t, err)

	_, err = svc.CreateInternalNote(ctx, custConv.Conversation.ID, staffID, support.CreateInternalNoteRequest{
		TextContent: "Секретная внутренняя заметка поддержки",
	})
	require.NoError(t, err)

	err = svc.MarkStaffRead(ctx, staffID, custConv.Conversation.ID)
	require.NoError(t, err)

	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	h := support.NewHandler(svc, logger)

	// 1. Customer API JSON
	t.Run("Customer API JSON omits all operational metadata", func(t *testing.T) {
		r := chi.NewRouter()
		r.Use(func(next http.Handler) http.Handler {
			return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
				ctx := context.WithValue(req.Context(), "userID", custID)
				next.ServeHTTP(w, req.WithContext(ctx))
			})
		})
		r.Get("/support", h.GetCustomerConversation)

		req := httptest.NewRequest("GET", "/support", nil)
		rec := httptest.NewRecorder()
		r.ServeHTTP(rec, req)
		assert.Equal(t, http.StatusOK, rec.Code)

		rawJSON := rec.Body.String()
		assert.NotContains(t, rawJSON, "internalNotes", "Customer JSON MUST NOT contain internalNotes")
		assert.NotContains(t, rawJSON, "internal_notes", "Customer JSON MUST NOT contain internal_notes")
		assert.NotContains(t, rawJSON, "assignedTo", "Customer JSON MUST NOT contain assignedTo")
		assert.NotContains(t, rawJSON, "assigned_to", "Customer JSON MUST NOT contain assigned_to")
		assert.NotContains(t, rawJSON, "priority", "Customer JSON MUST NOT contain priority")
		assert.NotContains(t, rawJSON, "categoryId", "Customer JSON MUST NOT contain categoryId")
		assert.NotContains(t, rawJSON, "category_id", "Customer JSON MUST NOT contain category_id")
		assert.NotContains(t, rawJSON, "categoryName", "Customer JSON MUST NOT contain categoryName")
		assert.NotContains(t, rawJSON, "category_name", "Customer JSON MUST NOT contain category_name")
		assert.NotContains(t, rawJSON, "staff_read", "Customer JSON MUST NOT contain staff_read")
		assert.NotContains(t, rawJSON, "Секретная внутренняя заметка", "Customer JSON MUST NOT contain note content")
	})

	// 2. Seller API JSON
	t.Run("Seller API JSON omits all operational metadata", func(t *testing.T) {
		_, err = svc.SendSellerMessage(ctx, sellerID, ownerID, support.SendMessageRequest{TextContent: "Продавец сообщение"})
		require.NoError(t, err)
		sellerConv, err := svc.GetSellerConversation(ctx, sellerID, ownerID)
		require.NoError(t, err)

		err = svc.UpdateSession(ctx, sellerConv.Conversation.ID, support.UpdateSessionRequest{
			Priority:   &prio,
			CategoryID: &catID,
			AssignedTo: &staffID,
		})
		require.NoError(t, err)

		_, err = svc.CreateInternalNote(ctx, sellerConv.Conversation.ID, staffID, support.CreateInternalNoteRequest{
			TextContent: "Секретная заметка для продавца",
		})
		require.NoError(t, err)

		r := chi.NewRouter()
		r.Use(func(next http.Handler) http.Handler {
			return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
				ctx := context.WithValue(req.Context(), "userID", ownerID)
				ctx = context.WithValue(ctx, "sellerID", sellerID)
				next.ServeHTTP(w, req.WithContext(ctx))
			})
		})
		r.Get("/support", h.GetSellerConversation)

		req := httptest.NewRequest("GET", "/support", nil)
		rec := httptest.NewRecorder()
		r.ServeHTTP(rec, req)
		assert.Equal(t, http.StatusOK, rec.Code)

		rawJSON := rec.Body.String()
		assert.NotContains(t, rawJSON, "internalNotes", "Seller JSON MUST NOT contain internalNotes")
		assert.NotContains(t, rawJSON, "internal_notes", "Seller JSON MUST NOT contain internal_notes")
		assert.NotContains(t, rawJSON, "assignedTo", "Seller JSON MUST NOT contain assignedTo")
		assert.NotContains(t, rawJSON, "assigned_to", "Seller JSON MUST NOT contain assigned_to")
		assert.NotContains(t, rawJSON, "priority", "Seller JSON MUST NOT contain priority")
		assert.NotContains(t, rawJSON, "categoryId", "Seller JSON MUST NOT contain categoryId")
		assert.NotContains(t, rawJSON, "category_id", "Seller JSON MUST NOT contain category_id")
		assert.NotContains(t, rawJSON, "categoryName", "Seller JSON MUST NOT contain categoryName")
		assert.NotContains(t, rawJSON, "category_name", "Seller JSON MUST NOT contain category_name")
		assert.NotContains(t, rawJSON, "staff_read", "Seller JSON MUST NOT contain staff_read")
		assert.NotContains(t, rawJSON, "Секретная заметка для продавца", "Seller JSON MUST NOT contain note content")
	})

	// 3. Admin API JSON
	t.Run("Admin API JSON contains operational metadata", func(t *testing.T) {
		r := chi.NewRouter()
		r.Use(func(next http.Handler) http.Handler {
			return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
				ctx := context.WithValue(req.Context(), "userID", staffID)
				next.ServeHTTP(w, req.WithContext(ctx))
			})
		})
		r.Get("/admin/support/conversations/{id}", h.GetAdminConversation)

		req := httptest.NewRequest("GET", fmt.Sprintf("/admin/support/conversations/%s", custConv.Conversation.ID), nil)
		rec := httptest.NewRecorder()
		r.ServeHTTP(rec, req)
		assert.Equal(t, http.StatusOK, rec.Code)

		rawJSON := rec.Body.String()
		assert.Contains(t, rawJSON, "internalNotes", "Admin JSON MUST contain internalNotes")
		assert.Contains(t, rawJSON, "assignedTo", "Admin JSON MUST contain assignedTo")
		assert.Contains(t, rawJSON, "priority", "Admin JSON MUST contain priority")
		assert.Contains(t, rawJSON, "categoryId", "Admin JSON MUST contain categoryId")
		assert.Contains(t, rawJSON, "Секретная внутренняя заметка", "Admin JSON MUST contain note content")
	})
}
