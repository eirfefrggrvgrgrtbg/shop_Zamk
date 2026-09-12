package returns_test

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/eirfefrggrvgrgrtbg/shop-zamk/backend/internal/app"
	"github.com/eirfefrggrvgrgrtbg/shop-zamk/backend/internal/auth"
	"github.com/eirfefrggrvgrgrtbg/shop-zamk/backend/internal/config"
	"github.com/eirfefrggrvgrgrtbg/shop-zamk/backend/internal/platform/redis"
	"github.com/eirfefrggrvgrgrtbg/shop-zamk/backend/internal/returns"
	"github.com/eirfefrggrvgrgrtbg/shop-zamk/backend/internal/testutil"
	goredis "github.com/redis/go-redis/v9"
)

type testReturnFixture struct {
	orderID      uuid.UUID
	orderItemID  uuid.UUID
	returnID     uuid.UUID
	returnItemID uuid.UUID
	allocationID uuid.UUID
}

func setupDamagedReturnFixture(t *testing.T, fix *m51Fixture, refundActorID uuid.UUID) testReturnFixture {
	t.Helper()
	ctx := context.Background()

	tOrd := fix.createDeliveredOrder(t, time.Now().Add(-1*time.Hour), 1)
	createSellerEarning(t, fix, tOrd.orderID, tOrd.orderItemID, fix.sellerAID, 700000)
	createSucceededPayment(t, fix, tOrd.orderID, 1000)
	evIDs := fix.createStagedEvidence(t, fix.userID, 2)

	resp, err := fix.svc.CreateReturn(ctx, fix.userID, tOrd.orderID, returns.CreateReturnRequest{
		Reason:  "defective",
		Comment: strPtr("item arrived damaged"),
		Items:   []returns.CreateReturnItemRequest{{OrderItemID: tOrd.orderItemID, Quantity: 1, EvidenceIDs: evIDs}},
	})
	require.NoError(t, err)
	retID := resp[0].Return.ID
	retItemID := resp[0].Items[0].ID

	require.NoError(t, fix.svc.UpdateReturnStatus(ctx, fix.userID, retID, returns.UpdateReturnStatusRequest{
		Status: "approved",
	}))

	fix.createArrivedReturnShipment(t, retID)
	require.NoError(t, fix.svc.StartReceiving(ctx, retID))
	require.NoError(t, fix.svc.InspectLegacyItem(ctx, retID, retItemID, returns.UpdateLegacyItemInspectionRequest{
		DamagedQuantity: 1,
	}))
	require.NoError(t, fix.svc.FinalizeReceiving(ctx, retID))

	// Refund return so financial deduction exists
	ref, err := fix.svc.CreateRefund(ctx, refundActorID, retID, returns.CreateRefundRequest{})
	require.NoError(t, err)
	require.NoError(t, fix.svc.ProcessRefundSuccess(ctx, ref.ID, time.Now()))

	allocs, err := fix.svc.GetReturnResponsibilityAllocations(ctx, retID)
	require.NoError(t, err)
	require.Len(t, allocs, 1)

	return testReturnFixture{
		orderID:      tOrd.orderID,
		orderItemID:  tOrd.orderItemID,
		returnID:     retID,
		returnItemID: retItemID,
		allocationID: allocs[0].ID,
	}
}

func TestSA5_3A_ResponsibilityRouter(t *testing.T) {
	fix := setupM51Fixture(t)
	ctx := context.Background()

	// Section 8: Database safety guard (also verified inside setupM51Fixture at line 58)
	testutil.AssertTestDatabase(t, fix.client.Pool)

	cfg := &config.Config{}
	cfg.JWT.AccessTokenSecret = "test-secret"
	cfg.JWT.AccessTokenTTLMinutes = 60
	cfg.RateLimit.Enabled = false
	cfg.Worker.ReturnWindowDays = 14

	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelError}))
	dummyRedis := &redis.Client{Client: goredis.NewClient(&goredis.Options{})}

	router, cancel := app.BuildRouter(context.Background(), cfg, fix.client, dummyRedis, logger)
	t.Cleanup(cancel)

	tokenService := auth.NewTokenService(cfg.JWT.AccessTokenSecret, cfg.JWT.RefreshTokenSecret, cfg.JWT.AccessTokenTTLMinutes)

	adminWithPermID := uuid.New()
	adminNoPermID := uuid.New()
	roleWithPermID := uuid.New()
	roleNoPermID := uuid.New()

	email1 := fmt.Sprintf("a1_%s@test.com", uuid.New().String())
	email2 := fmt.Sprintf("a2_%s@test.com", uuid.New().String())

	_, err := fix.client.Pool.Exec(ctx, `INSERT INTO staff_roles (id, code, name, is_system) VALUES 
		($1, $3, 'With Perm', false), 
		($2, $4, 'No Perm', false)`,
		roleWithPermID, roleNoPermID, "custom_with_perm_"+roleWithPermID.String(), "custom_no_perm_"+roleNoPermID.String())
	require.NoError(t, err)

	_, err = fix.client.Pool.Exec(ctx, `INSERT INTO staff_role_permissions (role_id, permission) VALUES ($1, 'returns.update_status')`, roleWithPermID)
	require.NoError(t, err)

	_, err = fix.client.Pool.Exec(ctx, `INSERT INTO users (id, name, email, password_hash, role, status) VALUES 
		($1, 'Admin1', $3, 'hash', 'admin', 'active'),
		($2, 'Admin2', $4, 'hash', 'admin', 'active')`, adminWithPermID, adminNoPermID, email1, email2)
	require.NoError(t, err)

	_, err = fix.client.Pool.Exec(ctx, `INSERT INTO staff_members (user_id, staff_role_id, status) VALUES 
		($1, $2, 'active'),
		($3, $4, 'active')`, adminWithPermID, roleWithPermID, adminNoPermID, roleNoPermID)
	require.NoError(t, err)

	adminToken, _ := tokenService.GenerateAccessToken(adminWithPermID, email1, "admin")
	noPermToken, _ := tokenService.GenerateAccessToken(adminNoPermID, email2, "admin")

	sendReq := func(token string, returnID, allocID uuid.UUID, body interface{}) *httptest.ResponseRecorder {
		b, _ := json.Marshal(body)
		req := httptest.NewRequest("PATCH", "/api/admin/returns/"+returnID.String()+"/responsibility/"+allocID.String(), bytes.NewReader(b))
		req.Header.Set("Content-Type", "application/json")
		if token != "" {
			req.Header.Set("Authorization", "Bearer "+token)
		}
		rr := httptest.NewRecorder()
		router.ServeHTTP(rr, req)
		return rr
	}

	// -------------------------------------------------------------------------
	// A. PENDING DAMAGED -> ZAMK
	// -------------------------------------------------------------------------
	t.Run("A_PendingDamaged_To_ZAMK", func(t *testing.T) {
		tf := setupDamagedReturnFixture(t, fix, adminWithPermID)

		// Prove allocation is pending and has physical disposition damaged from receiving
		var dispBefore *string
		var statusBefore string
		err := fix.client.Pool.QueryRow(ctx, "SELECT status, legacy_disposition FROM return_responsibility_allocations WHERE id = $1", tf.allocationID).Scan(&statusBefore, &dispBefore)
		require.NoError(t, err)
		assert.Equal(t, "pending", statusBefore)
		require.NotNil(t, dispBefore)
		assert.Equal(t, "damaged", *dispBefore)

		// Call Admin responsibility endpoint WITHOUT physical disposition and WITHOUT status
		note := "damaged during staging"
		reqBody := returns.UpdateReturnResponsibilityRequest{
			ResponsibleParty: "zamk",
			ReasonCode:       "zamk_warehouse_damage",
			InternalNote:     &note,
		}

		rr := sendReq(adminToken, tf.returnID, tf.allocationID, reqBody)
		assert.Equal(t, http.StatusOK, rr.Code)

		var resAlloc returns.ReturnResponsibilityAllocation
		require.NoError(t, json.Unmarshal(rr.Body.Bytes(), &resAlloc))
		assert.Equal(t, "resolved", resAlloc.Status)
		require.NotNil(t, resAlloc.ResponsibleParty)
		assert.Equal(t, "zamk", *resAlloc.ResponsibleParty)
		require.NotNil(t, resAlloc.ReasonCode)
		assert.Equal(t, "zamk_warehouse_damage", *resAlloc.ReasonCode)
		require.NotNil(t, resAlloc.LegacyDisposition)
		assert.Equal(t, "damaged", *resAlloc.LegacyDisposition)

		// In DB: existing physical disposition remains damaged
		var dispAfter *string
		var statusAfter string
		err = fix.client.Pool.QueryRow(ctx, "SELECT status, legacy_disposition FROM return_responsibility_allocations WHERE id = $1", tf.allocationID).Scan(&statusAfter, &dispAfter)
		require.NoError(t, err)
		assert.Equal(t, "resolved", statusAfter)
		require.NotNil(t, dispAfter)
		assert.Equal(t, "damaged", *dispAfter)

		// Exactly ONE canonical return_compensation ledger effect
		comps := getCompensationEntries(t, fix, tf.orderItemID)
		require.Len(t, comps, 1)
		assert.Equal(t, int64(700000), comps[0])
		assert.Equal(t, int64(700000), getNetCompensation(t, fix, tf.orderItemID))

		// ---------------------------------------------------------------------
		// B. REPEAT SAME ZAMK DECISION (idempotency verification on same alloc)
		// ---------------------------------------------------------------------
		t.Run("B_RepeatSameDecision_Idempotent", func(t *testing.T) {
			rrRepeat := sendReq(adminToken, tf.returnID, tf.allocationID, reqBody)
			assert.Equal(t, http.StatusOK, rrRepeat.Code)

			var repeatAlloc returns.ReturnResponsibilityAllocation
			require.NoError(t, json.Unmarshal(rrRepeat.Body.Bytes(), &repeatAlloc))
			assert.Equal(t, "resolved", repeatAlloc.Status)
			assert.Equal(t, "zamk", *repeatAlloc.ResponsibleParty)

			// Compensation count in DB remains exactly 1!
			compsRepeat := getCompensationEntries(t, fix, tf.orderItemID)
			require.Len(t, compsRepeat, 1)
			assert.Equal(t, int64(700000), compsRepeat[0])
		})
	})

	// -------------------------------------------------------------------------
	// C. PENDING DAMAGED -> CARRIER
	// -------------------------------------------------------------------------
	t.Run("C_PendingDamaged_To_CARRIER", func(t *testing.T) {
		tf := setupDamagedReturnFixture(t, fix, adminWithPermID)

		reqBody := returns.UpdateReturnResponsibilityRequest{
			ResponsibleParty: "carrier",
			ReasonCode:       "carrier_damage",
		}

		rr := sendReq(adminToken, tf.returnID, tf.allocationID, reqBody)
		assert.Equal(t, http.StatusOK, rr.Code)

		var resAlloc returns.ReturnResponsibilityAllocation
		require.NoError(t, json.Unmarshal(rr.Body.Bytes(), &resAlloc))
		assert.Equal(t, "resolved", resAlloc.Status)
		assert.Equal(t, "carrier", *resAlloc.ResponsibleParty)
		assert.Equal(t, "carrier_damage", *resAlloc.ReasonCode)
		require.NotNil(t, resAlloc.LegacyDisposition)
		assert.Equal(t, "damaged", *resAlloc.LegacyDisposition)

		// Carrier damage produces seller compensation
		comps := getCompensationEntries(t, fix, tf.orderItemID)
		require.Len(t, comps, 1)
		assert.Equal(t, int64(700000), comps[0])
	})

	// -------------------------------------------------------------------------
	// D. PENDING DAMAGED -> SELLER
	// -------------------------------------------------------------------------
	t.Run("D_PendingDamaged_To_SELLER", func(t *testing.T) {
		tf := setupDamagedReturnFixture(t, fix, adminWithPermID)

		reqBody := returns.UpdateReturnResponsibilityRequest{
			ResponsibleParty: "seller",
			ReasonCode:       "seller_product_defect",
		}

		rr := sendReq(adminToken, tf.returnID, tf.allocationID, reqBody)
		assert.Equal(t, http.StatusOK, rr.Code)

		var resAlloc returns.ReturnResponsibilityAllocation
		require.NoError(t, json.Unmarshal(rr.Body.Bytes(), &resAlloc))
		assert.Equal(t, "resolved", resAlloc.Status)
		assert.Equal(t, "seller", *resAlloc.ResponsibleParty)
		assert.Equal(t, "seller_product_defect", *resAlloc.ReasonCode)

		// Seller responsibility produces NO seller compensation
		comps := getCompensationEntries(t, fix, tf.orderItemID)
		assert.Len(t, comps, 0)
	})

	// -------------------------------------------------------------------------
	// E. REAL CROSS-RETURN MISMATCH
	// -------------------------------------------------------------------------
	t.Run("E_CrossReturnMismatch_Rejection", func(t *testing.T) {
		tfA := setupDamagedReturnFixture(t, fix, adminWithPermID)
		tfB := setupDamagedReturnFixture(t, fix, adminWithPermID)

		reqBody := returns.UpdateReturnResponsibilityRequest{
			ResponsibleParty: "zamk",
			ReasonCode:       "zamk_warehouse_damage",
		}

		// Call Return B endpoint with Allocation A ID
		rr := sendReq(adminToken, tfB.returnID, tfA.allocationID, reqBody)
		assert.Equal(t, http.StatusNotFound, rr.Code)

		// Prove Allocation A is completely unchanged
		var statusA string
		var partyA *string
		err := fix.client.Pool.QueryRow(ctx, "SELECT status, responsible_party FROM return_responsibility_allocations WHERE id = $1", tfA.allocationID).Scan(&statusA, &partyA)
		require.NoError(t, err)
		assert.Equal(t, "pending", statusA)
		assert.Nil(t, partyA)
	})

	// -------------------------------------------------------------------------
	// F. INVALID PARTY / REASON COMBINATIONS & MISSING FIELDS
	// -------------------------------------------------------------------------
	t.Run("F_InvalidPartyReasonCombinations", func(t *testing.T) {
		tf := setupDamagedReturnFixture(t, fix, adminWithPermID)

		// 1. Missing responsibleParty -> 400
		rrEmptyParty := sendReq(adminToken, tf.returnID, tf.allocationID, map[string]any{
			"reasonCode": "zamk_warehouse_damage",
		})
		assert.Equal(t, http.StatusBadRequest, rrEmptyParty.Code)

		// 2. Missing reasonCode -> 400
		rrEmptyReason := sendReq(adminToken, tf.returnID, tf.allocationID, map[string]any{
			"responsibleParty": "zamk",
		})
		assert.Equal(t, http.StatusBadRequest, rrEmptyReason.Code)

		// 3. Seller + zamk_warehouse_damage -> 400
		rr1 := sendReq(adminToken, tf.returnID, tf.allocationID, returns.UpdateReturnResponsibilityRequest{
			ResponsibleParty: "seller",
			ReasonCode:       "zamk_warehouse_damage",
		})
		assert.Equal(t, http.StatusBadRequest, rr1.Code)

		// 4. Zamk + seller_product_defect -> 400
		rr2 := sendReq(adminToken, tf.returnID, tf.allocationID, returns.UpdateReturnResponsibilityRequest{
			ResponsibleParty: "zamk",
			ReasonCode:       "seller_product_defect",
		})
		assert.Equal(t, http.StatusBadRequest, rr2.Code)

		// 5. Unknown responsibleParty -> 400
		rr3 := sendReq(adminToken, tf.returnID, tf.allocationID, returns.UpdateReturnResponsibilityRequest{
			ResponsibleParty: "alien",
			ReasonCode:       "zamk_warehouse_damage",
		})
		assert.Equal(t, http.StatusBadRequest, rr3.Code)
	})

	// -------------------------------------------------------------------------
	// G. AUTHORIZATION
	// -------------------------------------------------------------------------
	t.Run("G_AuthorizationGates", func(t *testing.T) {
		tf := setupDamagedReturnFixture(t, fix, adminWithPermID)
		reqBody := returns.UpdateReturnResponsibilityRequest{
			ResponsibleParty: "zamk",
			ReasonCode:       "zamk_warehouse_damage",
		}

		// 1. No auth -> 401
		rrNoAuth := sendReq("", tf.returnID, tf.allocationID, reqBody)
		assert.Equal(t, http.StatusUnauthorized, rrNoAuth.Code)

		// 2. Authenticated Admin employee without returns.update_status -> 403
		rrNoPerm := sendReq(noPermToken, tf.returnID, tf.allocationID, reqBody)
		assert.Equal(t, http.StatusForbidden, rrNoPerm.Code)

		// 3. Correct capability -> allowed 200
		rrPerm := sendReq(adminToken, tf.returnID, tf.allocationID, reqBody)
		assert.Equal(t, http.StatusOK, rrPerm.Code)
	})

	// -------------------------------------------------------------------------
	// H. PHYSICAL TRUTH CANNOT BE CHANGED
	// -------------------------------------------------------------------------
	t.Run("H_PhysicalTruthCannotBeChanged", func(t *testing.T) {
		tf := setupDamagedReturnFixture(t, fix, adminWithPermID)

		// Check stock movements before resolution
		var movCountBefore int
		err := fix.client.Pool.QueryRow(ctx, "SELECT COUNT(*) FROM stock_movements WHERE reference_id = $1", tf.returnID).Scan(&movCountBefore)
		require.NoError(t, err)

		// Request contains NO physical fields
		reqBody := returns.UpdateReturnResponsibilityRequest{
			ResponsibleParty: "zamk",
			ReasonCode:       "zamk_warehouse_damage",
		}

		rr := sendReq(adminToken, tf.returnID, tf.allocationID, reqBody)
		assert.Equal(t, http.StatusOK, rr.Code)

		// Receiving damaged_quantity remains 1 on return_items
		var damagedQty int
		var acceptedQty int
		var rejectedQty int
		err = fix.client.Pool.QueryRow(ctx, "SELECT damaged_quantity, accepted_quantity, rejected_quantity FROM return_items WHERE id = $1", tf.returnItemID).Scan(&damagedQty, &acceptedQty, &rejectedQty)
		require.NoError(t, err)
		assert.Equal(t, 1, damagedQty)
		assert.Equal(t, 0, acceptedQty)
		assert.Equal(t, 0, rejectedQty)

		// Allocation physical disposition remains canonical damaged
		var disp string
		err = fix.client.Pool.QueryRow(ctx, "SELECT legacy_disposition FROM return_responsibility_allocations WHERE id = $1", tf.allocationID).Scan(&disp)
		require.NoError(t, err)
		assert.Equal(t, "damaged", disp)

		// No sellable / restock movements created
		var movCountAfter int
		err = fix.client.Pool.QueryRow(ctx, "SELECT COUNT(*) FROM stock_movements WHERE reference_id = $1", tf.returnID).Scan(&movCountAfter)
		require.NoError(t, err)
		assert.Equal(t, movCountBefore, movCountAfter)
	})

	// -------------------------------------------------------------------------
	// I. CORRECTION SEMANTICS (Isolated Scenario)
	// -------------------------------------------------------------------------
	t.Run("I_CorrectionSemantics_SellerToZamkToSeller", func(t *testing.T) {
		tf := setupDamagedReturnFixture(t, fix, adminWithPermID)

		// Step 1: Resolve as Seller
		rr1 := sendReq(adminToken, tf.returnID, tf.allocationID, returns.UpdateReturnResponsibilityRequest{
			ResponsibleParty: "seller",
			ReasonCode:       "seller_product_defect",
		})
		assert.Equal(t, http.StatusOK, rr1.Code)
		assert.Len(t, getCompensationEntries(t, fix, tf.orderItemID), 0)

		// Step 2: Correct from Seller to ZAMK
		rr2 := sendReq(adminToken, tf.returnID, tf.allocationID, returns.UpdateReturnResponsibilityRequest{
			ResponsibleParty: "zamk",
			ReasonCode:       "zamk_warehouse_damage",
		})
		assert.Equal(t, http.StatusOK, rr2.Code)

		comps2 := getCompensationEntries(t, fix, tf.orderItemID)
		require.Len(t, comps2, 1)
		assert.Equal(t, int64(700000), comps2[0])
		assert.Equal(t, int64(700000), getNetCompensation(t, fix, tf.orderItemID))

		// Step 3: Correct back from ZAMK to Seller
		rr3 := sendReq(adminToken, tf.returnID, tf.allocationID, returns.UpdateReturnResponsibilityRequest{
			ResponsibleParty: "seller",
			ReasonCode:       "seller_product_defect",
		})
		assert.Equal(t, http.StatusOK, rr3.Code)

		comps3 := getCompensationEntries(t, fix, tf.orderItemID)
		require.Len(t, comps3, 2)
		assert.Equal(t, int64(700000), comps3[0], "Historical positive compensation row remains immutable")
		assert.Equal(t, int64(-700000), comps3[1], "Negative return_compensation_correction automatically appended")
		assert.Equal(t, int64(0), getNetCompensation(t, fix, tf.orderItemID))
	})
}
