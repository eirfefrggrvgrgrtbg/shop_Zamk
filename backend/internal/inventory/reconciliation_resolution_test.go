package inventory_test

import (
	"testing"
	"time"


	"github.com/eirfefrggrvgrgrtbg/shop-zamk/backend/internal/inventory"
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
		require.False(t, c.AllowedActions[1].Enabled)
		require.NotEmpty(t, c.AllowedActions[1].BlockedReason)
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
		// close_stale_allocation is disabled until P2.2B
		var hasCloseStale bool
		for _, a := range c.AllowedActions {
			if a.ID == inventory.ActionIDCloseStaleAllocation {
				hasCloseStale = true
				require.False(t, a.Enabled)
				require.NotEmpty(t, a.BlockedReason)
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

