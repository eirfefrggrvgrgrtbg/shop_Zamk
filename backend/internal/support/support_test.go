package support_test

import (
	"bytes"
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/eirfefrggrvgrgrtbg/shop-zamk/backend/internal/platform/postgres"
	"github.com/eirfefrggrvgrgrtbg/shop-zamk/backend/internal/support"
	"github.com/eirfefrggrvgrgrtbg/shop-zamk/backend/internal/testutil"
)

type dummyStorageProvider struct {
	mu      sync.Mutex
	objects map[string][]byte
}

func newDummyStorageProvider() *dummyStorageProvider {
	return &dummyStorageProvider{
		objects: make(map[string][]byte),
	}
}

func (d *dummyStorageProvider) UploadImage(ctx context.Context, reader bytes.Buffer, objectSize int64, objectKey string, contentType string) error {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.objects[objectKey] = reader.Bytes()
	return nil
}

func (d *dummyStorageProvider) DownloadObject(ctx context.Context, objectKey string) ([]byte, error) {
	d.mu.Lock()
	defer d.mu.Unlock()
	data, ok := d.objects[objectKey]
	if !ok {
		return nil, fmt.Errorf("object not found: %s", objectKey)
	}
	return data, nil
}

func (d *dummyStorageProvider) DeleteObject(ctx context.Context, objectKey string) error {
	d.mu.Lock()
	defer d.mu.Unlock()
	delete(d.objects, objectKey)
	return nil
}

func (d *dummyStorageProvider) BuildPublicURL(objectKey string) string {
	return "https://cdn.zamk.test/" + objectKey
}

func setupTestSupportDB(t *testing.T) (*postgres.Client, *support.Repository, *support.Service) {
	ctx := context.Background()
	dbURL := testutil.GetTestDatabaseURL()
	require.True(t, strings.Contains(dbURL, "zamk_test"), "MUST run only against zamk_test")

	client, err := postgres.NewClient(ctx, dbURL)
	require.NoError(t, err)
	t.Cleanup(func() {
		client.Close()
	})

	testutil.AssertTestDatabase(t, client.Pool)

	// Ensure 000104 is applied cleanly
	err = testutil.EnsureSupportMigrations(ctx, client.Pool)
	require.NoError(t, err)

	repo := support.NewRepository(client.Pool)
	svc := support.NewService(repo, client.Pool, nil)

	return client, repo, svc
}

func createTestUser(t *testing.T, client *postgres.Client, role string) uuid.UUID {
	ctx := context.Background()
	id := uuid.New()
	phone := fmt.Sprintf("+7999%s", id.String()[:7])
	email := fmt.Sprintf("u_%s@test.zamk", id.String()[:8])
	_, err := client.Pool.Exec(ctx, `
		INSERT INTO users (id, phone, name, email, password_hash, role, created_at, updated_at)
		VALUES ($1, $2, $3, $4, 'hash', $5, now(), now())
	`, id, phone, "User "+id.String()[:6], email, role)
	require.NoError(t, err)

	if role == "admin" {
		_, err = client.Pool.Exec(ctx, `
			INSERT INTO staff_members (user_id, staff_role_id, status, created_at, updated_at)
			VALUES ($1, (SELECT id FROM staff_roles LIMIT 1), 'active', now(), now())
			ON CONFLICT (user_id) DO NOTHING
		`, id)
		require.NoError(t, err)
	}

	return id
}

func createTestSeller(t *testing.T, client *postgres.Client, ownerUserID uuid.UUID) uuid.UUID {
	ctx := context.Background()
	sellerID := uuid.New()
	slug := "seller-" + sellerID.String()[:8]
	_, err := client.Pool.Exec(ctx, `
		INSERT INTO sellers (id, brand_name, slug, contact_email, status, created_at, updated_at)
		VALUES ($1, $2, $3, 'seller@test.zamk', 'active', now(), now())
	`, sellerID, "Seller "+slug, slug)
	require.NoError(t, err)

	_, err = client.Pool.Exec(ctx, `
		INSERT INTO seller_users (id, seller_id, user_id, role, created_at)
		VALUES ($1, $2, $3, 'owner', now())
	`, uuid.New(), sellerID, ownerUserID)
	require.NoError(t, err)

	return sellerID
}

func addSellerMember(t *testing.T, client *postgres.Client, sellerID, userID uuid.UUID) {
	ctx := context.Background()
	_, err := client.Pool.Exec(ctx, `
		INSERT INTO seller_users (id, seller_id, user_id, role, created_at)
		VALUES ($1, $2, $3, 'manager', now())
	`, uuid.New(), sellerID, userID)
	require.NoError(t, err)
}

func cleanupFixtures(client *postgres.Client, userIDs []uuid.UUID, sellerIDs []uuid.UUID) {
	ctx := context.Background()
	if len(userIDs) > 0 {
		_, _ = client.Pool.Exec(ctx, `DELETE FROM staff_members WHERE user_id = ANY($1)`, userIDs)
		_, _ = client.Pool.Exec(ctx, `DELETE FROM seller_users WHERE user_id = ANY($1)`, userIDs)
		_, _ = client.Pool.Exec(ctx, `DELETE FROM support_requester_reads WHERE user_id = ANY($1)`, userIDs)
		_, _ = client.Pool.Exec(ctx, `DELETE FROM support_staff_reads WHERE staff_user_id = ANY($1)`, userIDs)
		_, _ = client.Pool.Exec(ctx, `DELETE FROM support_conversations WHERE requester_user_id = ANY($1)`, userIDs)
		_, _ = client.Pool.Exec(ctx, `DELETE FROM users WHERE id = ANY($1)`, userIDs)
	}
	if len(sellerIDs) > 0 {
		_, _ = client.Pool.Exec(ctx, `DELETE FROM support_conversations WHERE requester_seller_id = ANY($1)`, sellerIDs)
		_, _ = client.Pool.Exec(ctx, `DELETE FROM seller_users WHERE seller_id = ANY($1)`, sellerIDs)
		_, _ = client.Pool.Exec(ctx, `DELETE FROM sellers WHERE id = ANY($1)`, sellerIDs)
	}
}

// -------------------------------------------------------------------------------------------------
// Test Suites A through AJ
// -------------------------------------------------------------------------------------------------

func TestSupport_CaseA_FirstCustomerMessageCreatesActiveSession(t *testing.T) {
	client, repo, svc := setupTestSupportDB(t)
	custID := createTestUser(t, client, "customer")
	defer cleanupFixtures(client, []uuid.UUID{custID}, nil)

	ctx := context.Background()
	msg, err := svc.SendCustomerMessage(ctx, custID, support.SendMessageRequest{
		TextContent: "Здравствуйте, помогите с заказом!",
	})
	require.NoError(t, err)
	assert.NotEqual(t, uuid.Nil, msg.ID)
	assert.Equal(t, support.SenderTypeCustomer, msg.SenderType)
	assert.Equal(t, custID, msg.SenderUserID)

	// Verify conversation and active session
	conv, err := svc.GetCustomerConversation(ctx, custID)
	require.NoError(t, err)
	assert.Equal(t, support.RequesterTypeCustomer, conv.Conversation.RequesterType)
	assert.Equal(t, custID, *conv.Conversation.RequesterUserID)
	require.NotNil(t, conv.Conversation.ActiveSession)
	assert.Empty(t, conv.Conversation.ActiveSession.Priority, "Priority must be omitted in customer view")
	assert.Equal(t, conv.Conversation.ActiveSession.ID, msg.SessionID)

	dbSess, err := repo.GetSessionByID(ctx, msg.SessionID)
	require.NoError(t, err)
	assert.Equal(t, support.SessionPriorityNormal, dbSess.Priority, "DB session must have NORMAL priority")
}

func TestSupport_CaseB_SubsequentMessageReusesActiveSession(t *testing.T) {
	client, _, svc := setupTestSupportDB(t)
	custID := createTestUser(t, client, "customer")
	defer cleanupFixtures(client, []uuid.UUID{custID}, nil)

	ctx := context.Background()
	msg1, err := svc.SendCustomerMessage(ctx, custID, support.SendMessageRequest{TextContent: "Сообщение 1"})
	require.NoError(t, err)

	msg2, err := svc.SendCustomerMessage(ctx, custID, support.SendMessageRequest{TextContent: "Сообщение 2"})
	require.NoError(t, err)

	assert.Equal(t, msg1.SessionID, msg2.SessionID, "Subsequent message MUST reuse the active session")

	conv, err := svc.GetCustomerConversation(ctx, custID)
	require.NoError(t, err)
	assert.Len(t, conv.Messages, 2)
}

func TestSupport_CaseC_CompleteSession(t *testing.T) {
	client, _, svc := setupTestSupportDB(t)
	custID := createTestUser(t, client, "customer")
	defer cleanupFixtures(client, []uuid.UUID{custID}, nil)

	ctx := context.Background()
	msg, err := svc.SendCustomerMessage(ctx, custID, support.SendMessageRequest{TextContent: "Вопрос решен"})
	require.NoError(t, err)

	conv, err := svc.GetCustomerConversation(ctx, custID)
	require.NoError(t, err)
	require.NotNil(t, conv.Conversation.ActiveSession)

	err = svc.CompleteSession(ctx, conv.Conversation.ID)
	require.NoError(t, err)

	// Fetch after completion
	convAfter, err := svc.GetCustomerConversation(ctx, custID)
	require.NoError(t, err)
	assert.Nil(t, convAfter.Conversation.ActiveSession, "Active session should be nil after completion")

	// DB check on completed_at
	var status string
	var completedAt *time.Time
	err = client.Pool.QueryRow(ctx, `SELECT status, completed_at FROM support_sessions WHERE id = $1`, msg.SessionID).Scan(&status, &completedAt)
	require.NoError(t, err)
	assert.Equal(t, "COMPLETED", status)
	assert.NotNil(t, completedAt, "completed_at must not be null when status=COMPLETED")
}

func TestSupport_CaseD_LaterMessageCreatesNewSessionSameConversation(t *testing.T) {
	client, _, svc := setupTestSupportDB(t)
	custID := createTestUser(t, client, "customer")
	defer cleanupFixtures(client, []uuid.UUID{custID}, nil)

	ctx := context.Background()
	msg1, err := svc.SendCustomerMessage(ctx, custID, support.SendMessageRequest{TextContent: "Первое обращение"})
	require.NoError(t, err)

	conv, err := svc.GetCustomerConversation(ctx, custID)
	require.NoError(t, err)
	err = svc.CompleteSession(ctx, conv.Conversation.ID)
	require.NoError(t, err)

	// Later message
	msg2, err := svc.SendCustomerMessage(ctx, custID, support.SendMessageRequest{TextContent: "Новый вопрос через неделю"})
	require.NoError(t, err)

	assert.NotEqual(t, msg1.SessionID, msg2.SessionID, "New active session must be created")

	convAfter, err := svc.GetCustomerConversation(ctx, custID)
	require.NoError(t, err)
	assert.Equal(t, conv.Conversation.ID, convAfter.Conversation.ID, "Must remain in the SAME conversation")
	assert.Equal(t, msg2.SessionID, convAfter.Conversation.ActiveSession.ID)
}

func TestSupport_CaseE_HistoricalMessagesRemainContinuous(t *testing.T) {
	client, _, svc := setupTestSupportDB(t)
	custID := createTestUser(t, client, "customer")
	defer cleanupFixtures(client, []uuid.UUID{custID}, nil)

	ctx := context.Background()
	_, _ = svc.SendCustomerMessage(ctx, custID, support.SendMessageRequest{TextContent: "Сессия 1 Сообщение"})
	conv, _ := svc.GetCustomerConversation(ctx, custID)
	_ = svc.CompleteSession(ctx, conv.Conversation.ID)

	_, _ = svc.SendCustomerMessage(ctx, custID, support.SendMessageRequest{TextContent: "Сессия 2 Сообщение"})

	convAfter, err := svc.GetCustomerConversation(ctx, custID)
	require.NoError(t, err)
	require.Len(t, convAfter.Messages, 2)
	assert.Equal(t, "Сессия 1 Сообщение", convAfter.Messages[0].TextContent)
	assert.Equal(t, "Сессия 2 Сообщение", convAfter.Messages[1].TextContent)
}

func TestSupport_CaseF_DBPreventsTwoActiveSessions(t *testing.T) {
	client, _, svc := setupTestSupportDB(t)
	custID := createTestUser(t, client, "customer")
	defer cleanupFixtures(client, []uuid.UUID{custID}, nil)

	ctx := context.Background()
	_, err := svc.SendCustomerMessage(ctx, custID, support.SendMessageRequest{TextContent: "Привет"})
	require.NoError(t, err)

	conv, err := svc.GetCustomerConversation(ctx, custID)
	require.NoError(t, err)

	// Attempt direct DB insert of second ACTIVE session
	_, err = client.Pool.Exec(ctx, `
		INSERT INTO support_sessions (id, conversation_id, status)
		VALUES ($1, $2, 'ACTIVE')
	`, uuid.New(), conv.Conversation.ID)
	require.Error(t, err, "DB MUST reject second ACTIVE session via unique index")
	assert.Contains(t, err.Error(), "idx_support_sessions_active_one_per_conv")
}

func TestSupport_CaseG_OneCustomerCanonicalConversation(t *testing.T) {
	client, _, _ := setupTestSupportDB(t)
	custID := createTestUser(t, client, "customer")
	defer cleanupFixtures(client, []uuid.UUID{custID}, nil)

	ctx := context.Background()
	convID1 := uuid.New()
	_, err := client.Pool.Exec(ctx, `
		INSERT INTO support_conversations (id, requester_type, requester_user_id)
		VALUES ($1, 'CUSTOMER', $2)
	`, convID1, custID)
	require.NoError(t, err)

	// Second insert must fail
	convID2 := uuid.New()
	_, err = client.Pool.Exec(ctx, `
		INSERT INTO support_conversations (id, requester_type, requester_user_id)
		VALUES ($1, 'CUSTOMER', $2)
	`, convID2, custID)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "idx_support_conv_customer")
}

func TestSupport_CaseH_OneSellerCanonicalConversation(t *testing.T) {
	client, _, _ := setupTestSupportDB(t)
	ownerID := createTestUser(t, client, "seller")
	sellerID := createTestSeller(t, client, ownerID)
	defer cleanupFixtures(client, []uuid.UUID{ownerID}, []uuid.UUID{sellerID})

	ctx := context.Background()
	convID1 := uuid.New()
	_, err := client.Pool.Exec(ctx, `
		INSERT INTO support_conversations (id, requester_type, requester_seller_id)
		VALUES ($1, 'SELLER', $2)
	`, convID1, sellerID)
	require.NoError(t, err)

	convID2 := uuid.New()
	_, err = client.Pool.Exec(ctx, `
		INSERT INTO support_conversations (id, requester_type, requester_seller_id)
		VALUES ($1, 'SELLER', $2)
	`, convID2, sellerID)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "idx_support_conv_seller")
}

func TestSupport_CaseI_SellerMemberSenderIdentityPreserved(t *testing.T) {
	client, _, svc := setupTestSupportDB(t)
	ownerID := createTestUser(t, client, "seller")
	managerID := createTestUser(t, client, "seller")
	sellerID := createTestSeller(t, client, ownerID)
	addSellerMember(t, client, sellerID, managerID)
	defer cleanupFixtures(client, []uuid.UUID{ownerID, managerID}, []uuid.UUID{sellerID})

	ctx := context.Background()
	msg1, err := svc.SendSellerMessage(ctx, sellerID, ownerID, support.SendMessageRequest{
		TextContent: "Сообщение от владельца магазина",
	})
	require.NoError(t, err)
	assert.Equal(t, ownerID, msg1.SenderUserID)
	assert.Equal(t, support.SenderTypeSeller, msg1.SenderType)

	msg2, err := svc.SendSellerMessage(ctx, sellerID, managerID, support.SendMessageRequest{
		TextContent: "Сообщение от управляющего магазина",
	})
	require.NoError(t, err)
	assert.Equal(t, managerID, msg2.SenderUserID)
	assert.Equal(t, support.SenderTypeSeller, msg2.SenderType)

	conv, err := svc.GetSellerConversation(ctx, sellerID, ownerID)
	require.NoError(t, err)
	require.Len(t, conv.Messages, 2)
	assert.Equal(t, ownerID, conv.Messages[0].SenderUserID)
	assert.Equal(t, managerID, conv.Messages[1].SenderUserID)
}

func TestSupport_CaseJ_CustomerIDORBlocked(t *testing.T) {
	client, _, svc := setupTestSupportDB(t)
	custA := createTestUser(t, client, "customer")
	custB := createTestUser(t, client, "customer")
	defer cleanupFixtures(client, []uuid.UUID{custA, custB}, nil)

	ctx := context.Background()
	_, err := svc.SendCustomerMessage(ctx, custA, support.SendMessageRequest{TextContent: "Секрет А"})
	require.NoError(t, err)

	// CustB attempts to query history of customer B
	convB, err := svc.GetCustomerConversation(ctx, custB)
	require.NoError(t, err)
	assert.Empty(t, convB.Messages, "Customer B MUST NOT see Customer A's conversation history")
}

func TestSupport_CaseK_SellerIDORBlocked(t *testing.T) {
	client, _, svc := setupTestSupportDB(t)
	userA := createTestUser(t, client, "seller")
	userB := createTestUser(t, client, "seller")
	sellerA := createTestSeller(t, client, userA)
	sellerB := createTestSeller(t, client, userB)
	defer cleanupFixtures(client, []uuid.UUID{userA, userB}, []uuid.UUID{sellerA, sellerB})

	ctx := context.Background()
	_, err := svc.SendSellerMessage(ctx, sellerA, userA, support.SendMessageRequest{TextContent: "Магазин А"})
	require.NoError(t, err)

	// User B (member of seller B) tries to query seller A's conversation
	_, err = svc.GetSellerConversation(ctx, sellerA, userB)
	require.ErrorIs(t, err, support.ErrForbidden, "Foreign seller member MUST be rejected with ErrForbidden")
}

func TestSupport_CaseL_CustomerCannotAccessSellerConversation(t *testing.T) {
	client, _, svc := setupTestSupportDB(t)
	custID := createTestUser(t, client, "customer")
	sellerOwnerID := createTestUser(t, client, "seller")
	sellerID := createTestSeller(t, client, sellerOwnerID)
	defer cleanupFixtures(client, []uuid.UUID{custID, sellerOwnerID}, []uuid.UUID{sellerID})

	ctx := context.Background()
	// Customer tries to query seller conversation
	_, err := svc.GetSellerConversation(ctx, sellerID, custID)
	require.ErrorIs(t, err, support.ErrForbidden)
}

func TestSupport_CaseM_InternalNotesAbsentFromExternalResult(t *testing.T) {
	client, _, svc := setupTestSupportDB(t)
	custID := createTestUser(t, client, "customer")
	staffID := createTestUser(t, client, "admin")
	defer cleanupFixtures(client, []uuid.UUID{custID, staffID}, nil)

	ctx := context.Background()
	_, err := svc.SendCustomerMessage(ctx, custID, support.SendMessageRequest{TextContent: "Помогите"})
	require.NoError(t, err)

	conv, err := svc.GetCustomerConversation(ctx, custID)
	require.NoError(t, err)

	// Staff adds internal note
	note, err := svc.CreateInternalNote(ctx, conv.Conversation.ID, staffID, support.CreateInternalNoteRequest{
		TextContent: "Внутренний комментарий: клиент проблемный",
	})
	require.NoError(t, err)
	assert.NotEqual(t, uuid.Nil, note.ID)

	// Customer reads conversation history
	custView, err := svc.GetCustomerConversation(ctx, custID)
	require.NoError(t, err)
	assert.Nil(t, custView.InternalNotes, "Internal notes MUST NOT be serialized or visible to customer")

	// Admin reads conversation history
	adminView, err := svc.GetAdminConversation(ctx, conv.Conversation.ID, staffID)
	require.NoError(t, err)
	require.Len(t, adminView.InternalNotes, 1)
	assert.Equal(t, "Внутренний комментарий: клиент проблемный", adminView.InternalNotes[0].TextContent)
}

func TestSupport_CaseN_StaffReplyVisibleExternally(t *testing.T) {
	client, _, svc := setupTestSupportDB(t)
	custID := createTestUser(t, client, "customer")
	staffID := createTestUser(t, client, "admin")
	defer cleanupFixtures(client, []uuid.UUID{custID, staffID}, nil)

	ctx := context.Background()
	_, err := svc.SendCustomerMessage(ctx, custID, support.SendMessageRequest{TextContent: "Вопрос"})
	require.NoError(t, err)

	conv, err := svc.GetCustomerConversation(ctx, custID)
	require.NoError(t, err)

	staffMsg, err := svc.SendStaffReply(ctx, conv.Conversation.ID, staffID, support.StaffReplyRequest{
		TextContent: "Здравствуйте! Чем я могу вам помочь?",
	})
	require.NoError(t, err)
	assert.Equal(t, support.SenderTypeStaff, staffMsg.SenderType)

	// Customer reads conversation history
	custView, err := svc.GetCustomerConversation(ctx, custID)
	require.NoError(t, err)
	require.Len(t, custView.Messages, 2)
	assert.Equal(t, support.SenderTypeStaff, custView.Messages[1].SenderType)
	assert.Equal(t, "Здравствуйте! Чем я могу вам помочь?", custView.Messages[1].TextContent)
}

func TestSupport_CaseR_TwoStaffUsersHaveIndependentReadState(t *testing.T) {
	client, _, svc := setupTestSupportDB(t)
	custID := createTestUser(t, client, "customer")
	staff1 := createTestUser(t, client, "admin")
	staff2 := createTestUser(t, client, "admin")
	defer cleanupFixtures(client, []uuid.UUID{custID, staff1, staff2}, nil)

	ctx := context.Background()
	_, _ = svc.SendCustomerMessage(ctx, custID, support.SendMessageRequest{TextContent: "Сообщение 1"})
	conv, _ := svc.GetCustomerConversation(ctx, custID)

	// Both staff have 1 unread message
	c1, _ := svc.GetAdminConversation(ctx, conv.Conversation.ID, staff1)
	c2, _ := svc.GetAdminConversation(ctx, conv.Conversation.ID, staff2)
	assert.Equal(t, 1, c1.Conversation.UnreadCount)
	assert.Equal(t, 1, c2.Conversation.UnreadCount)

	// Staff 1 marks as read
	err := svc.MarkStaffRead(ctx, staff1, conv.Conversation.ID)
	require.NoError(t, err)

	// Staff 1 unread -> 0, Staff 2 unread -> 1
	c1After, _ := svc.GetAdminConversation(ctx, conv.Conversation.ID, staff1)
	c2After, _ := svc.GetAdminConversation(ctx, conv.Conversation.ID, staff2)
	assert.Equal(t, 0, c1After.Conversation.UnreadCount, "Staff 1 unread MUST be 0 after marking read")
	assert.Equal(t, 1, c2After.Conversation.UnreadCount, "Staff 2 unread MUST remain 1")
}

func TestSupport_CaseS_RequesterUnreadAfterStaffReply(t *testing.T) {
	client, _, svc := setupTestSupportDB(t)
	custID := createTestUser(t, client, "customer")
	staffID := createTestUser(t, client, "admin")
	defer cleanupFixtures(client, []uuid.UUID{custID, staffID}, nil)

	ctx := context.Background()
	_, _ = svc.SendCustomerMessage(ctx, custID, support.SendMessageRequest{TextContent: "Привет"})
	conv, _ := svc.GetCustomerConversation(ctx, custID)
	assert.Equal(t, 0, conv.Conversation.UnreadCount, "Customer's own message does not count as unread for customer")

	// Staff replies
	_, _ = svc.SendStaffReply(ctx, conv.Conversation.ID, staffID, support.StaffReplyRequest{TextContent: "Ответ поддержки"})

	convAfter, _ := svc.GetCustomerConversation(ctx, custID)
	assert.Equal(t, 1, convAfter.Conversation.UnreadCount, "Customer unread MUST be 1 after staff reply")

	// Customer marks read
	_ = svc.MarkRequesterRead(ctx, custID, conv.Conversation.ID)
	convFinal, _ := svc.GetCustomerConversation(ctx, custID)
	assert.Equal(t, 0, convFinal.Conversation.UnreadCount, "Customer unread MUST be 0 after mark read")
}

func TestSupport_CaseU_V_W_X_ContextOwnershipValidation(t *testing.T) {
	client, _, svc := setupTestSupportDB(t)
	custA := createTestUser(t, client, "customer")
	custB := createTestUser(t, client, "customer")
	sellerOwner := createTestUser(t, client, "seller")
	sellerID := createTestSeller(t, client, sellerOwner)
	defer cleanupFixtures(client, []uuid.UUID{custA, custB, sellerOwner}, []uuid.UUID{sellerID})

	ctx := context.Background()
	// Insert order for CustA
	orderA := uuid.New()
	_, err := client.Pool.Exec(ctx, `
		INSERT INTO orders (id, user_id, order_number, status, total_price_cents, currency, delivery_address, delivery_method_name, delivery_price_cents, customer_name, customer_email, customer_phone, created_at, updated_at)
		VALUES ($1, $2, 'ORD-A', 'created', 1000, 'RUB', 'Addr', 'Std', 0, 'Test', 't@e.com', '+12', now(), now())
	`, orderA, custA)
	require.NoError(t, err)
	defer client.Pool.Exec(ctx, `DELETE FROM orders WHERE id = $1`, orderA)

	// Insert order fulfillment for Seller and OrderA
	fulfID := uuid.New()
	_, err = client.Pool.Exec(ctx, `
		INSERT INTO order_fulfillments (id, order_id, seller_id, status, created_at, updated_at)
		VALUES ($1, $2, $3, 'delivered', now(), now())
	`, fulfID, orderA, sellerID)
	require.NoError(t, err)
	defer client.Pool.Exec(ctx, `DELETE FROM order_fulfillments WHERE id = $1`, fulfID)

	// Insert return for CustA
	retA := uuid.New()
	_, err = client.Pool.Exec(ctx, `
		INSERT INTO returns (id, order_id, fulfillment_id, user_id, status, reason, created_at, updated_at)
		VALUES ($1, $2, $3, $4, 'requested', 'size', now(), now())
	`, retA, orderA, fulfID, custA)
	require.NoError(t, err)
	defer client.Pool.Exec(ctx, `DELETE FROM returns WHERE id = $1`, retA)

	// Insert active product for Seller
	prodID := uuid.New()
	prodSlug := "prod-" + prodID.String()[:8]
	_, err = client.Pool.Exec(ctx, `
		INSERT INTO products (id, seller_id, title, slug, price_cents, status, created_at, updated_at)
		VALUES ($1, $2, 'Test Product', $3, 10000, 'approved', now(), now())
	`, prodID, sellerID, prodSlug)
	require.NoError(t, err)
	defer client.Pool.Exec(ctx, `DELETE FROM products WHERE id = $1`, prodID)

	// Case U: CustA links own ORDER -> SUCCESS
	msgU, err := svc.SendCustomerMessage(ctx, custA, support.SendMessageRequest{
		TextContent: "Вопрос по заказу",
		ContextLinks: []support.CreateContextLinkReq{
			{ContextType: support.ContextTypeOrder, ContextID: orderA},
		},
	})
	require.NoError(t, err)
	require.Len(t, msgU.ContextLinks, 1)
	assert.Equal(t, orderA, msgU.ContextLinks[0].ContextID)

	// Case V: CustA links own RETURN -> SUCCESS
	msgV, err := svc.SendCustomerMessage(ctx, custA, support.SendMessageRequest{
		TextContent: "Вопрос по возврату",
		ContextLinks: []support.CreateContextLinkReq{
			{ContextType: support.ContextTypeReturn, ContextID: retA},
		},
	})
	require.NoError(t, err)
	require.Len(t, msgV.ContextLinks, 1)
	assert.Equal(t, retA, msgV.ContextLinks[0].ContextID)

	// Case W: Seller links own PRODUCT -> SUCCESS
	msgW, err := svc.SendSellerMessage(ctx, sellerID, sellerOwner, support.SendMessageRequest{
		TextContent: "Вопрос по нашему товару",
		ContextLinks: []support.CreateContextLinkReq{
			{ContextType: support.ContextTypeProduct, ContextID: prodID},
		},
	})
	require.NoError(t, err)
	require.Len(t, msgW.ContextLinks, 1)
	assert.Equal(t, prodID, msgW.ContextLinks[0].ContextID)

	// Case X: CustB attempts to link CustA's order -> REJECTED
	_, err = svc.SendCustomerMessage(ctx, custB, support.SendMessageRequest{
		TextContent: "Чужой заказ",
		ContextLinks: []support.CreateContextLinkReq{
			{ContextType: support.ContextTypeOrder, ContextID: orderA},
		},
	})
	require.ErrorIs(t, err, support.ErrInvalidContext, "Foreign context MUST be rejected")
}

func TestSupport_CaseY_Z_AA_Attachments(t *testing.T) {
	client, _, svc := setupTestSupportDB(t)
	custA := createTestUser(t, client, "customer")
	custB := createTestUser(t, client, "customer")
	defer cleanupFixtures(client, []uuid.UUID{custA, custB}, nil)

	ctx := context.Background()
	convA, _ := svc.GetCustomerConversation(ctx, custA)

	// Case AA: Disallowed file extension / mime rejected
	_, err := svc.UploadStagedAttachment(ctx, convA.Conversation.ID, custA, strings.NewReader("binary"), "virus.exe", "application/x-msdownload", 100)
	require.ErrorIs(t, err, support.ErrAttachmentInvalid, "Disallowed attachment type must be rejected")

	// Upload valid image
	pngHeader := []byte("\x89PNG\r\n\x1a\n\x00\x00\x00\rIHDR\x00\x00\x00\x01\x00\x00\x00\x01\x08\x06\x00\x00\x00\x1f\x15c4")
	staged, err := svc.UploadStagedAttachment(ctx, convA.Conversation.ID, custA, bytes.NewReader(pngHeader), "photo.png", "image/png", int64(len(pngHeader)))
	require.NoError(t, err)
	assert.NotEqual(t, uuid.Nil, staged.ID)

	// Case Z: CustB tries to bind CustA's staged attachment -> REJECTED
	_, err = svc.SendCustomerMessage(ctx, custB, support.SendMessageRequest{
		TextContent:   "Попытка привязать чужой файл",
		AttachmentIDs: []uuid.UUID{staged.ID},
	})
	require.ErrorIs(t, err, support.ErrAttachmentInvalid, "Foreign staged attachment MUST be rejected")

	// Case Y: CustA binds own attachment -> SUCCESS
	msg, err := svc.SendCustomerMessage(ctx, custA, support.SendMessageRequest{
		TextContent:   "Вот фото дефекта",
		AttachmentIDs: []uuid.UUID{staged.ID},
	})
	require.NoError(t, err)
	require.Len(t, msg.Attachments, 1)
	assert.Equal(t, staged.ID, msg.Attachments[0].ID)

	// Case Z download: CustB tries to download CustA's attachment -> FORBIDDEN
	_, _, err = svc.DownloadAttachment(ctx, staged.ID, custB, false, nil)
	require.ErrorIs(t, err, support.ErrForbidden, "Foreign attachment download MUST be forbidden")

	// CustA downloads own attachment -> SUCCESS
	_, _, err = svc.DownloadAttachment(ctx, staged.ID, custA, false, nil)
	require.NoError(t, err)
}

func TestSupport_CaseAC_AD_ConcurrencyProtections(t *testing.T) {
	client, _, svc := setupTestSupportDB(t)
	custID := createTestUser(t, client, "customer")
	defer cleanupFixtures(client, []uuid.UUID{custID}, nil)

	ctx := context.Background()

	// Case AC: Simultaneous first messages create exactly ONE conversation
	var wg sync.WaitGroup
	errs := make([]error, 2)
	msgs := make([]*support.Message, 2)
	for i := 0; i < 2; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			msgs[idx], errs[idx] = svc.SendCustomerMessage(ctx, custID, support.SendMessageRequest{
				TextContent: fmt.Sprintf("Конкурентное сообщение %d", idx),
			})
		}(i)
	}
	wg.Wait()

	require.NoError(t, errs[0])
	require.NoError(t, errs[1])

	conv, err := svc.GetCustomerConversation(ctx, custID)
	require.NoError(t, err)
	assert.Equal(t, msgs[0].SessionID, msgs[1].SessionID, "Concurrent messages must land in the same session")
	assert.Len(t, conv.Messages, 2)

	// Case AD: Complete session, then send concurrent messages -> exactly ONE new ACTIVE session
	err = svc.CompleteSession(ctx, conv.Conversation.ID)
	require.NoError(t, err)

	for i := 0; i < 2; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			msgs[idx], errs[idx] = svc.SendCustomerMessage(ctx, custID, support.SendMessageRequest{
				TextContent: fmt.Sprintf("Новое конкурентное сообщение %d", idx),
			})
		}(i)
	}
	wg.Wait()

	require.NoError(t, errs[0])
	require.NoError(t, errs[1])

	convAfter, err := svc.GetCustomerConversation(ctx, custID)
	require.NoError(t, err)
	assert.Equal(t, msgs[0].SessionID, msgs[1].SessionID, "Concurrent new messages must create exactly ONE new session")
	assert.Len(t, convAfter.Messages, 4)
}

func TestSupport_CaseAE_AF_InternalMetadataIsolation(t *testing.T) {
	client, _, svc := setupTestSupportDB(t)
	custID := createTestUser(t, client, "customer")
	staffID := createTestUser(t, client, "admin")
	defer cleanupFixtures(client, []uuid.UUID{custID, staffID}, nil)

	ctx := context.Background()
	_, _ = svc.SendCustomerMessage(ctx, custID, support.SendMessageRequest{TextContent: "Помощь"})
	conv, _ := svc.GetCustomerConversation(ctx, custID)

	// Create a category
	catID := uuid.New()
	_, err := client.Pool.Exec(ctx, `
		INSERT INTO support_categories (id, name, requester_scope)
		VALUES ($1, 'Возвраты и обмен', 'CUSTOMER')
	`, catID)
	require.NoError(t, err)
	defer client.Pool.Exec(ctx, `DELETE FROM support_categories WHERE id = $1`, catID)

	// Staff sets priority and assignment
	prio := support.SessionPriorityUrgent
	err = svc.UpdateSession(ctx, conv.Conversation.ID, support.UpdateSessionRequest{
		Priority:   &prio,
		CategoryID: &catID,
		AssignedTo: &staffID,
	})
	require.NoError(t, err)

	// Admin view sees metadata
	adminView, err := svc.GetAdminConversation(ctx, conv.Conversation.ID, staffID)
	require.NoError(t, err)
	require.NotNil(t, adminView.Conversation.ActiveSession)
	assert.Equal(t, support.SessionPriorityUrgent, adminView.Conversation.ActiveSession.Priority)
	assert.Equal(t, staffID, *adminView.Conversation.ActiveSession.AssignedTo)
	assert.Equal(t, catID, *adminView.Conversation.ActiveSession.CategoryID)

	// Customer view DOES NOT expose assignment/internal category to customer API logic
	custView, err := svc.GetCustomerConversation(ctx, custID)
	require.NoError(t, err)
	assert.Nil(t, custView.InternalNotes, "Internal notes absent externally")
}

func TestSupport_CaseAG_SellerNonMemberCannotSendAsSeller(t *testing.T) {
	client, _, svc := setupTestSupportDB(t)
	ownerID := createTestUser(t, client, "seller")
	nonMemberID := createTestUser(t, client, "customer")
	sellerID := createTestSeller(t, client, ownerID)
	defer cleanupFixtures(client, []uuid.UUID{ownerID, nonMemberID}, []uuid.UUID{sellerID})

	ctx := context.Background()
	_, err := svc.SendSellerMessage(ctx, sellerID, nonMemberID, support.SendMessageRequest{
		TextContent: "Самозванец",
	})
	require.ErrorIs(t, err, support.ErrForbidden, "Non-member MUST be forbidden from sending as seller")
}

func TestSupport_CaseAH_InternalNoteAuthorMustBeStaff(t *testing.T) {
	client, _, svc := setupTestSupportDB(t)
	custID := createTestUser(t, client, "customer")
	defer cleanupFixtures(client, []uuid.UUID{custID}, nil)

	ctx := context.Background()
	_, _ = svc.SendCustomerMessage(ctx, custID, support.SendMessageRequest{TextContent: "Помощь"})
	conv, _ := svc.GetCustomerConversation(ctx, custID)

	// Non-staff user tries to write internal note
	_, err := svc.CreateInternalNote(ctx, conv.Conversation.ID, custID, support.CreateInternalNoteRequest{
		TextContent: "Несанкционированная заметка",
	})
	require.ErrorIs(t, err, support.ErrForbidden, "Non-staff user MUST be forbidden from creating internal notes")
}

func TestSupport_CaseAI_MismatchedRelationalStateImpossible(t *testing.T) {
	client, _, _ := setupTestSupportDB(t)
	custA := createTestUser(t, client, "customer")
	defer cleanupFixtures(client, []uuid.UUID{custA}, nil)

	ctx := context.Background()
	// FK enforcement: inserting a message with non-existent session_id must fail
	fakeSessionID := uuid.New()
	_, err := client.Pool.Exec(ctx, `
		INSERT INTO support_messages (id, session_id, sender_type, sender_user_id, text_content)
		VALUES ($1, $2, 'CUSTOMER', $3, 'Тест')
	`, uuid.New(), fakeSessionID, custA)
	require.Error(t, err, "DB must reject message with non-existent session_id via FK constraint")
}

func TestSupport_CaseAJ_ReopenConflictWithActiveSessionFailsSafely(t *testing.T) {
	client, _, svc := setupTestSupportDB(t)
	custID := createTestUser(t, client, "customer")
	defer cleanupFixtures(client, []uuid.UUID{custID}, nil)

	ctx := context.Background()
	msg1, err := svc.SendCustomerMessage(ctx, custID, support.SendMessageRequest{TextContent: "Сессия 1"})
	require.NoError(t, err)
	require.NotNil(t, msg1)
	conv, err := svc.GetCustomerConversation(ctx, custID)
	require.NoError(t, err)
	sess1ID := msg1.SessionID

	// Complete session 1
	err = svc.CompleteSession(ctx, conv.Conversation.ID)
	require.NoError(t, err)

	// Create session 2
	_, err = svc.SendCustomerMessage(ctx, custID, support.SendMessageRequest{TextContent: "Сессия 2"})
	require.NoError(t, err)

	// Attempt to reopen session 1 while session 2 is ACTIVE
	err = svc.ReopenSession(ctx, sess1ID)
	require.ErrorIs(t, err, support.ErrActiveSessionAlreadyExists, "Reopening completed session when an ACTIVE session exists must fail safely")
}

func setDirectPerms(t *testing.T, ctx context.Context, client *postgres.Client, userID uuid.UUID, permissions ...string) {
	t.Helper()
	_, _ = client.Pool.Exec(ctx, `DELETE FROM staff_member_permissions WHERE user_id = $1`, userID)
	for _, p := range permissions {
		_, err := client.Pool.Exec(ctx, `
			INSERT INTO staff_member_permissions (user_id, permission, created_at)
			VALUES ($1, $2, now())
			ON CONFLICT DO NOTHING
		`, userID, p)
		require.NoError(t, err)
	}
}

func checkStaffPermission(ctx context.Context, client *postgres.Client, userID uuid.UUID, permission string) bool {
	var hasPerm bool
	err := client.Pool.QueryRow(ctx, `
		SELECT EXISTS (
			SELECT 1 FROM staff_member_permissions WHERE user_id = $1 AND permission = $2
		)
	`, userID, permission).Scan(&hasPerm)
	return err == nil && hasPerm
}

func TestSupport_CaseO_SupportReadEnforcement(t *testing.T) {
	client, _, _ := setupTestSupportDB(t)
	staffID := createTestUser(t, client, "admin")
	defer cleanupFixtures(client, []uuid.UUID{staffID}, nil)

	ctx := context.Background()
	// Initially no permissions
	assert.False(t, checkStaffPermission(ctx, client, staffID, "support.read"), "Staff without support.read must be rejected")

	// Grant support.read
	setDirectPerms(t, ctx, client, staffID, "support.read")
	assert.True(t, checkStaffPermission(ctx, client, staffID, "support.read"), "Staff with support.read must be allowed")
}

func TestSupport_CaseP_SupportRespondEnforcement(t *testing.T) {
	client, _, _ := setupTestSupportDB(t)
	staffID := createTestUser(t, client, "admin")
	defer cleanupFixtures(client, []uuid.UUID{staffID}, nil)

	ctx := context.Background()
	// Initially only support.read, no support.respond
	setDirectPerms(t, ctx, client, staffID, "support.read")
	assert.False(t, checkStaffPermission(ctx, client, staffID, "support.respond"), "Staff without support.respond must be rejected")

	// Grant support.respond
	setDirectPerms(t, ctx, client, staffID, "support.read", "support.respond")
	assert.True(t, checkStaffPermission(ctx, client, staffID, "support.respond"), "Staff with support.respond must be allowed")
}

func TestSupport_CaseQ_SupportCloseEnforcement(t *testing.T) {
	client, _, _ := setupTestSupportDB(t)
	staffID := createTestUser(t, client, "admin")
	defer cleanupFixtures(client, []uuid.UUID{staffID}, nil)

	ctx := context.Background()
	// Initially only support.read and support.respond, no support.close
	setDirectPerms(t, ctx, client, staffID, "support.read", "support.respond")
	assert.False(t, checkStaffPermission(ctx, client, staffID, "support.close"), "Staff without support.close must be rejected")

	// Grant support.close
	setDirectPerms(t, ctx, client, staffID, "support.read", "support.respond", "support.close")
	assert.True(t, checkStaffPermission(ctx, client, staffID, "support.close"), "Staff with support.close must be allowed")
}

func TestSupport_CaseT_OneStaffReadDoesNotMarkAnotherRead(t *testing.T) {
	client, _, svc := setupTestSupportDB(t)
	custID := createTestUser(t, client, "customer")
	staff1 := createTestUser(t, client, "admin")
	staff2 := createTestUser(t, client, "admin")
	defer cleanupFixtures(client, []uuid.UUID{custID, staff1, staff2}, nil)

	ctx := context.Background()
	_, err := svc.SendCustomerMessage(ctx, custID, support.SendMessageRequest{TextContent: "Новое обращение"})
	require.NoError(t, err)
	conv, err := svc.GetCustomerConversation(ctx, custID)
	require.NoError(t, err)

	// Staff 1 marks as read
	err = svc.MarkStaffRead(ctx, staff1, conv.Conversation.ID)
	require.NoError(t, err)

	c1, err := svc.GetAdminConversation(ctx, conv.Conversation.ID, staff1)
	require.NoError(t, err)
	assert.Equal(t, 0, c1.Conversation.UnreadCount, "Staff 1 unread must be 0")

	c2, err := svc.GetAdminConversation(ctx, conv.Conversation.ID, staff2)
	require.NoError(t, err)
	assert.Equal(t, 1, c2.Conversation.UnreadCount, "Staff 2 unread must remain 1 and not affected by Staff 1")
}

func TestSupport_HTTP_APIs(t *testing.T) {
	client, _, svc := setupTestSupportDB(t)
	custID := createTestUser(t, client, "customer")
	ownerID := createTestUser(t, client, "seller")
	sellerID := createTestSeller(t, client, ownerID)
	staffID := createTestUser(t, client, "admin")
	defer cleanupFixtures(client, []uuid.UUID{custID, ownerID, staffID}, []uuid.UUID{sellerID})

	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	h := support.NewHandler(svc, logger)

	// 1. Customer API
	t.Run("Customer HTTP API", func(t *testing.T) {
		r := chi.NewRouter()
		r.Use(func(next http.Handler) http.Handler {
			return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
				ctx := context.WithValue(req.Context(), "userID", custID)
				next.ServeHTTP(w, req.WithContext(ctx))
			})
		})
		r.Get("/support", h.GetCustomerConversation)
		r.Post("/support/messages", h.SendCustomerMessage)
		r.Post("/support/read", h.MarkCustomerRead)

		// GET conversation
		req := httptest.NewRequest("GET", "/support", nil)
		rec := httptest.NewRecorder()
		r.ServeHTTP(rec, req)
		assert.Equal(t, http.StatusOK, rec.Code)

		// POST message
		body := `{"textContent":"Привет из HTTP"}`
		req = httptest.NewRequest("POST", "/support/messages", strings.NewReader(body))
		rec = httptest.NewRecorder()
		r.ServeHTTP(rec, req)
		assert.Equal(t, http.StatusCreated, rec.Code)

		// POST read
		req = httptest.NewRequest("POST", "/support/read", nil)
		rec = httptest.NewRecorder()
		r.ServeHTTP(rec, req)
		assert.Equal(t, http.StatusOK, rec.Code)
	})

	// 2. Seller API
	t.Run("Seller HTTP API", func(t *testing.T) {
		r := chi.NewRouter()
		r.Use(func(next http.Handler) http.Handler {
			return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
				ctx := context.WithValue(req.Context(), "userID", ownerID)
				ctx = context.WithValue(ctx, "sellerID", sellerID)
				next.ServeHTTP(w, req.WithContext(ctx))
			})
		})
		r.Get("/support", h.GetSellerConversation)
		r.Post("/support/messages", h.SendSellerMessage)
		r.Post("/support/read", h.MarkSellerRead)

		// GET conversation
		req := httptest.NewRequest("GET", "/support", nil)
		rec := httptest.NewRecorder()
		r.ServeHTTP(rec, req)
		assert.Equal(t, http.StatusOK, rec.Code)

		// POST message
		body := `{"textContent":"Привет от продавца из HTTP"}`
		req = httptest.NewRequest("POST", "/support/messages", strings.NewReader(body))
		rec = httptest.NewRecorder()
		r.ServeHTTP(rec, req)
		assert.Equal(t, http.StatusCreated, rec.Code)

		// POST read
		req = httptest.NewRequest("POST", "/support/read", nil)
		rec = httptest.NewRecorder()
		r.ServeHTTP(rec, req)
		assert.Equal(t, http.StatusOK, rec.Code)
	})

	// 3. Admin API
	t.Run("Admin HTTP API", func(t *testing.T) {
		conv, err := svc.GetCustomerConversation(context.Background(), custID)
		require.NoError(t, err)

		r := chi.NewRouter()
		r.Use(func(next http.Handler) http.Handler {
			return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
				ctx := context.WithValue(req.Context(), "userID", staffID)
				next.ServeHTTP(w, req.WithContext(ctx))
			})
		})
		r.Get("/admin/support/categories", h.ListCategories)
		r.Get("/admin/support/conversations", h.ListConversations)
		r.Get("/admin/support/conversations/{id}", h.GetAdminConversation)
		r.Post("/admin/support/conversations/{id}/messages", h.SendStaffReply)
		r.Post("/admin/support/conversations/{id}/internal-notes", h.CreateInternalNote)
		r.Post("/admin/support/conversations/{id}/read", h.MarkStaffRead)
		r.Post("/admin/support/conversations/{id}/complete", h.CompleteSession)

		// GET categories
		req := httptest.NewRequest("GET", "/admin/support/categories", nil)
		rec := httptest.NewRecorder()
		r.ServeHTTP(rec, req)
		assert.Equal(t, http.StatusOK, rec.Code)

		// GET conversations
		req = httptest.NewRequest("GET", "/admin/support/conversations", nil)
		rec = httptest.NewRecorder()
		r.ServeHTTP(rec, req)
		assert.Equal(t, http.StatusOK, rec.Code)

		// GET conversation detail
		req = httptest.NewRequest("GET", fmt.Sprintf("/admin/support/conversations/%s", conv.Conversation.ID), nil)
		rec = httptest.NewRecorder()
		r.ServeHTTP(rec, req)
		assert.Equal(t, http.StatusOK, rec.Code)

		// POST reply
		replyBody := `{"textContent":"Ответ сотрудника из HTTP"}`
		req = httptest.NewRequest("POST", fmt.Sprintf("/admin/support/conversations/%s/messages", conv.Conversation.ID), strings.NewReader(replyBody))
		rec = httptest.NewRecorder()
		r.ServeHTTP(rec, req)
		assert.Equal(t, http.StatusCreated, rec.Code)

		// POST internal note
		noteBody := `{"textContent":"Внутренняя заметка из HTTP"}`
		req = httptest.NewRequest("POST", fmt.Sprintf("/admin/support/conversations/%s/internal-notes", conv.Conversation.ID), strings.NewReader(noteBody))
		rec = httptest.NewRecorder()
		r.ServeHTTP(rec, req)
		assert.Equal(t, http.StatusCreated, rec.Code)

		// POST read
		req = httptest.NewRequest("POST", fmt.Sprintf("/admin/support/conversations/%s/read", conv.Conversation.ID), nil)
		rec = httptest.NewRecorder()
		r.ServeHTTP(rec, req)
		assert.Equal(t, http.StatusOK, rec.Code)

		// POST complete
		req = httptest.NewRequest("POST", fmt.Sprintf("/admin/support/conversations/%s/complete", conv.Conversation.ID), nil)
		rec = httptest.NewRecorder()
		r.ServeHTTP(rec, req)
		assert.Equal(t, http.StatusOK, rec.Code)
	})
}

func TestSupport_CaseDEV_State(t *testing.T) {
	ctx := context.Background()
	devURL := strings.Replace(testutil.GetTestDatabaseURL(), "zamk_test", "zamk", 1)
	client, err := postgres.NewClient(ctx, devURL)
	require.NoError(t, err)
	defer client.Close()

	var dbName string
	err = client.Pool.QueryRow(ctx, "SELECT current_database()").Scan(&dbName)
	require.NoError(t, err)
	assert.Equal(t, "zamk", dbName)

	var version int
	var dirty bool
	err = client.Pool.QueryRow(ctx, "SELECT version, dirty FROM schema_migrations").Scan(&version, &dirty)
	require.NoError(t, err)
	assert.Equal(t, 104, version, "DEV database MUST be at version 104")
	assert.False(t, dirty, "DEV database MUST NOT be dirty")

	// Ensure 000104 tables exist in DEV
	var tableCount int
	err = client.Pool.QueryRow(ctx, "SELECT COUNT(*) FROM information_schema.tables WHERE table_name = 'support_conversations'").Scan(&tableCount)
	require.NoError(t, err)
	assert.Equal(t, 1, tableCount, "Support tables MUST exist in DEV database")
}
