package inventory_test

import (
	"context"
	"sync"
	"testing"

	"github.com/eirfefrggrvgrgrtbg/shop-zamk/backend/internal/inventory"
	"github.com/eirfefrggrvgrgrtbg/shop-zamk/backend/internal/supplies"
	"github.com/eirfefrggrvgrgrtbg/shop-zamk/backend/internal/testutil"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

func setupReconciliationEnv(t *testing.T) (context.Context, *inventory.Repository, uuid.UUID, uuid.UUID, uuid.UUID) {
	ctx := context.Background()
	testutil.AssertTestDatabase(t, testDB)

	pfx := "recon_" + uuid.NewString()[:8]
	sellerID := uuid.New()
	_, err := testDB.Exec(ctx, `
		INSERT INTO sellers (id, brand_name, slug, contact_email, status)
		VALUES ($1, 'Recon Brand', $2, $3, 'active')
	`, sellerID, pfx+"-brand", pfx+"@test.com")
	require.NoError(t, err)

	productID := uuid.New()
	_, err = testDB.Exec(ctx, `
		INSERT INTO products (id, title, slug, price_cents, status, seller_id)
		VALUES ($1, 'Recon Prod', $2, 1000, 'published', $3)
	`, productID, pfx+"-prod", sellerID)
	require.NoError(t, err)

	variantID := uuid.New()
	_, err = testDB.Exec(ctx, `
		INSERT INTO product_variants (id, product_id, sku, barcode)
		VALUES ($1, $2, $3, $4)
	`, variantID, productID, "SKU-"+pfx, "BAR-"+pfx)
	require.NoError(t, err)

	_, err = testDB.Exec(ctx, `
		INSERT INTO inventory_items (id, product_id, product_variant_id, seller_id)
		VALUES ($1, $2, $3, $4)
	`, uuid.New(), productID, variantID, sellerID)
	require.NoError(t, err)

	adminID := uuid.New()
	_, err = testDB.Exec(ctx, "INSERT INTO users (id, email, name, password_hash, role) VALUES ($1, $2, 'admin', 'hash', 'admin')", adminID, pfx+"-admin@test.com")
	require.NoError(t, err)

	repo := inventory.NewRepository(testDB)
	return ctx, repo, variantID, adminID, sellerID
}

func TestReconciliation_StartSession(t *testing.T) {
	ctx, repo, variantID, adminID, sellerID := setupReconciliationEnv(t)

	supplyID := uuid.New()
	_, err := testDB.Exec(ctx, "INSERT INTO seller_supplies (id, seller_id, status, supply_number, handoff_method, created_at, updated_at) VALUES ($1, $2, 'completed', $3::text::text, 'hub', NOW(), NOW())", supplyID, sellerID, uuid.NewString())
	require.NoError(t, err)

	supplyItemID := uuid.New()
	_, err = testDB.Exec(ctx, "INSERT INTO seller_supply_items (id, supply_id, variant_id, expected_quantity, created_at, updated_at) VALUES ($1, $2, $3, 10, NOW(), NOW())", supplyItemID, supplyID, variantID)
	require.NoError(t, err)

	unit1 := uuid.New()
	unit2 := uuid.New()
	unit3 := uuid.New()

	_, err = testDB.Exec(ctx, `
		INSERT INTO inventory_units (id, unit_code, product_variant_id, origin_supply_id, origin_supply_item_id, unit_index, status)
		VALUES
		($1, $7, $4, $5, $6, 1, 'warehouse'),
		($2, $8, $4, $5, $6, 2, 'shipped'),
		($3, $9, $4, $5, $6, 3, 'warehouse')
	`, unit1, unit2, unit3, variantID, supplyID, supplyItemID, mustGenerateUnitCode(), mustGenerateUnitCode(), mustGenerateUnitCode())
	require.NoError(t, err)

	sessionID := uuid.New()
	err = repo.StartReconciliationSession(ctx, sessionID, variantID, adminID)
	require.NoError(t, err)

	session, err := repo.GetActiveReconciliationSession(ctx, variantID)
	require.NoError(t, err)
	require.NotNil(t, session)
	require.Equal(t, sessionID, session.ID)
	require.Equal(t, 2, session.ExpectedCount)

	err = repo.StartReconciliationSession(ctx, uuid.New(), variantID, adminID)
	require.Error(t, err)
}

func TestReconciliation_ScanClassifications(t *testing.T) {
	ctx, repo, variantID, adminID, sellerID := setupReconciliationEnv(t)

	supplyID := uuid.New()
	_, err := testDB.Exec(ctx, "INSERT INTO seller_supplies (id, seller_id, status, supply_number, handoff_method, created_at, updated_at) VALUES ($1, $2, 'completed', $3::text::text, 'hub', NOW(), NOW())", supplyID, sellerID, uuid.NewString())
	require.NoError(t, err)

	supplyItemID := uuid.New()
	_, err = testDB.Exec(ctx, "INSERT INTO seller_supply_items (id, supply_id, variant_id, expected_quantity, created_at, updated_at) VALUES ($1, $2, $3, 10, NOW(), NOW())", supplyItemID, supplyID, variantID)
	require.NoError(t, err)

	unitExpected := uuid.New()
	zmuExpected := mustGenerateUnitCode()
	unitShipped := uuid.New()
	zmuShipped := mustGenerateUnitCode()

	_, err = testDB.Exec(ctx, `
		INSERT INTO inventory_units (id, unit_code, product_variant_id, origin_supply_id, origin_supply_item_id, unit_index, status)
		VALUES
		($1, $6, $3, $4, $5, 1, 'warehouse'),
		($2, $7, $3, $4, $5, 2, 'shipped')
	`, unitExpected, unitShipped, variantID, supplyID, supplyItemID, zmuExpected, zmuShipped)
	require.NoError(t, err)

	// Other variant

	otherProductID := uuid.New()
	pfxOther := "other_" + uuid.NewString()[:8]
	_, err = testDB.Exec(ctx, `
		INSERT INTO products (id, title, slug, price_cents, status, seller_id)
		VALUES ($1, 'Other Prod', $2, 1000, 'published', $3)
	`, otherProductID, pfxOther+"-prod", sellerID)
    require.NoError(t, err)

	otherVariantID := uuid.New()
	_, err = testDB.Exec(ctx, `
		INSERT INTO product_variants (id, product_id, sku, barcode)
		VALUES ($1, $2, $3, $4)
	`, otherVariantID, otherProductID, "SKU-"+pfxOther, "BAR-"+pfxOther)
	require.NoError(t, err)


	_, err = testDB.Exec(ctx, `
		INSERT INTO inventory_items (id, product_id, product_variant_id, seller_id)
		VALUES ($1, $2, $3, $4)
	`, uuid.New(), otherProductID, otherVariantID, sellerID)
	require.NoError(t, err)

	otherSupplyItemID := uuid.New()
	_, err = testDB.Exec(ctx, "INSERT INTO seller_supply_items (id, supply_id, variant_id, expected_quantity, created_at, updated_at) VALUES ($1, $2, $3, 10, NOW(), NOW())", otherSupplyItemID, supplyID, otherVariantID)
	require.NoError(t, err)

	unitOther := uuid.New()
	zmuOther := mustGenerateUnitCode()
	_, err = testDB.Exec(ctx, `
		INSERT INTO inventory_units (id, unit_code, product_variant_id, origin_supply_id, origin_supply_item_id, unit_index, status)
		VALUES ($1, $5, $2, $3, $4, 1, 'warehouse')
	`, unitOther, otherVariantID, supplyID, otherSupplyItemID, zmuOther)
	require.NoError(t, err)

	sessionID := uuid.New()
	err = repo.StartReconciliationSession(ctx, sessionID, variantID, adminID)
	require.NoError(t, err)

	res, err := repo.ProcessReconciliationScan(ctx, sessionID, zmuExpected, adminID)
	require.NoError(t, err)
	require.Equal(t, "expected_found", res.Classification)

	res, err = repo.ProcessReconciliationScan(ctx, sessionID, zmuExpected, adminID)
	require.NoError(t, err)
	require.Equal(t, "duplicate", res.Classification)

	res, err = repo.ProcessReconciliationScan(ctx, sessionID, zmuShipped, adminID)
	require.NoError(t, err)
	require.Equal(t, "unexpected_found", res.Classification)

	res, err = repo.ProcessReconciliationScan(ctx, sessionID, zmuOther, adminID)
	require.NoError(t, err)
	require.Equal(t, "wrong_variant", res.Classification)

	res, err = repo.ProcessReconciliationScan(ctx, sessionID, "ZMU-MAGIC", adminID)
	require.NoError(t, err)
	require.Equal(t, "unknown_code", res.Classification)
}

func TestReconciliation_AtomicScanIdempotency(t *testing.T) {
	ctx, repo, variantID, adminID, sellerID := setupReconciliationEnv(t)

	supplyID := uuid.New()
	_, err := testDB.Exec(ctx, "INSERT INTO seller_supplies (id, seller_id, status, supply_number, handoff_method, created_at, updated_at) VALUES ($1, $2, 'completed', $3::text::text, 'hub', NOW(), NOW())", supplyID, sellerID, uuid.NewString())
	require.NoError(t, err)

	supplyItemID := uuid.New()
	_, err = testDB.Exec(ctx, "INSERT INTO seller_supply_items (id, supply_id, variant_id, expected_quantity, created_at, updated_at) VALUES ($1, $2, $3, 10, NOW(), NOW())", supplyItemID, supplyID, variantID)
	require.NoError(t, err)

	unitID := uuid.New()
	zmu := mustGenerateUnitCode()
	_, err = testDB.Exec(ctx, `
		INSERT INTO inventory_units (id, unit_code, product_variant_id, origin_supply_id, origin_supply_item_id, unit_index, status)
		VALUES ($1, $5, $2, $3, $4, 1, 'warehouse')
	`, unitID, variantID, supplyID, supplyItemID, zmu)
	require.NoError(t, err)

	sessionID := uuid.New()
	err = repo.StartReconciliationSession(ctx, sessionID, variantID, adminID)
	require.NoError(t, err)

	// Concurrently scan the SAME unit
	var wg sync.WaitGroup
	results := make(chan *inventory.ScanReconciliationResponse, 2)
	errorsChan := make(chan error, 2)

	for i := 0; i < 2; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			res, err := repo.ProcessReconciliationScan(ctx, sessionID, zmu, adminID)
			if err != nil {
				errorsChan <- err
			} else {
				results <- res
			}
		}()
	}
	wg.Wait()
	close(results)
	close(errorsChan)

	require.Empty(t, errorsChan)
	var classifications []string
	for res := range results {
		classifications = append(classifications, res.Classification)
	}
	require.Len(t, classifications, 2)
	require.Contains(t, classifications, "expected_found")
	require.Contains(t, classifications, "duplicate")

	var scanCount int
	err = testDB.QueryRow(ctx, "SELECT count(*) FROM inventory_reconciliation_scans WHERE session_id = $1 AND inventory_unit_id = $2", sessionID, unitID).Scan(&scanCount)
	require.NoError(t, err)
	require.Equal(t, 1, scanCount, "Exactly 1 scan must be persisted for resolved unit")
}

func TestReconciliation_ConcurrentScanAndReviewTransition(t *testing.T) {
	ctx, repo, variantID, adminID, sellerID := setupReconciliationEnv(t)

	supplyID := uuid.New()
	_, err := testDB.Exec(ctx, "INSERT INTO seller_supplies (id, seller_id, status, supply_number, handoff_method, created_at, updated_at) VALUES ($1, $2, 'completed', $3::text::text, 'hub', NOW(), NOW())", supplyID, sellerID, uuid.NewString())
	require.NoError(t, err)

	supplyItemID := uuid.New()
	_, err = testDB.Exec(ctx, "INSERT INTO seller_supply_items (id, supply_id, variant_id, expected_quantity, created_at, updated_at) VALUES ($1, $2, $3, 10, NOW(), NOW())", supplyItemID, supplyID, variantID)
	require.NoError(t, err)

	unitID := uuid.New()
	zmu := mustGenerateUnitCode()
	_, err = testDB.Exec(ctx, `
		INSERT INTO inventory_units (id, unit_code, product_variant_id, origin_supply_id, origin_supply_item_id, unit_index, status)
		VALUES ($1, $5, $2, $3, $4, 1, 'warehouse')
	`, unitID, variantID, supplyID, supplyItemID, zmu)
	require.NoError(t, err)

	sessionID := uuid.New()
	err = repo.StartReconciliationSession(ctx, sessionID, variantID, adminID)
	require.NoError(t, err)

	// Move to review
	err = repo.ChangeReconciliationStatus(ctx, sessionID, "in_progress", "review", adminID)
	require.NoError(t, err)

	// Attempt scan after review transition
	_, err = repo.ProcessReconciliationScan(ctx, sessionID, zmu, adminID)
	require.ErrorIs(t, err, inventory.ErrReconciliationNotInProgress)

	var scanCount int
	err = testDB.QueryRow(ctx, "SELECT count(*) FROM inventory_reconciliation_scans WHERE session_id = $1", sessionID).Scan(&scanCount)
	require.NoError(t, err)
	require.Equal(t, 0, scanCount, "No scan can be committed when session is not in_progress")
}

func TestReconciliation_StateMachine(t *testing.T) {
	ctx, repo, variantID, adminID, sellerID := setupReconciliationEnv(t)

	supplyID := uuid.New()
	_, err := testDB.Exec(ctx, "INSERT INTO seller_supplies (id, seller_id, status, supply_number, handoff_method, created_at, updated_at) VALUES ($1, $2, 'completed', $3::text::text, 'hub', NOW(), NOW())", supplyID, sellerID, uuid.NewString())
	require.NoError(t, err)

	supplyItemID := uuid.New()
	_, err = testDB.Exec(ctx, "INSERT INTO seller_supply_items (id, supply_id, variant_id, expected_quantity, created_at, updated_at) VALUES ($1, $2, $3, 10, NOW(), NOW())", supplyItemID, supplyID, variantID)
	require.NoError(t, err)

	unitID := uuid.New()
	zmu := mustGenerateUnitCode()
	_, err = testDB.Exec(ctx, `
		INSERT INTO inventory_units (id, unit_code, product_variant_id, origin_supply_id, origin_supply_item_id, unit_index, status)
		VALUES ($1, $5, $2, $3, $4, 1, 'warehouse')
	`, unitID, variantID, supplyID, supplyItemID, zmu)
	require.NoError(t, err)

	// 1. Legal: in_progress -> review -> completed
	session1ID := uuid.New()
	err = repo.StartReconciliationSession(ctx, session1ID, variantID, adminID)
	require.NoError(t, err)

	// Second active session while first is in_progress -> 409
	err = repo.StartReconciliationSession(ctx, uuid.New(), variantID, adminID)
	require.ErrorIs(t, err, inventory.ErrReconciliationAlreadyActive)

	// in_progress -> review
	err = repo.ChangeReconciliationStatus(ctx, session1ID, "in_progress", "review", adminID)
	require.NoError(t, err)

	// Second active session while first is review -> 409
	err = repo.StartReconciliationSession(ctx, uuid.New(), variantID, adminID)
	require.ErrorIs(t, err, inventory.ErrReconciliationAlreadyActive)

	// Illegal: scan in review
	_, err = repo.ProcessReconciliationScan(ctx, session1ID, zmu, adminID)
	require.ErrorIs(t, err, inventory.ErrReconciliationNotInProgress)

	// review -> completed
	err = repo.ChangeReconciliationStatus(ctx, session1ID, "review", "completed", adminID)
	require.NoError(t, err)

	// Illegal: scan in completed
	_, err = repo.ProcessReconciliationScan(ctx, session1ID, zmu, adminID)
	require.ErrorIs(t, err, inventory.ErrReconciliationNotInProgress)

	// Illegal: completed -> review
	err = repo.ChangeReconciliationStatus(ctx, session1ID, "completed", "review", adminID)
	require.ErrorIs(t, err, inventory.ErrInvalidReconciliationState)

	// 2. Completed session allows new session to start
	session2ID := uuid.New()
	err = repo.StartReconciliationSession(ctx, session2ID, variantID, adminID)
	require.NoError(t, err)

	// Legal: in_progress -> cancelled
	err = repo.ChangeReconciliationStatus(ctx, session2ID, "in_progress", "cancelled", adminID)
	require.NoError(t, err)

	// Illegal: scan in cancelled
	_, err = repo.ProcessReconciliationScan(ctx, session2ID, zmu, adminID)
	require.ErrorIs(t, err, inventory.ErrReconciliationNotInProgress)

	// Illegal: cancelled -> review
	err = repo.ChangeReconciliationStatus(ctx, session2ID, "cancelled", "review", adminID)
	require.ErrorIs(t, err, inventory.ErrInvalidReconciliationState)

	// 3. Cancelled session allows new session to start
	session3ID := uuid.New()
	err = repo.StartReconciliationSession(ctx, session3ID, variantID, adminID)
	require.NoError(t, err)
}

func TestReconciliation_WrongVariantAndUnexpectedContext(t *testing.T) {
	ctx, repo, variantID, adminID, sellerID := setupReconciliationEnv(t)

	supplyID := uuid.New()
	_, err := testDB.Exec(ctx, "INSERT INTO seller_supplies (id, seller_id, status, supply_number, handoff_method, created_at, updated_at) VALUES ($1, $2, 'completed', $3::text::text, 'hub', NOW(), NOW())", supplyID, sellerID, uuid.NewString())
	require.NoError(t, err)

	supplyItemID := uuid.New()
	_, err = testDB.Exec(ctx, "INSERT INTO seller_supply_items (id, supply_id, variant_id, expected_quantity, created_at, updated_at) VALUES ($1, $2, $3, 10, NOW(), NOW())", supplyItemID, supplyID, variantID)
	require.NoError(t, err)

	// Same variant unexpected unit (shipped)
	zmuUnexpected := mustGenerateUnitCode()
	_, err = testDB.Exec(ctx, `
		INSERT INTO inventory_units (id, unit_code, product_variant_id, origin_supply_id, origin_supply_item_id, unit_index, status)
		VALUES ($1, $5, $2, $3, $4, 1, 'shipped')
	`, uuid.New(), variantID, supplyID, supplyItemID, zmuUnexpected)
	require.NoError(t, err)

	// Wrong variant unit with complete product details
	otherProductID := uuid.New()
	pfxOther := "wv_" + uuid.NewString()[:8]
	_, err = testDB.Exec(ctx, `
		INSERT INTO products (id, title, slug, price_cents, status, seller_id)
		VALUES ($1, 'Silk Scarf', $2, 2500, 'published', $3)
	`, otherProductID, pfxOther+"-prod", sellerID)
	require.NoError(t, err)

	otherVariantID := uuid.New()
	_, err = testDB.Exec(ctx, `
		INSERT INTO product_variants (id, product_id, sku, barcode, size, color)
		VALUES ($1, $2, $3, $4, 'XL', 'Красный')
	`, otherVariantID, otherProductID, "SKU-"+pfxOther, "BAR-"+pfxOther)
	require.NoError(t, err)

	otherSupplyItemID := uuid.New()
	_, err = testDB.Exec(ctx, "INSERT INTO seller_supply_items (id, supply_id, variant_id, expected_quantity, created_at, updated_at) VALUES ($1, $2, $3, 5, NOW(), NOW())", otherSupplyItemID, supplyID, otherVariantID)
	require.NoError(t, err)

	zmuWrongVariant := mustGenerateUnitCode()
	_, err = testDB.Exec(ctx, `
		INSERT INTO inventory_units (id, unit_code, product_variant_id, origin_supply_id, origin_supply_item_id, unit_index, status)
		VALUES ($1, $5, $2, $3, $4, 1, 'warehouse')
	`, uuid.New(), otherVariantID, supplyID, otherSupplyItemID, zmuWrongVariant)
	require.NoError(t, err)

	sessionID := uuid.New()
	err = repo.StartReconciliationSession(ctx, sessionID, variantID, adminID)
	require.NoError(t, err)

	// Scan wrong variant
	resWV, err := repo.ProcessReconciliationScan(ctx, sessionID, zmuWrongVariant, adminID)
	require.NoError(t, err)
	require.Equal(t, "wrong_variant", resWV.Classification)
	require.NotNil(t, resWV.UnitContext)
	require.Equal(t, "Silk Scarf", resWV.UnitContext.ProductTitle)
	require.Equal(t, "XL", resWV.UnitContext.Size)
	require.Equal(t, "Красный", resWV.UnitContext.Color)
	require.Equal(t, "SKU-"+pfxOther, resWV.UnitContext.SKU)
	require.Equal(t, "BAR-"+pfxOther, resWV.UnitContext.Barcode)
	require.Equal(t, "warehouse", resWV.UnitContext.Status)

	// Scan same variant unexpected
	resUnexp, err := repo.ProcessReconciliationScan(ctx, sessionID, zmuUnexpected, adminID)
	require.NoError(t, err)
	require.Equal(t, "unexpected_found", resUnexp.Classification)
	require.NotNil(t, resUnexp.UnitContext)
	require.Equal(t, "shipped", resUnexp.UnitContext.Status)
}

func TestReconciliation_ListSessionsByVariant(t *testing.T) {
	ctx, repo, variantID, adminID, sellerID := setupReconciliationEnv(t)

	supplyID := uuid.New()
	_, err := testDB.Exec(ctx, "INSERT INTO seller_supplies (id, seller_id, status, supply_number, handoff_method, created_at, updated_at) VALUES ($1, $2, 'completed', $3::text::text, 'hub', NOW(), NOW())", supplyID, sellerID, uuid.NewString())
	require.NoError(t, err)

	supplyItemID := uuid.New()
	_, err = testDB.Exec(ctx, "INSERT INTO seller_supply_items (id, supply_id, variant_id, expected_quantity, created_at, updated_at) VALUES ($1, $2, $3, 10, NOW(), NOW())", supplyItemID, supplyID, variantID)
	require.NoError(t, err)

	s1 := uuid.New()
	require.NoError(t, repo.StartReconciliationSession(ctx, s1, variantID, adminID))
	require.NoError(t, repo.ChangeReconciliationStatus(ctx, s1, "in_progress", "cancelled", adminID))

	s2 := uuid.New()
	require.NoError(t, repo.StartReconciliationSession(ctx, s2, variantID, adminID))
	require.NoError(t, repo.ChangeReconciliationStatus(ctx, s2, "in_progress", "review", adminID))
	require.NoError(t, repo.ChangeReconciliationStatus(ctx, s2, "review", "completed", adminID))

	s3 := uuid.New()
	require.NoError(t, repo.StartReconciliationSession(ctx, s3, variantID, adminID))

	sessions, err := repo.ListReconciliationSessionsByVariant(ctx, variantID, 10)
	require.NoError(t, err)
	require.Len(t, sessions, 3)
	require.Equal(t, s3, sessions[0].ID) // most recent
	require.Equal(t, "in_progress", sessions[0].Status)
	require.Equal(t, s2, sessions[1].ID)
	require.Equal(t, "completed", sessions[1].Status)
	require.Equal(t, s1, sessions[2].ID)
	require.Equal(t, "cancelled", sessions[2].Status)
}

func TestReconciliation_ZeroBusinessMutation(t *testing.T) {
	ctx, repo, variantID, adminID, sellerID := setupReconciliationEnv(t)

	supplyID := uuid.New()
	_, err := testDB.Exec(ctx, "INSERT INTO seller_supplies (id, seller_id, status, supply_number, handoff_method, created_at, updated_at) VALUES ($1, $2, 'completed', $3::text::text, 'hub', NOW(), NOW())", supplyID, sellerID, uuid.NewString())
	require.NoError(t, err)

	supplyItemID := uuid.New()
	_, err = testDB.Exec(ctx, "INSERT INTO seller_supply_items (id, supply_id, variant_id, expected_quantity, created_at, updated_at) VALUES ($1, $2, $3, 10, NOW(), NOW())", supplyItemID, supplyID, variantID)
	require.NoError(t, err)

	unitMissing := uuid.New()
	unitUnexpected := uuid.New()
	zmuMissing := mustGenerateUnitCode()
	zmuUnexpected := mustGenerateUnitCode()

	_, err = testDB.Exec(ctx, `
		INSERT INTO inventory_units (id, unit_code, product_variant_id, origin_supply_id, origin_supply_item_id, unit_index, status)
		VALUES
		($1, $6, $3, $4, $5, 1, 'warehouse'),
		($2, $7, $3, $4, $5, 2, 'shipped')
	`, unitMissing, unitUnexpected, variantID, supplyID, supplyItemID, zmuMissing, zmuUnexpected)
	require.NoError(t, err)

	// Snapshot state before: units, items, reservations, allocations
	type BusinessUnitState struct {
		Status string
	}
	beforeUnits := make(map[uuid.UUID]BusinessUnitState)

	rows, err := testDB.Query(ctx, "SELECT id, status FROM inventory_units WHERE product_variant_id = $1", variantID)
	require.NoError(t, err)
	for rows.Next() {
		var id uuid.UUID
		var u BusinessUnitState
		require.NoError(t, rows.Scan(&id, &u.Status))
		beforeUnits[id] = u
	}
	rows.Close()

	var beforeItemTotal, beforeItemReserved int
	err = testDB.QueryRow(ctx, "SELECT total_stock, reserved_stock FROM inventory_items WHERE product_variant_id = $1", variantID).Scan(&beforeItemTotal, &beforeItemReserved)
	require.NoError(t, err)

	var beforeReservationsCount int
	err = testDB.QueryRow(ctx, "SELECT count(*) FROM reservations WHERE product_variant_id = $1", variantID).Scan(&beforeReservationsCount)
	require.NoError(t, err)

	var beforeAllocationsCount int
	err = testDB.QueryRow(ctx, "SELECT count(*) FROM order_item_allocations WHERE inventory_unit_id IN ($1, $2)", unitMissing, unitUnexpected).Scan(&beforeAllocationsCount)
	require.NoError(t, err)

	// Perform full reconciliation count flow
	sessionID := uuid.New()
	err = repo.StartReconciliationSession(ctx, sessionID, variantID, adminID)
	require.NoError(t, err)

	// Scan unexpected unit
	_, err = repo.ProcessReconciliationScan(ctx, sessionID, zmuUnexpected, adminID)
	require.NoError(t, err)

	// Move to review
	err = repo.ChangeReconciliationStatus(ctx, sessionID, "in_progress", "review", adminID)
	require.NoError(t, err)

	// Complete count
	err = repo.ChangeReconciliationStatus(ctx, sessionID, "review", "completed", adminID)
	require.NoError(t, err)

	// Assert zero business mutations
	rows2, err := testDB.Query(ctx, "SELECT id, status FROM inventory_units WHERE product_variant_id = $1", variantID)
	require.NoError(t, err)
	for rows2.Next() {
		var id uuid.UUID
		var status string
		require.NoError(t, rows2.Scan(&id, &status))
		require.Equal(t, beforeUnits[id].Status, status, "Unit status must not be modified by reconciliation")
	}
	rows2.Close()

	var afterItemTotal, afterItemReserved int
	err = testDB.QueryRow(ctx, "SELECT total_stock, reserved_stock FROM inventory_items WHERE product_variant_id = $1", variantID).Scan(&afterItemTotal, &afterItemReserved)
	require.NoError(t, err)
	require.Equal(t, beforeItemTotal, afterItemTotal, "total_stock must not be modified by reconciliation")
	require.Equal(t, beforeItemReserved, afterItemReserved, "reserved_stock must not be modified by reconciliation")

	var afterReservationsCount int
	err = testDB.QueryRow(ctx, "SELECT count(*) FROM reservations WHERE product_variant_id = $1", variantID).Scan(&afterReservationsCount)
	require.NoError(t, err)
	require.Equal(t, beforeReservationsCount, afterReservationsCount, "Reservations count must not change")

	var afterAllocationsCount int
	err = testDB.QueryRow(ctx, "SELECT count(*) FROM order_item_allocations WHERE inventory_unit_id IN ($1, $2)", unitMissing, unitUnexpected).Scan(&afterAllocationsCount)
	require.NoError(t, err)
	require.Equal(t, beforeAllocationsCount, afterAllocationsCount, "Allocations count must not change")
}

func mustGenerateUnitCode() string {
	code, _ := supplies.GenerateUnitCode()
	return code
}
