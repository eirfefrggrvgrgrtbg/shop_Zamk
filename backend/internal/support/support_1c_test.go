package support_test

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/eirfefrggrvgrgrtbg/shop-zamk/backend/internal/support"
)

func TestSupport_1C_ReadModelEnrichmentAndSearch(t *testing.T) {
	client, _, svc := setupTestSupportDB(t)
	custID := createTestUser(t, client, "customer")
	sellerUserID := createTestUser(t, client, "seller")
	sellerID := createTestSeller(t, client, sellerUserID)
	staffID := createTestUser(t, client, "admin")
	defer cleanupFixtures(client, []uuid.UUID{custID, staffID, sellerUserID}, []uuid.UUID{sellerID})

	ctx := context.Background()

	// 1. Customer initiates dialogue
	msgCust, err := svc.SendCustomerMessage(ctx, custID, support.SendMessageRequest{
		TextContent: "Здравствуйте, вопрос по доставке",
	})
	require.NoError(t, err)
	require.NotNil(t, msgCust)

	// 2. Seller initiates dialogue
	msgSeller, err := svc.SendSellerMessage(ctx, sellerID, sellerUserID, support.SendMessageRequest{
		TextContent: "Вопрос по выплатам и балансу",
	})
	require.NoError(t, err)
	require.NotNil(t, msgSeller)

	// 3. List without filter or search: both conversations returned with enriched requester summaries
	allConvs, err := svc.ListConversations(ctx, "", "", staffID)
	require.NoError(t, err)
	assert.GreaterOrEqual(t, len(allConvs), 2)

	var custConv, selConv *support.Conversation
	for i := range allConvs {
		if allConvs[i].RequesterType == support.RequesterTypeCustomer && allConvs[i].RequesterUserID != nil && *allConvs[i].RequesterUserID == custID {
			custConv = &allConvs[i]
		}
		if allConvs[i].RequesterType == support.RequesterTypeSeller && allConvs[i].RequesterSellerID != nil && *allConvs[i].RequesterSellerID == sellerID {
			selConv = &allConvs[i]
		}
	}
	require.NotNil(t, custConv, "Customer conversation must be found")
	require.NotNil(t, selConv, "Seller conversation must be found")

	assert.NotEmpty(t, custConv.RequesterName, "Customer name should be enriched")
	assert.NotEmpty(t, custConv.RequesterEmail, "Customer email should be enriched")
	assert.Equal(t, "Здравствуйте, вопрос по доставке", custConv.LatestMessageText)

	assert.NotEmpty(t, selConv.RequesterStoreName, "Seller store name should be enriched")
	assert.Equal(t, "Вопрос по выплатам и балансу", selConv.LatestMessageText)

	// 4. Search by customer email
	searchByEmail, err := svc.ListConversations(ctx, "", custConv.RequesterEmail, staffID)
	require.NoError(t, err)
	assert.GreaterOrEqual(t, len(searchByEmail), 1)
	assert.Equal(t, custConv.ID, searchByEmail[0].ID)

	// 5. Search by seller store name
	searchByStore, err := svc.ListConversations(ctx, "", selConv.RequesterStoreName, staffID)
	require.NoError(t, err)
	assert.GreaterOrEqual(t, len(searchByStore), 1)
	assert.Equal(t, selConv.ID, searchByStore[0].ID)

	// 6. Search by message snippet
	searchByMsg, err := svc.ListConversations(ctx, "", "доставке", staffID)
	require.NoError(t, err)
	assert.GreaterOrEqual(t, len(searchByMsg), 1)
	assert.Equal(t, custConv.ID, searchByMsg[0].ID)

	// 7. GetAdminConversation enriches requester summary
	adminDetail, err := svc.GetAdminConversation(ctx, custConv.ID, staffID)
	require.NoError(t, err)
	assert.Equal(t, custConv.RequesterName, adminDetail.Conversation.RequesterName)
	assert.Equal(t, custConv.RequesterEmail, adminDetail.Conversation.RequesterEmail)
}
