package inventory_test

import (
	"context"
	"testing"
	"time"

	"github.com/eirfefrggrvgrgrtbg/shop-zamk/backend/internal/inventory"
	"github.com/eirfefrggrvgrgrtbg/shop-zamk/backend/internal/platform/postgres"
	"github.com/eirfefrggrvgrgrtbg/shop-zamk/backend/internal/testutil"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)


func TestClassifyResolutionFact_MatrixAK(t *testing.T) {
	variantID := uuid.New()
	orderID := uuid.New()
	fulfillmentID := uuid.New()
	supplyID := uuid.New()
	allocID := uuid.New()
	now := time.Now()

	wh := "warehouse"
	exp := "expected"
	ship := "shipped"
	dam := "damaged"

	ordNum := "ORD-1001"
	ordPaid := "paid"
	ordDeliv := "delivered"
	ordCanc := "cancelled"
	fulAssem := "assembling"
	fulDeliv := "delivered"
	supNum := "SUP-555"
	relReason := "cancelled"

	unexFound := "unexpected_found"
	expFound := "expected_found"
	wrongVar := "wrong_variant"
	unknownCode := "unknown_code"
	dup := "duplicate"

	// Case A: missing_free
	t.Run("Case A: missing_free", func(t *testing.T) {
		fact := inventory.RawResolutionFact{
			UnitID:         uuid.New(),
			UnitCode:       mustGenerateUnitCode(),
			VariantID:      variantID,
			ProductTitle:   "Пальто",
			SnapshotStatus: &wh,
			CurrentStatus:  wh,
			Classification: nil,
		}
		c := inventory.ClassifyResolutionFact(fact)
		require.NotNil(t, c)
		require.Equal(t, inventory.CaseTypeMissingFree, c.CaseType)
		require.Equal(t, inventory.SeverityWarning, c.Severity)
		require.Equal(t, "Единица не найдена", c.Title)
		require.Contains(t, c.Explanation, "Ожидаемая единица товара не найдена на складе")
		require.Len(t, c.AllowedActions, 2)
		require.Equal(t, inventory.ActionIDRecount, c.AllowedActions[0].ID)
		require.True(t, c.AllowedActions[0].Enabled)
		require.Equal(t, "Перепроверить ZMU", c.AllowedActions[0].Label)
		require.Equal(t, "/warehouse/free-scan?unitCode="+fact.UnitCode, c.AllowedActions[0].Route)
		require.Equal(t, inventory.ActionIDConfirmMissing, c.AllowedActions[1].ID)
		require.True(t, c.AllowedActions[1].Enabled, "confirm_missing must be enabled in P2.2B")
		require.Empty(t, c.AllowedActions[1].BlockedReason, "confirm_missing must not have a blocked reason in P2.2B")
	})

	// Case B: missing_live_allocated
	t.Run("Case B: missing_live_allocated", func(t *testing.T) {
		fact := inventory.RawResolutionFact{
			UnitID:            uuid.New(),
			UnitCode:          mustGenerateUnitCode(),
			VariantID:         variantID,
			ProductTitle:      "Пальто",
			SnapshotStatus:    &wh,
			CurrentStatus:     wh,
			Classification:    nil,
			AllocationID:      &allocID,
			OrderID:           &orderID,
			OrderNumber:       &ordNum,
			OrderStatus:       &ordPaid,
			FulfillmentID:     &fulfillmentID,
			FulfillmentStatus: &fulAssem,
		}
		c := inventory.ClassifyResolutionFact(fact)
		require.NotNil(t, c)
		require.Equal(t, inventory.CaseTypeMissingLiveAllocated, c.CaseType)
		require.Equal(t, inventory.SeverityHigh, c.Severity)
		require.Equal(t, "Не найдена — назначена заказу", c.Title)
		require.Contains(t, c.Explanation, "ORD-1001")
		require.Equal(t, "/orders/"+orderID.String(), c.AllowedActions[0].Route)
		require.True(t, c.AllowedActions[0].Enabled)
	})

	// Case C: missing_picked_not_shipped
	t.Run("Case C: missing_picked_not_shipped", func(t *testing.T) {
		fact := inventory.RawResolutionFact{
			UnitID:             uuid.New(),
			UnitCode:           mustGenerateUnitCode(),
			VariantID:          variantID,
			ProductTitle:       "Пальто",
			SnapshotStatus:     &wh,
			CurrentStatus:      wh,
			Classification:     nil,
			AllocationID:       &allocID,
			AllocationPickedAt: &now,
			OrderID:            &orderID,
			OrderNumber:        &ordNum,
			OrderStatus:        &ordPaid,
			FulfillmentID:      &fulfillmentID,
			FulfillmentStatus:  &fulAssem,
		}
		c := inventory.ClassifyResolutionFact(fact)
		require.NotNil(t, c)
		require.Equal(t, inventory.CaseTypeMissingPickedNotShipped, c.CaseType)
		require.Equal(t, inventory.SeverityCritical, c.Severity)
		require.Equal(t, "Не найдена — уже собрана в заказ", c.Title)
		require.Contains(t, c.Explanation, "была отобрана")
		// Has picking handoff route
		var hasPickingHandoff bool
		for _, a := range c.AllowedActions {
			if a.ID == inventory.ActionIDInspectAllocation {
				hasPickingHandoff = true
				require.Equal(t, "/fulfillment/picking/"+fulfillmentID.String(), a.Route)
				require.True(t, a.Enabled)
			}
		}
		require.True(t, hasPickingHandoff, "Expected picking handoff action")
	})

	// Case D: expected_found
	t.Run("Case D: expected_found", func(t *testing.T) {
		code := mustGenerateUnitCode()
		fact := inventory.RawResolutionFact{
			UnitID:             uuid.New(),
			UnitCode:           code,
			VariantID:          variantID,
			ProductTitle:       "Пальто",
			CurrentStatus:      exp,
			Classification:     &unexFound,
			OriginSupplyID:     &supplyID,
			OriginSupplyNumber: &supNum,
		}
		c := inventory.ClassifyResolutionFact(fact)
		require.NotNil(t, c)
		require.Equal(t, inventory.CaseTypeExpectedFound, c.CaseType)
		require.Equal(t, inventory.SeverityInfo, c.Severity)
		require.Equal(t, "Приёмка единицы не завершена", c.Title)
		require.Equal(t, "/warehouse/free-scan?unitCode="+code, c.AllowedActions[0].Route)
		require.True(t, c.AllowedActions[0].Enabled)
	})

	// Case E: shipped_found
	t.Run("Case E: shipped_found", func(t *testing.T) {
		fact := inventory.RawResolutionFact{
			UnitID:            uuid.New(),
			UnitCode:          mustGenerateUnitCode(),
			VariantID:         variantID,
			ProductTitle:      "Пальто",
			CurrentStatus:     ship,
			Classification:    &unexFound,
			OrderID:           &orderID,
			OrderNumber:       &ordNum,
			OrderStatus:       &ordDeliv,
			FulfillmentID:     &fulfillmentID,
			FulfillmentStatus: &fulDeliv,
		}
		c := inventory.ClassifyResolutionFact(fact)
		require.NotNil(t, c)
		require.Equal(t, inventory.CaseTypeShippedFound, c.CaseType)
		require.Equal(t, inventory.SeverityCritical, c.Severity)
		require.Equal(t, "Найдена, хотя числится отгруженной", c.Title)
		require.NotNil(t, c.HistoricalContext)
		require.Equal(t, ordNum, c.HistoricalContext.OrderNumber)
		require.Equal(t, ordDeliv, c.HistoricalContext.OrderStatus)
		// Investigate action is blocked with reason
		var hasInvestigate bool
		for _, a := range c.AllowedActions {
			if a.ID == inventory.ActionIDInvestigateShippedFound {
				hasInvestigate = true
				require.False(t, a.Enabled)
				require.Equal(t, inventory.ActionSafetyBlocked, a.SafetyLevel)
				require.Equal(t, "Требуется ручная проверка отгрузки", a.Label)
				require.Equal(t, "Автоматическое исправление недоступно.", a.BlockedReason)
			}
		}
		require.True(t, hasInvestigate)
	})

	// Case F: damaged_found
	t.Run("Case F: damaged_found", func(t *testing.T) {
		fact := inventory.RawResolutionFact{
			UnitID:         uuid.New(),
			UnitCode:       mustGenerateUnitCode(),
			VariantID:      variantID,
			ProductTitle:   "Пальто",
			CurrentStatus:  dam,
			Classification: &unexFound,
		}
		c := inventory.ClassifyResolutionFact(fact)
		require.NotNil(t, c)
		require.Equal(t, inventory.CaseTypeDamagedFound, c.CaseType)
		require.Equal(t, inventory.SeverityWarning, c.Severity)
		require.Equal(t, "Найдена бракованная единица", c.Title)
		require.Contains(t, c.Explanation, "браком")
	})

	// Case G: stale_allocation (missing unit with delivered/cancelled order)
	t.Run("Case G: stale_allocation on missing unit", func(t *testing.T) {
		fact := inventory.RawResolutionFact{
			UnitID:            uuid.New(),
			UnitCode:          mustGenerateUnitCode(),
			VariantID:         variantID,
			ProductTitle:      "Пальто",
			SnapshotStatus:    &wh,
			CurrentStatus:     wh,
			Classification:    nil,
			AllocationID:      &allocID,
			OrderID:           &orderID,
			OrderNumber:       &ordNum,
			OrderStatus:       &ordDeliv,
			FulfillmentID:     &fulfillmentID,
			FulfillmentStatus: &fulDeliv,
		}
		c := inventory.ClassifyResolutionFact(fact)
		require.NotNil(t, c)
		require.Equal(t, inventory.CaseTypeStaleAllocation, c.CaseType)
		require.Equal(t, inventory.SeverityHigh, c.Severity)
		require.Equal(t, "Не найдена — старое назначение", c.Title)
		// close_stale_allocation is now enabled in P2.2B
		var hasCloseStale bool
		for _, a := range c.AllowedActions {
			if a.ID == inventory.ActionIDCloseStaleAllocation {
				hasCloseStale = true
				require.True(t, a.Enabled, "close_stale_allocation must be enabled in P2.2B")
				require.Empty(t, a.BlockedReason, "close_stale_allocation must not have a blocked reason in P2.2B")
			}
		}
		require.True(t, hasCloseStale)
	})

	// Case H: changed_during_count (Precedence over missing!)
	t.Run("Case H: changed_during_count takes precedence over missing", func(t *testing.T) {
		// Unit was warehouse at snapshot, but became shipped while count was open, not scanned.
		// MUST NOT become missing_free!
		fact := inventory.RawResolutionFact{
			UnitID:         uuid.New(),
			UnitCode:       mustGenerateUnitCode(),
			VariantID:      variantID,
			ProductTitle:   "Пальто",
			SnapshotStatus: &wh,
			CurrentStatus:  ship,
			Classification: nil,
		}
		c := inventory.ClassifyResolutionFact(fact)
		require.NotNil(t, c)
		require.Equal(t, inventory.CaseTypeChangedDuringCount, c.CaseType)
		require.Equal(t, inventory.SeverityWarning, c.Severity)
		require.Equal(t, "Состояние изменилось во время проверки", c.Title)
		require.Contains(t, c.Explanation, "На складе")
		require.Contains(t, c.Explanation, "Отгружен")
	})

	// Case I: wrong_variant excluded
	t.Run("Case I: wrong_variant excluded", func(t *testing.T) {
		fact := inventory.RawResolutionFact{
			UnitID:         uuid.New(),
			UnitCode:       mustGenerateUnitCode(),
			VariantID:      variantID,
			CurrentStatus:  wh,
			Classification: &wrongVar,
		}
		c := inventory.ClassifyResolutionFact(fact)
		require.Nil(t, c, "wrong_variant must be excluded from resolution cases")
	})

	// Case J: unknown_code excluded
	t.Run("Case J: unknown_code excluded", func(t *testing.T) {
		fact := inventory.RawResolutionFact{
			UnitID:         uuid.New(),
			UnitCode:       mustGenerateUnitCode(),
			VariantID:      variantID,
			CurrentStatus:  wh,
			Classification: &unknownCode,
		}
		c := inventory.ClassifyResolutionFact(fact)
		require.Nil(t, c, "unknown_code must be excluded from resolution cases")
	})

	// Case K: duplicate excluded
	t.Run("Case K: duplicate excluded", func(t *testing.T) {
		fact := inventory.RawResolutionFact{
			UnitID:         uuid.New(),
			UnitCode:       mustGenerateUnitCode(),
			VariantID:      variantID,
			CurrentStatus:  wh,
			Classification: &dup,
		}
		c := inventory.ClassifyResolutionFact(fact)
		require.Nil(t, c, "duplicate must be excluded from resolution cases")
	})

	// Clean expected_found with no status change excluded
	t.Run("Clean expected_found excluded", func(t *testing.T) {
		fact := inventory.RawResolutionFact{
			UnitID:         uuid.New(),
			UnitCode:       mustGenerateUnitCode(),
			VariantID:      variantID,
			SnapshotStatus: &wh,
			CurrentStatus:  wh,
			Classification: &expFound,
		}
		c := inventory.ClassifyResolutionFact(fact)
		require.Nil(t, c, "Clean expected_found must not generate a discrepancy case")
	})

	_ = ordCanc
	_ = relReason
}

func TestReconciliationResolutionPlan_DB(t *testing.T) {
	ctx, repo, variantID, adminID, sellerID := setupReconciliationEnv(t)

	// 1. Nonexistent session ID -> returns ErrReconciliationNotFound
	_, err := repo.GetReconciliationResolutionPlan(ctx, uuid.New())
	require.ErrorIs(t, err, inventory.ErrReconciliationNotFound)

	// 2. Set up supply and items
	supplyID := uuid.New()
	_, err = testDB.Exec(ctx, "INSERT INTO seller_supplies (id, supply_number, seller_id, status, handoff_method, created_at, updated_at) VALUES ($1, $3, $2, 'completed', 'courier', now(), now())", supplyID, sellerID, "SUP-"+uuid.New().String()[:8])
	require.NoError(t, err)

	supplyItemID := uuid.New()
	_, err = testDB.Exec(ctx, "INSERT INTO seller_supply_items (id, supply_id, variant_id, expected_quantity, accepted_quantity, damaged_quantity, missing_quantity, extra_quantity, created_at, updated_at) VALUES ($1, $2, $3, 10, 0, 0, 0, 0, now(), now())", supplyItemID, supplyID, variantID)
	require.NoError(t, err)

	codeMissingFree := mustGenerateUnitCode()
	codeMissingLive := mustGenerateUnitCode()
	codeUnexpectedShipped := mustGenerateUnitCode()
	codeUnexpectedFree := mustGenerateUnitCode()

	unitMissingFree := uuid.New()
	unitMissingLive := uuid.New()
	unitUnexpectedShipped := uuid.New()
	unitUnexpectedFree := uuid.New()

	_, err = testDB.Exec(ctx, "INSERT INTO inventory_units (id, product_variant_id, origin_supply_id, origin_supply_item_id, unit_code, status, unit_index, created_at, updated_at) VALUES ($1, $2, $3, $4, $5, 'warehouse', 1, now(), now())", unitMissingFree, variantID, supplyID, supplyItemID, codeMissingFree)
	require.NoError(t, err)
	_, err = testDB.Exec(ctx, "INSERT INTO inventory_units (id, product_variant_id, origin_supply_id, origin_supply_item_id, unit_code, status, unit_index, created_at, updated_at) VALUES ($1, $2, $3, $4, $5, 'warehouse', 2, now(), now())", unitMissingLive, variantID, supplyID, supplyItemID, codeMissingLive)
	require.NoError(t, err)
	_, err = testDB.Exec(ctx, "INSERT INTO inventory_units (id, product_variant_id, origin_supply_id, origin_supply_item_id, unit_code, status, unit_index, created_at, updated_at) VALUES ($1, $2, $3, $4, $5, 'shipped', 3, now(), now())", unitUnexpectedShipped, variantID, supplyID, supplyItemID, codeUnexpectedShipped)
	require.NoError(t, err)
	_, err = testDB.Exec(ctx, "INSERT INTO inventory_units (id, product_variant_id, origin_supply_id, origin_supply_item_id, unit_code, status, unit_index, created_at, updated_at) VALUES ($1, $2, $3, $4, $5, 'warehouse', 4, now(), now())", unitUnexpectedFree, variantID, supplyID, supplyItemID, codeUnexpectedFree)
	require.NoError(t, err)

	// User and live order
	userID := uuid.New()
	_, err = testDB.Exec(ctx, "INSERT INTO users (id, email, name, password_hash, role) VALUES ($1, $2, 'cust', 'hash', 'customer')", userID, uuid.New().String()+"@test.com")
	require.NoError(t, err)

	orderID := uuid.New()
	orderItemID := uuid.New()
	orderNum := "ORD-" + uuid.New().String()[:8]
	_, err = testDB.Exec(ctx, "INSERT INTO orders (id, order_number, user_id, status, customer_name, customer_phone, customer_email, delivery_address, created_at, updated_at) VALUES ($1, $3, $2, 'assembling', 'cust', '+123', 'a@b.com', 'addr', now(), now())", orderID, userID, orderNum)
	require.NoError(t, err)
	fulfillmentID := uuid.New()
	_, err = testDB.Exec(ctx, "INSERT INTO order_fulfillments (id, order_id, seller_id, status, created_at, updated_at) VALUES ($1, $2, $3, 'assembling', now(), now())", fulfillmentID, orderID, sellerID)
	require.NoError(t, err)

	var pid uuid.UUID
	err = testDB.QueryRow(ctx, "SELECT product_id FROM product_variants WHERE id = $1", variantID).Scan(&pid)
	require.NoError(t, err)

	_, err = testDB.Exec(ctx, "INSERT INTO order_items (id, order_id, product_id, product_variant_id, seller_id, title, product_slug, price_cents, quantity, subtotal_price_cents, order_fulfillment_id, created_at) VALUES ($1, $2, $3, $4, $5, 'title', 'slug', 100, 1, 100, $6, now())", orderItemID, orderID, pid, variantID, sellerID, fulfillmentID)
	require.NoError(t, err)
	_, err = testDB.Exec(ctx, "INSERT INTO order_item_allocations (id, order_item_id, inventory_unit_id, created_at) VALUES ($1, $2, $3, now())", uuid.New(), orderItemID, unitMissingLive)
	require.NoError(t, err)

	sessionID := uuid.New()
	require.NoError(t, repo.StartReconciliationSession(ctx, sessionID, variantID, adminID))

	_, err = testDB.Exec(ctx, "INSERT INTO inventory_reconciliation_scans (id, session_id, inventory_unit_id, raw_code, classification, scanned_by) VALUES ($1, $2, $3, $5, 'unexpected_found', $4)", uuid.New(), sessionID, unitUnexpectedShipped, adminID, codeUnexpectedShipped)
	require.NoError(t, err)
	_, err = testDB.Exec(ctx, "INSERT INTO inventory_reconciliation_scans (id, session_id, inventory_unit_id, raw_code, classification, scanned_by) VALUES ($1, $2, $3, $5, 'unexpected_found', $4)", uuid.New(), sessionID, unitUnexpectedFree, adminID, codeUnexpectedFree)
	require.NoError(t, err)

	// Record unit counts before reading resolution plan
	var beforeUnitsCount int
	err = testDB.QueryRow(ctx, "SELECT count(*) FROM inventory_units WHERE product_variant_id = $1", variantID).Scan(&beforeUnitsCount)
	require.NoError(t, err)

	plan, err := repo.GetReconciliationResolutionPlan(ctx, sessionID)
	require.NoError(t, err)
	require.NotNil(t, plan)
	require.Equal(t, sessionID, plan.SessionID)
	require.Len(t, plan.Cases, 4)

	// Verify ZERO DB MUTATIONS
	var afterUnitsCount int
	err = testDB.QueryRow(ctx, "SELECT count(*) FROM inventory_units WHERE product_variant_id = $1", variantID).Scan(&afterUnitsCount)
	require.NoError(t, err)
	require.Equal(t, beforeUnitsCount, afterUnitsCount, "Read model must never mutate inventory units")

	caseTypes := make(map[string]inventory.ReconciliationResolutionCase)
	for _, c := range plan.Cases {
		caseTypes[c.CaseType] = c
	}

	require.Contains(t, caseTypes, inventory.CaseTypeMissingFree)
	require.Equal(t, inventory.SeverityWarning, caseTypes[inventory.CaseTypeMissingFree].Severity)
	require.Equal(t, "Единица не найдена", caseTypes[inventory.CaseTypeMissingFree].Title)

	require.Contains(t, caseTypes, inventory.CaseTypeMissingLiveAllocated)
	require.Equal(t, inventory.SeverityHigh, caseTypes[inventory.CaseTypeMissingLiveAllocated].Severity)
	require.Equal(t, "Не найдена — назначена заказу", caseTypes[inventory.CaseTypeMissingLiveAllocated].Title)
	require.Contains(t, caseTypes[inventory.CaseTypeMissingLiveAllocated].Explanation, orderNum)

	require.Contains(t, caseTypes, inventory.CaseTypeShippedFound)
	require.Equal(t, inventory.SeverityCritical, caseTypes[inventory.CaseTypeShippedFound].Severity)
	require.Equal(t, "Найдена, хотя числится отгруженной", caseTypes[inventory.CaseTypeShippedFound].Title)

	require.Contains(t, caseTypes, inventory.CaseTypeUnexpectedFree)
	require.Equal(t, inventory.SeverityInfo, caseTypes[inventory.CaseTypeUnexpectedFree].Severity)
	require.Equal(t, "Неожиданно найден свободный остаток", caseTypes[inventory.CaseTypeUnexpectedFree].Title)
}

func setupReconciliationResolutionTestService(t *testing.T) (context.Context, *inventory.Service, *inventory.Repository, uuid.UUID, uuid.UUID, uuid.UUID) {
	ctx, repo, variantID, adminID, sellerID := setupReconciliationEnv(t)
	pgClient, err := postgres.NewClient(ctx, testutil.GetTestDatabaseURL())
	require.NoError(t, err)
	svc := inventory.NewService(repo, nil, pgClient)
	return ctx, svc, repo, variantID, adminID, sellerID
}

func TestResolveReconciliationCase_ConfirmMissing_FreeAndAccountingInvariants(t *testing.T) {
	ctx, svc, repo, variantID, adminID, sellerID := setupReconciliationResolutionTestService(t)

	supplyID := uuid.New()
	_, err := testDB.Exec(ctx, "INSERT INTO seller_supplies (id, supply_number, seller_id, status, handoff_method, created_at, updated_at) VALUES ($1, $3, $2, 'completed', 'courier', now(), now())", supplyID, sellerID, "SUP-"+uuid.New().String()[:8])
	require.NoError(t, err)

	supplyItemID := uuid.New()
	_, err = testDB.Exec(ctx, "INSERT INTO seller_supply_items (id, supply_id, variant_id, expected_quantity, accepted_quantity, damaged_quantity, missing_quantity, extra_quantity, created_at, updated_at) VALUES ($1, $2, $3, 10, 0, 0, 0, 0, now(), now())", supplyItemID, supplyID, variantID)
	require.NoError(t, err)

	_, err = testDB.Exec(ctx, "UPDATE inventory_items SET total_stock = 5, reserved_stock = 0 WHERE product_variant_id = $1", variantID)
	require.NoError(t, err)

	unit1 := uuid.New()
	unit2 := uuid.New()
	code1 := mustGenerateUnitCode()
	code2 := mustGenerateUnitCode()

	_, err = testDB.Exec(ctx, "INSERT INTO inventory_units (id, product_variant_id, origin_supply_id, origin_supply_item_id, unit_code, status, unit_index, created_at, updated_at) VALUES ($1, $2, $3, $4, $5, 'warehouse', 1, now(), now())", unit1, variantID, supplyID, supplyItemID, code1)
	require.NoError(t, err)
	_, err = testDB.Exec(ctx, "INSERT INTO inventory_units (id, product_variant_id, origin_supply_id, origin_supply_item_id, unit_code, status, unit_index, created_at, updated_at) VALUES ($1, $2, $3, $4, $5, 'warehouse', 2, now(), now())", unit2, variantID, supplyID, supplyItemID, code2)
	require.NoError(t, err)

	sessionID := uuid.New()
	require.NoError(t, repo.StartReconciliationSession(ctx, sessionID, variantID, adminID))
	require.NoError(t, repo.ChangeReconciliationStatus(ctx, sessionID, "in_progress", "review", adminID))

	req := inventory.ResolveReconciliationCaseRequest{
		UnitID:   &unit1,
		ActionID: inventory.ActionIDConfirmMissing,
	}
	plan, err := svc.ResolveReconciliationCase(ctx, sessionID, adminID, req)
	require.NoError(t, err)
	require.NotNil(t, plan)

	var unit1Case *inventory.ReconciliationResolutionCase
	for i := range plan.Cases {
		if plan.Cases[i].UnitID == unit1 {
			unit1Case = &plan.Cases[i]
			break
		}
	}
	require.NotNil(t, unit1Case, "Resolved unit must be present in returned plan")
	require.NotNil(t, unit1Case.Resolution, "Resolution audit must be populated")
	require.Equal(t, inventory.ActionIDConfirmMissing, unit1Case.Resolution.ActionID)
	require.Equal(t, adminID, unit1Case.Resolution.PerformedBy)
	require.Empty(t, unit1Case.AllowedActions, "Allowed actions must be empty for resolved case")

	var u1Status string
	err = testDB.QueryRow(ctx, "SELECT status FROM inventory_units WHERE id = $1", unit1).Scan(&u1Status)
	require.NoError(t, err)
	require.Equal(t, "written_off", u1Status)

	var totalStock, resStock int
	err = testDB.QueryRow(ctx, "SELECT total_stock, reserved_stock FROM inventory_items WHERE product_variant_id = $1", variantID).Scan(&totalStock, &resStock)
	require.NoError(t, err)
	require.Equal(t, 4, totalStock, "Aggregate total stock must be decremented by 1")
	require.Equal(t, 0, resStock)

	var physWhCount int
	err = testDB.QueryRow(ctx, "SELECT count(*) FROM inventory_units WHERE product_variant_id = $1 AND status = 'warehouse'", variantID).Scan(&physWhCount)
	require.NoError(t, err)
	require.Equal(t, 1, physWhCount)

	legacyOnHand := totalStock - physWhCount
	require.Equal(t, 3, legacyOnHand, "Legacy on_hand stock must remain unchanged after physical write-off")

	var movCount int
	err = testDB.QueryRow(ctx, "SELECT count(*) FROM stock_movements WHERE reference_id = $1 AND type = 'write_off' AND quantity = 1", sessionID).Scan(&movCount)
	require.NoError(t, err)
	require.Equal(t, 1, movCount)


	var auditCount int
	err = testDB.QueryRow(ctx, "SELECT count(*) FROM inventory_reconciliation_resolutions WHERE session_id = $1 AND inventory_unit_id = $2 AND action_id = 'confirm_missing'", sessionID, unit1).Scan(&auditCount)
	require.NoError(t, err)
	require.Equal(t, 1, auditCount)

	planIdempotent, err := svc.ResolveReconciliationCase(ctx, sessionID, adminID, req)
	require.NoError(t, err)
	require.NotNil(t, planIdempotent)

	err = testDB.QueryRow(ctx, "SELECT total_stock FROM inventory_items WHERE product_variant_id = $1", variantID).Scan(&totalStock)
	require.NoError(t, err)
	require.Equal(t, 4, totalStock, "Idempotent call must not double decrement aggregate stock")
}

func createTestOrderItem(ctx context.Context, t *testing.T, variantID, sellerID uuid.UUID) uuid.UUID {
	userID := uuid.New()
	_, err := testDB.Exec(ctx, "INSERT INTO users (id, email, name, password_hash, role) VALUES ($1, $2, 'cust', 'hash', 'customer')", userID, uuid.New().String()+"@test.com")
	require.NoError(t, err)

	orderID := uuid.New()
	orderItemID := uuid.New()
	orderNum := "ORD-" + uuid.New().String()[:8]
	_, err = testDB.Exec(ctx, "INSERT INTO orders (id, order_number, user_id, status, customer_name, customer_phone, customer_email, delivery_address, created_at, updated_at) VALUES ($1, $3, $2, 'assembling', 'cust', '+123', 'a@b.com', 'addr', now(), now())", orderID, userID, orderNum)
	require.NoError(t, err)

	fulfillmentID := uuid.New()
	_, err = testDB.Exec(ctx, "INSERT INTO order_fulfillments (id, order_id, seller_id, status, created_at, updated_at) VALUES ($1, $2, $3, 'assembling', now(), now())", fulfillmentID, orderID, sellerID)
	require.NoError(t, err)

	var pid uuid.UUID
	err = testDB.QueryRow(ctx, "SELECT product_id FROM product_variants WHERE id = $1", variantID).Scan(&pid)
	require.NoError(t, err)

	_, err = testDB.Exec(ctx, "INSERT INTO order_items (id, order_id, product_id, product_variant_id, seller_id, title, product_slug, price_cents, quantity, subtotal_price_cents, order_fulfillment_id, created_at) VALUES ($1, $2, $3, $4, $5, 'title', 'slug', 100, 1, 100, $6, now())", orderItemID, orderID, pid, variantID, sellerID, fulfillmentID)
	require.NoError(t, err)

	return orderItemID
}

func TestResolveReconciliationCase_CloseStaleAllocation_ThenConfirmMissing(t *testing.T) {
	ctx, svc, repo, variantID, adminID, sellerID := setupReconciliationResolutionTestService(t)

	supplyID := uuid.New()
	_, err := testDB.Exec(ctx, "INSERT INTO seller_supplies (id, supply_number, seller_id, status, handoff_method, created_at, updated_at) VALUES ($1, $3, $2, 'completed', 'courier', now(), now())", supplyID, sellerID, "SUP-"+uuid.New().String()[:8])
	require.NoError(t, err)

	supplyItemID := uuid.New()
	_, err = testDB.Exec(ctx, "INSERT INTO seller_supply_items (id, supply_id, variant_id, expected_quantity, accepted_quantity, damaged_quantity, missing_quantity, extra_quantity, created_at, updated_at) VALUES ($1, $2, $3, 10, 0, 0, 0, 0, now(), now())", supplyItemID, supplyID, variantID)
	require.NoError(t, err)

	_, err = testDB.Exec(ctx, "UPDATE inventory_items SET total_stock = 5, reserved_stock = 1 WHERE product_variant_id = $1", variantID)
	require.NoError(t, err)

	unitStale := uuid.New()
	codeStale := mustGenerateUnitCode()
	_, err = testDB.Exec(ctx, "INSERT INTO inventory_units (id, product_variant_id, origin_supply_id, origin_supply_item_id, unit_code, status, unit_index, created_at, updated_at) VALUES ($1, $2, $3, $4, $5, 'warehouse', 1, now(), now())", unitStale, variantID, supplyID, supplyItemID, codeStale)
	require.NoError(t, err)

	allocID := uuid.New()
	orderItemID := createTestOrderItem(ctx, t, variantID, sellerID)
	_, err = testDB.Exec(ctx, "UPDATE orders SET status = 'delivered' WHERE id = (SELECT order_id FROM order_items WHERE id = $1)", orderItemID)
	require.NoError(t, err)
	_, err = testDB.Exec(ctx, "INSERT INTO order_item_allocations (id, order_item_id, inventory_unit_id, created_at) VALUES ($1, $2, $3, now())", allocID, orderItemID, unitStale)
	require.NoError(t, err)

	sessionID := uuid.New()
	require.NoError(t, repo.StartReconciliationSession(ctx, sessionID, variantID, adminID))
	require.NoError(t, repo.ChangeReconciliationStatus(ctx, sessionID, "in_progress", "review", adminID))

	reqClose := inventory.ResolveReconciliationCaseRequest{
		UnitID:   &unitStale,
		ActionID: inventory.ActionIDCloseStaleAllocation,
	}
	_, err = svc.ResolveReconciliationCase(ctx, sessionID, adminID, reqClose)
	require.NoError(t, err)

	// Verify allocation released
	var releasedAt *time.Time
	var reason *string
	err = testDB.QueryRow(ctx, "SELECT released_at, release_reason FROM order_item_allocations WHERE id = $1", allocID).Scan(&releasedAt, &reason)
	require.NoError(t, err)
	require.NotNil(t, releasedAt)
	require.Equal(t, "inventory_reconciliation", *reason)

	// Verify unit status is STILL warehouse
	var uStatus string
	err = testDB.QueryRow(ctx, "SELECT status FROM inventory_units WHERE id = $1", unitStale).Scan(&uStatus)
	require.NoError(t, err)
	require.Equal(t, "warehouse", uStatus)

	// Verify reserved stock decremented
	var resStock int
	err = testDB.QueryRow(ctx, "SELECT reserved_stock FROM inventory_items WHERE product_variant_id = $1", variantID).Scan(&resStock)
	require.NoError(t, err)
	require.Equal(t, 0, resStock)

	// Step 2: Now perform terminal confirm_missing on the same unit!
	// Partial unique index MUST NOT block this.
	reqConfirm := inventory.ResolveReconciliationCaseRequest{
		UnitID:   &unitStale,
		ActionID: inventory.ActionIDConfirmMissing,
	}
	planAfterConfirm, err := svc.ResolveReconciliationCase(ctx, sessionID, adminID, reqConfirm)
	require.NoError(t, err, "Confirm missing must succeed after close_stale_allocation")
	require.NotNil(t, planAfterConfirm)

	err = testDB.QueryRow(ctx, "SELECT status FROM inventory_units WHERE id = $1", unitStale).Scan(&uStatus)
	require.NoError(t, err)
	require.Equal(t, "written_off", uStatus)

	// Resolution counts semantics: 2 audit actions, 1 resolved discrepancy case
	require.Equal(t, 2, planAfterConfirm.ResolutionsCount, "2 resolution audit records exist")
	require.Equal(t, 1, planAfterConfirm.ResolvedCasesCount, "1 discrepancy case was resolved")

	// Verify unit traceability timeline
	trc, err := repo.GetAdminInventoryUnitTraceability(ctx, codeStale)
	require.NoError(t, err)
	var foundCloseStale, foundWrittenOff bool
	for _, ev := range trc.Timeline {
		if ev.EventName == "Старое назначение освобождено" {
			foundCloseStale = true
			require.Contains(t, ev.Description, "освобождено по результатам инвентаризации")
		}
		if ev.EventName == "Списана по результатам инвентаризации" {
			foundWrittenOff = true
		}
	}
	require.True(t, foundCloseStale, "Must include 'Старое назначение освобождено'")
	require.True(t, foundWrittenOff, "Must include 'Списана по результатам инвентаризации'")
}

func TestResolveReconciliationCase_MissingLiveAllocated_RequiresReplacement(t *testing.T) {
	ctx, svc, repo, variantID, adminID, sellerID := setupReconciliationResolutionTestService(t)

	supplyID := uuid.New()
	_, err := testDB.Exec(ctx, "INSERT INTO seller_supplies (id, supply_number, seller_id, status, handoff_method, created_at, updated_at) VALUES ($1, $3, $2, 'completed', 'courier', now(), now())", supplyID, sellerID, "SUP-"+uuid.New().String()[:8])
	require.NoError(t, err)

	supplyItemID := uuid.New()
	_, err = testDB.Exec(ctx, "INSERT INTO seller_supply_items (id, supply_id, variant_id, expected_quantity, accepted_quantity, damaged_quantity, missing_quantity, extra_quantity, created_at, updated_at) VALUES ($1, $2, $3, 10, 0, 0, 0, 0, now(), now())", supplyItemID, supplyID, variantID)
	require.NoError(t, err)

	_, err = testDB.Exec(ctx, "UPDATE inventory_items SET total_stock = 5, reserved_stock = 1 WHERE product_variant_id = $1", variantID)
	require.NoError(t, err)

	unitLive := uuid.New()
	codeLive := mustGenerateUnitCode()
	_, err = testDB.Exec(ctx, "INSERT INTO inventory_units (id, product_variant_id, origin_supply_id, origin_supply_item_id, unit_code, status, unit_index, created_at, updated_at) VALUES ($1, $2, $3, $4, $5, 'warehouse', 1, now(), now())", unitLive, variantID, supplyID, supplyItemID, codeLive)
	require.NoError(t, err)

	allocID := uuid.New()
	orderItemID := createTestOrderItem(ctx, t, variantID, sellerID)
	_, err = testDB.Exec(ctx, "INSERT INTO order_item_allocations (id, order_item_id, inventory_unit_id, created_at) VALUES ($1, $2, $3, now())", allocID, orderItemID, unitLive)
	require.NoError(t, err)

	sessionID := uuid.New()
	require.NoError(t, repo.StartReconciliationSession(ctx, sessionID, variantID, adminID))
	require.NoError(t, repo.ChangeReconciliationStatus(ctx, sessionID, "in_progress", "review", adminID))

	reqWithoutRep := inventory.ResolveReconciliationCaseRequest{
		UnitID:   &unitLive,
		ActionID: inventory.ActionIDConfirmMissing,
	}
	_, err = svc.ResolveReconciliationCase(ctx, sessionID, adminID, reqWithoutRep)
	require.Error(t, err, "Must fail without replacement unit for live allocation")

	unitRep := uuid.New()
	codeRep := mustGenerateUnitCode()
	_, err = testDB.Exec(ctx, "INSERT INTO inventory_units (id, product_variant_id, origin_supply_id, origin_supply_item_id, unit_code, status, unit_index, created_at, updated_at) VALUES ($1, $2, $3, $4, $5, 'warehouse', 2, now(), now())", unitRep, variantID, supplyID, supplyItemID, codeRep)
	require.NoError(t, err)

	reqWithRep := inventory.ResolveReconciliationCaseRequest{
		UnitID:            &unitLive,
		ActionID:          inventory.ActionIDConfirmMissing,
		ReplacementUnitID: &unitRep,
	}
	plan, err := svc.ResolveReconciliationCase(ctx, sessionID, adminID, reqWithRep)
	require.NoError(t, err)
	require.NotNil(t, plan)

	var oldStatus string
	err = testDB.QueryRow(ctx, "SELECT status FROM inventory_units WHERE id = $1", unitLive).Scan(&oldStatus)
	require.NoError(t, err)
	require.Equal(t, "written_off", oldStatus)

	var allocUnitID uuid.UUID
	err = testDB.QueryRow(ctx, "SELECT inventory_unit_id FROM order_item_allocations WHERE id = $1", allocID).Scan(&allocUnitID)
	require.NoError(t, err)
	require.Equal(t, unitRep, allocUnitID, "Allocation must be rebound to replacement unit")

	var repResCount int
	err = testDB.QueryRow(ctx, "SELECT count(*) FROM inventory_reconciliation_resolutions WHERE session_id = $1 AND inventory_unit_id = $2 AND replacement_inventory_unit_id = $3", sessionID, unitLive, unitRep).Scan(&repResCount)
	require.NoError(t, err)
	require.Equal(t, 1, repResCount)
}

func TestResolveReconciliationCase_SessionNotInReview(t *testing.T) {
	ctx, svc, repo, variantID, adminID, sellerID := setupReconciliationResolutionTestService(t)

	supplyID := uuid.New()
	_, err := testDB.Exec(ctx, "INSERT INTO seller_supplies (id, supply_number, seller_id, status, handoff_method, created_at, updated_at) VALUES ($1, $3, $2, 'completed', 'courier', now(), now())", supplyID, sellerID, "SUP-"+uuid.New().String()[:8])
	require.NoError(t, err)

	supplyItemID := uuid.New()
	_, err = testDB.Exec(ctx, "INSERT INTO seller_supply_items (id, supply_id, variant_id, expected_quantity, accepted_quantity, damaged_quantity, missing_quantity, extra_quantity, created_at, updated_at) VALUES ($1, $2, $3, 10, 0, 0, 0, 0, now(), now())", supplyItemID, supplyID, variantID)
	require.NoError(t, err)

	unit1 := uuid.New()
	code1 := mustGenerateUnitCode()
	_, err = testDB.Exec(ctx, "INSERT INTO inventory_units (id, product_variant_id, origin_supply_id, origin_supply_item_id, unit_code, status, unit_index, created_at, updated_at) VALUES ($1, $2, $3, $4, $5, 'warehouse', 1, now(), now())", unit1, variantID, supplyID, supplyItemID, code1)
	require.NoError(t, err)

	sessionID := uuid.New()
	require.NoError(t, repo.StartReconciliationSession(ctx, sessionID, variantID, adminID))

	req := inventory.ResolveReconciliationCaseRequest{
		UnitID:   &unit1,
		ActionID: inventory.ActionIDConfirmMissing,
	}
	_, err = svc.ResolveReconciliationCase(ctx, sessionID, adminID, req)
	require.Error(t, err)
	require.ErrorIs(t, err, inventory.ErrInvalidReconciliationState)
}
