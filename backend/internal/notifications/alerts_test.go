package notifications_test

import (
	"context"
	"errors"
	"os"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/eirfefrggrvgrgrtbg/shop-zamk/backend/internal/notifications"
	"github.com/eirfefrggrvgrgrtbg/shop-zamk/backend/internal/platform/postgres"
)

func getTestDB(t *testing.T) *postgres.Client {
	t.Helper()
	connStr := os.Getenv("TEST_DATABASE_URL")
	if connStr == "" {
		connStr = "postgres://zamk:zamk_password@localhost:5433/zamk_test?sslmode=disable"
	}
	ctx := context.Background()
	db, err := postgres.NewClient(ctx, connStr)
	if err != nil {
		t.Skipf("Database not available at %s: %v", connStr, err)
	}
	assertTestDatabase(t, db)
	t.Cleanup(db.Close)
	return db
}

func assertTestDatabase(t *testing.T, db *postgres.Client) {
	t.Helper()
	var dbName string
	err := db.Pool.QueryRow(context.Background(), "SELECT current_database()").Scan(&dbName)
	if err != nil {
		t.Fatalf("failed to query current_database: %v", err)
	}
	if dbName != "zamk_test" {
		t.Fatalf("CRITICAL DB SAFETY VIOLATION: tests must run against exactly 'zamk_test', got: %q", dbName)
	}
}

func createTestSeller(t *testing.T, db *postgres.Client, name string) uuid.UUID {
	t.Helper()
	assertTestDatabase(t, db)
	sellerID := uuid.New()
	slug := "test-seller-" + sellerID.String()
	_, err := db.Pool.Exec(context.Background(),
		"INSERT INTO sellers (id, brand_name, slug, contact_email, status, created_at, updated_at) VALUES ($1, $2, $3, 'test@example.com', 'active', now(), now())",
		sellerID, name, slug,
	)
	if err != nil {
		t.Fatalf("failed to create test seller: %v", err)
	}
	return sellerID
}

func cleanupTestFixtures(t *testing.T, db *postgres.Client, sellerIDs []uuid.UUID) {
	t.Helper()
	assertTestDatabase(t, db)
	if len(sellerIDs) == 0 {
		return
	}
	ctx := context.Background()
	_, _ = db.Pool.Exec(ctx, "DELETE FROM notifications WHERE recipient_seller_id = ANY($1)", sellerIDs)
	_, _ = db.Pool.Exec(ctx, "DELETE FROM sellers WHERE id = ANY($1)", sellerIDs)
}

func TestSchemaInvariants(t *testing.T) {
	db := getTestDB(t)

	sellerID := createTestSeller(t, db, "Schema Invariant Seller")
	t.Cleanup(func() {
		cleanupTestFixtures(t, db, []uuid.UUID{sellerID})
	})

	ctx := context.Background()

	// 1. kind = 'event' with non-null status must fail
	activeStatus := "active"
	_, err := db.Pool.Exec(ctx, `
		INSERT INTO notifications (id, recipient_seller_id, recipient_kind, type, title, body, entity_type, entity_id, created_at, kind, severity, status, dedupe_key, action_url, resolved_at)
		VALUES ($1, $2, 'seller', 'test_event', 'Title', 'Body', 'product', $1, now(), 'event', 'info', $3, NULL, NULL, NULL)
	`, uuid.New(), sellerID, activeStatus)
	if err == nil {
		t.Fatal("expected error inserting event with status != NULL, got nil")
	}

	// 2. kind = 'event' with non-null resolved_at must fail
	now := time.Now()
	_, err = db.Pool.Exec(ctx, `
		INSERT INTO notifications (id, recipient_seller_id, recipient_kind, type, title, body, entity_type, entity_id, created_at, kind, severity, status, dedupe_key, action_url, resolved_at)
		VALUES ($1, $2, 'seller', 'test_event', 'Title', 'Body', 'product', $1, now(), 'event', 'info', NULL, NULL, NULL, $3)
	`, uuid.New(), sellerID, now)
	if err == nil {
		t.Fatal("expected error inserting event with resolved_at != NULL, got nil")
	}

	// 3. kind = 'alert' with status = NULL must fail
	_, err = db.Pool.Exec(ctx, `
		INSERT INTO notifications (id, recipient_seller_id, recipient_kind, type, title, body, entity_type, entity_id, created_at, kind, severity, status, dedupe_key, action_url, resolved_at)
		VALUES ($1, $2, 'seller', 'test_alert', 'Title', 'Body', 'product', $1, now(), 'alert', 'critical', NULL, 'dedupe-1', NULL, NULL)
	`, uuid.New(), sellerID)
	if err == nil {
		t.Fatal("expected error inserting alert with status = NULL, got nil")
	}

	// 4. kind = 'alert' with status = 'active' and non-null resolved_at must fail
	_, err = db.Pool.Exec(ctx, `
		INSERT INTO notifications (id, recipient_seller_id, recipient_kind, type, title, body, entity_type, entity_id, created_at, kind, severity, status, dedupe_key, action_url, resolved_at)
		VALUES ($1, $2, 'seller', 'test_alert', 'Title', 'Body', 'product', $1, now(), 'alert', 'critical', 'active', 'dedupe-2', NULL, $3)
	`, uuid.New(), sellerID, now)
	if err == nil {
		t.Fatal("expected error inserting active alert with resolved_at != NULL, got nil")
	}

	// 5. kind = 'alert' with status = 'resolved' and resolved_at = NULL must fail
	_, err = db.Pool.Exec(ctx, `
		INSERT INTO notifications (id, recipient_seller_id, recipient_kind, type, title, body, entity_type, entity_id, created_at, kind, severity, status, dedupe_key, action_url, resolved_at)
		VALUES ($1, $2, 'seller', 'test_alert', 'Title', 'Body', 'product', $1, now(), 'alert', 'critical', 'resolved', 'dedupe-3', NULL, NULL)
	`, uuid.New(), sellerID)
	if err == nil {
		t.Fatal("expected error inserting resolved alert with resolved_at = NULL, got nil")
	}

	// 6. kind = 'alert' and recipient_kind = 'seller' with dedupe_key = NULL must fail
	_, err = db.Pool.Exec(ctx, `
		INSERT INTO notifications (id, recipient_seller_id, recipient_kind, type, title, body, entity_type, entity_id, created_at, kind, severity, status, dedupe_key, action_url, resolved_at)
		VALUES ($1, $2, 'seller', 'test_alert', 'Title', 'Body', 'product', $1, now(), 'alert', 'critical', 'active', NULL, NULL, NULL)
	`, uuid.New(), sellerID)
	if err == nil {
		t.Fatal("expected error inserting seller alert with dedupe_key = NULL, got nil")
	}

	// 7. kind = 'alert' and recipient_kind = 'seller' with empty dedupe_key must fail
	_, err = db.Pool.Exec(ctx, `
		INSERT INTO notifications (id, recipient_seller_id, recipient_kind, type, title, body, entity_type, entity_id, created_at, kind, severity, status, dedupe_key, action_url, resolved_at)
		VALUES ($1, $2, 'seller', 'test_alert', 'Title', 'Body', 'product', $1, now(), 'alert', 'critical', 'active', '   ', NULL, NULL)
	`, uuid.New(), sellerID)
	if err == nil {
		t.Fatal("expected error inserting seller alert with whitespace dedupe_key, got nil")
	}

	// 8. valid active alert must succeed
	validActiveID := uuid.New()
	_, err = db.Pool.Exec(ctx, `
		INSERT INTO notifications (id, recipient_seller_id, recipient_kind, type, title, body, entity_type, entity_id, created_at, kind, severity, status, dedupe_key, action_url, resolved_at)
		VALUES ($1, $2, 'seller', 'test_alert', 'Title', 'Body', 'product', $1, now(), 'alert', 'critical', 'active', 'valid-dedupe-1', NULL, NULL)
	`, validActiveID, sellerID)
	if err != nil {
		t.Fatalf("expected valid active alert to succeed, got: %v", err)
	}

	// 9. valid resolved alert must succeed
	validResolvedID := uuid.New()
	_, err = db.Pool.Exec(ctx, `
		INSERT INTO notifications (id, recipient_seller_id, recipient_kind, type, title, body, entity_type, entity_id, created_at, kind, severity, status, dedupe_key, action_url, resolved_at)
		VALUES ($1, $2, 'seller', 'test_alert', 'Title', 'Body', 'product', $1, now(), 'alert', 'critical', 'resolved', 'valid-dedupe-2', NULL, now())
	`, validResolvedID, sellerID)
	if err != nil {
		t.Fatalf("expected valid resolved alert to succeed, got: %v", err)
	}

	// 10. valid event must succeed
	validEventID := uuid.New()
	_, err = db.Pool.Exec(ctx, `
		INSERT INTO notifications (id, recipient_seller_id, recipient_kind, type, title, body, entity_type, entity_id, created_at, kind, severity, status, dedupe_key, action_url, resolved_at)
		VALUES ($1, $2, 'seller', 'test_event', 'Title', 'Body', 'product', $1, now(), 'event', 'info', NULL, NULL, NULL, NULL)
	`, validEventID, sellerID)
	if err != nil {
		t.Fatalf("expected valid event to succeed, got: %v", err)
	}
}


func TestSellerAlertValidation(t *testing.T) {
	db := getTestDB(t)

	sellerID := createTestSeller(t, db, "Validation Seller")
	t.Cleanup(func() {
		cleanupTestFixtures(t, db, []uuid.UUID{sellerID})
	})

	repo := notifications.NewRepository(db)
	svc := notifications.NewService(repo, nil, nil)
	ctx := context.Background()

	tx, err := db.Pool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback(ctx)

	dedupeKey := "valid-dedupe"

	// Case A: RecipientKind != seller
	_, err = svc.CreateOrUpdateActiveSellerAlertTx(ctx, tx, notifications.Notification{
		RecipientKind:     notifications.RecipientKindCustomer,
		RecipientSellerID: &sellerID,
		Kind:              notifications.KindAlert,
		Severity:          notifications.SeverityCritical,
		DedupeKey:         &dedupeKey,
	})
	if !errors.Is(err, notifications.ErrMalformedAlert) {
		t.Fatalf("expected ErrMalformedAlert for non-seller recipient_kind, got: %v", err)
	}

	// Case B: RecipientSellerID is nil
	_, err = svc.CreateOrUpdateActiveSellerAlertTx(ctx, tx, notifications.Notification{
		RecipientKind: notifications.RecipientKindSeller,
		Kind:          notifications.KindAlert,
		Severity:      notifications.SeverityCritical,
		DedupeKey:     &dedupeKey,
	})
	if !errors.Is(err, notifications.ErrMalformedAlert) {
		t.Fatalf("expected ErrMalformedAlert for nil recipient_seller_id, got: %v", err)
	}

	// Case C: DedupeKey is nil
	_, err = svc.CreateOrUpdateActiveSellerAlertTx(ctx, tx, notifications.Notification{
		RecipientKind:     notifications.RecipientKindSeller,
		RecipientSellerID: &sellerID,
		Kind:              notifications.KindAlert,
		Severity:          notifications.SeverityCritical,
		DedupeKey:         nil,
	})
	if !errors.Is(err, notifications.ErrMalformedAlert) {
		t.Fatalf("expected ErrMalformedAlert for nil dedupe_key, got: %v", err)
	}

	// Case D: DedupeKey is empty string
	emptyDedupe := "   "
	_, err = svc.CreateOrUpdateActiveSellerAlertTx(ctx, tx, notifications.Notification{
		RecipientKind:     notifications.RecipientKindSeller,
		RecipientSellerID: &sellerID,
		Kind:              notifications.KindAlert,
		Severity:          notifications.SeverityCritical,
		DedupeKey:         &emptyDedupe,
	})
	if !errors.Is(err, notifications.ErrMalformedAlert) {
		t.Fatalf("expected ErrMalformedAlert for empty dedupe_key, got: %v", err)
	}

	// Case E: Kind != alert
	_, err = svc.CreateOrUpdateActiveSellerAlertTx(ctx, tx, notifications.Notification{
		RecipientKind:     notifications.RecipientKindSeller,
		RecipientSellerID: &sellerID,
		Kind:              notifications.KindEvent,
		Severity:          notifications.SeverityCritical,
		DedupeKey:         &dedupeKey,
	})
	if !errors.Is(err, notifications.ErrMalformedAlert) {
		t.Fatalf("expected ErrMalformedAlert for kind != alert, got: %v", err)
	}

	// Case F: Invalid severity
	_, err = svc.CreateOrUpdateActiveSellerAlertTx(ctx, tx, notifications.Notification{
		RecipientKind:     notifications.RecipientKindSeller,
		RecipientSellerID: &sellerID,
		Kind:              notifications.KindAlert,
		Severity:          "extreme",
		DedupeKey:         &dedupeKey,
	})
	if !errors.Is(err, notifications.ErrMalformedAlert) {
		t.Fatalf("expected ErrMalformedAlert for invalid severity, got: %v", err)
	}
}

func TestAlertsUpsertSemantics(t *testing.T) {
	db := getTestDB(t)

	sellerID := createTestSeller(t, db, "Upsert Seller")
	t.Cleanup(func() {
		cleanupTestFixtures(t, db, []uuid.UUID{sellerID})
	})

	repo := notifications.NewRepository(db)
	ctx := context.Background()

	dedupeKey := "stock_critical_upsert_test"
	actionURL1 := "/inventory?variant=1"
	actionURL2 := "/inventory?variant=2"

	n1 := notifications.Notification{
		ID:                uuid.New(),
		RecipientSellerID: &sellerID,
		RecipientKind:     notifications.RecipientKindSeller,
		Type:              "stock_critical",
		Title:             "Initial Critical Stock",
		Body:              "Initial body text",
		Kind:              notifications.KindAlert,
		Severity:          notifications.SeverityWarning,
		DedupeKey:         &dedupeKey,
		ActionURL:         &actionURL1,
		Metadata:          map[string]interface{}{"count": float64(1)},
		CreatedAt:         time.Now().Add(-1 * time.Hour).Truncate(time.Microsecond),
	}

	// 1. First insert
	tx1, err := db.Pool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	id1, err := repo.UpsertActiveSellerAlertTx(ctx, tx1, &n1)
	if err != nil {
		tx1.Rollback(ctx)
		t.Fatalf("first upsert failed: %v", err)
	}
	if err := tx1.Commit(ctx); err != nil {
		t.Fatal(err)
	}
	if id1 != n1.ID {
		t.Fatalf("expected ID %s on first insert, got %s", n1.ID, id1)
	}

	// Mark it read to prove read_at is preserved
	if err := repo.MarkRead(ctx, id1, nil, &sellerID, notifications.RecipientKindSeller); err != nil {
		t.Fatalf("MarkRead failed: %v", err)
	}

	// 2. Second insert with changed title, body, severity, action_url, metadata
	n2 := notifications.Notification{
		ID:                uuid.New(), // should be ignored in favor of id1
		RecipientSellerID: &sellerID,
		RecipientKind:     notifications.RecipientKindSeller,
		Type:              "stock_critical",
		Title:             "Refreshed Critical Stock",
		Body:              "Refreshed body text",
		Kind:              notifications.KindAlert,
		Severity:          notifications.SeverityCritical,
		DedupeKey:         &dedupeKey,
		ActionURL:         &actionURL2,
		Metadata:          map[string]interface{}{"count": float64(2)},
		CreatedAt:         time.Now(), // should not overwrite original created_at
	}

	tx2, err := db.Pool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	id2, err := repo.UpsertActiveSellerAlertTx(ctx, tx2, &n2)
	if err != nil {
		tx2.Rollback(ctx)
		t.Fatalf("second upsert failed: %v", err)
	}
	if err := tx2.Commit(ctx); err != nil {
		t.Fatal(err)
	}

	if id2 != id1 {
		t.Fatalf("canonical upsert requirement failed: expected existing ID %s, got %s", id1, id2)
	}

	// 3. Verify DB record: refreshed content, preserved identity/read_at/created_at
	var savedTitle, savedBody, savedSeverity, savedActionURL, savedStatus string
	var savedCreatedAt time.Time
	var savedReadAt, savedResolvedAt *time.Time
	var savedMetadata map[string]interface{}

	err = db.Pool.QueryRow(ctx, `
		SELECT title, body, severity, action_url, status, created_at, read_at, resolved_at, metadata
		FROM notifications
		WHERE id = $1
	`, id1).Scan(&savedTitle, &savedBody, &savedSeverity, &savedActionURL, &savedStatus, &savedCreatedAt, &savedReadAt, &savedResolvedAt, &savedMetadata)
	if err != nil {
		t.Fatalf("failed to query alert row: %v", err)
	}

	if savedTitle != "Refreshed Critical Stock" {
		t.Errorf("expected refreshed title 'Refreshed Critical Stock', got: %q", savedTitle)
	}
	if savedBody != "Refreshed body text" {
		t.Errorf("expected refreshed body 'Refreshed body text', got: %q", savedBody)
	}
	if savedSeverity != notifications.SeverityCritical {
		t.Errorf("expected refreshed severity 'critical', got: %q", savedSeverity)
	}
	if savedActionURL != actionURL2 {
		t.Errorf("expected refreshed action_url %q, got: %q", actionURL2, savedActionURL)
	}
	if savedMetadata["count"] != float64(2) {
		t.Errorf("expected refreshed metadata count 2, got: %v", savedMetadata["count"])
	}
	if savedStatus != notifications.StatusActive {
		t.Errorf("expected status 'active', got: %q", savedStatus)
	}
	if savedReadAt == nil {
		t.Error("expected read_at to be preserved, got nil")
	}
	if savedResolvedAt != nil {
		t.Errorf("expected resolved_at to be nil for active alert, got: %v", savedResolvedAt)
	}

	// Exactly one active row exists
	var count int
	err = db.Pool.QueryRow(ctx, `
		SELECT count(*) FROM notifications
		WHERE recipient_seller_id = $1 AND dedupe_key = $2 AND status = 'active'
	`, sellerID, dedupeKey).Scan(&count)
	if err != nil {
		t.Fatal(err)
	}
	if count != 1 {
		t.Fatalf("expected exactly 1 active row for seller + dedupe_key, got %d", count)
	}
}

func TestRealConcurrency(t *testing.T) {
	db := getTestDB(t)

	sellerID := createTestSeller(t, db, "Concurrency Seller")
	t.Cleanup(func() {
		cleanupTestFixtures(t, db, []uuid.UUID{sellerID})
	})

	repo := notifications.NewRepository(db)
	ctx := context.Background()

	dedupeKey := "concurrency_dedupe_" + uuid.New().String()

	n1 := notifications.Notification{
		ID:                uuid.New(),
		RecipientSellerID: &sellerID,
		RecipientKind:     notifications.RecipientKindSeller,
		Type:              "stock_critical",
		Title:             "Goroutine 1 Alert",
		Body:              "Body from G1",
		Kind:              notifications.KindAlert,
		Severity:          notifications.SeverityCritical,
		DedupeKey:         &dedupeKey,
		Metadata:          map[string]interface{}{"worker": 1},
		CreatedAt:         time.Now(),
	}

	n2 := notifications.Notification{
		ID:                uuid.New(),
		RecipientSellerID: &sellerID,
		RecipientKind:     notifications.RecipientKindSeller,
		Type:              "stock_critical",
		Title:             "Goroutine 2 Alert",
		Body:              "Body from G2",
		Kind:              notifications.KindAlert,
		Severity:          notifications.SeverityCritical,
		DedupeKey:         &dedupeKey,
		Metadata:          map[string]interface{}{"worker": 2},
		CreatedAt:         time.Now(),
	}

	startGate := make(chan struct{})
	var wg sync.WaitGroup
	wg.Add(2)

	var id1, id2 uuid.UUID
	var err1, err2 error

	go func() {
		defer wg.Done()
		tx, err := db.Pool.Begin(ctx)
		if err != nil {
			err1 = err
			return
		}
		defer tx.Rollback(ctx)

		<-startGate // synchronize execution start

		id, err := repo.UpsertActiveSellerAlertTx(ctx, tx, &n1)
		if err != nil {
			err1 = err
			return
		}
		if err := tx.Commit(ctx); err != nil {
			err1 = err
			return
		}
		id1 = id
	}()

	go func() {
		defer wg.Done()
		tx, err := db.Pool.Begin(ctx)
		if err != nil {
			err2 = err
			return
		}
		defer tx.Rollback(ctx)

		<-startGate // synchronize execution start

		id, err := repo.UpsertActiveSellerAlertTx(ctx, tx, &n2)
		if err != nil {
			err2 = err
			return
		}
		if err := tx.Commit(ctx); err != nil {
			err2 = err
			return
		}
		id2 = id
	}()

	// Fire both goroutines simultaneously
	close(startGate)
	wg.Wait()

	if err1 != nil {
		t.Fatalf("goroutine 1 failed: %v", err1)
	}
	if err2 != nil {
		t.Fatalf("goroutine 2 failed: %v", err2)
	}

	if id1 == uuid.Nil || id2 == uuid.Nil {
		t.Fatalf("expected non-nil IDs, got id1=%s, id2=%s", id1, id2)
	}

	// Both operations must return the SAME canonical active alert ID
	if id1 != id2 {
		t.Fatalf("concurrency violation: expected identical canonical ID, got id1=%s, id2=%s", id1, id2)
	}

	// Exactly one ACTIVE row must exist in DB
	var count int
	err := db.Pool.QueryRow(ctx, `
		SELECT count(*) FROM notifications
		WHERE recipient_seller_id = $1 AND dedupe_key = $2 AND status = 'active'
	`, sellerID, dedupeKey).Scan(&count)
	if err != nil {
		t.Fatal(err)
	}
	if count != 1 {
		t.Fatalf("expected exactly 1 active row for seller + dedupe_key, got %d", count)
	}
}

func TestReadVsResolvedIndependent(t *testing.T) {
	db := getTestDB(t)

	sellerID := createTestSeller(t, db, "Read vs Resolved Seller")
	t.Cleanup(func() {
		cleanupTestFixtures(t, db, []uuid.UUID{sellerID})
	})

	repo := notifications.NewRepository(db)
	ctx := context.Background()

	// --- Case A: active + unread -> mark read -> active + read -> resolve -> resolved + read ---
	dedupeA := "read_then_resolve_" + uuid.New().String()
	nA := notifications.Notification{
		ID:                uuid.New(),
		RecipientSellerID: &sellerID,
		RecipientKind:     notifications.RecipientKindSeller,
		Type:              "stock_critical",
		Title:             "Alert A",
		Body:              "Body A",
		Kind:              notifications.KindAlert,
		Severity:          notifications.SeverityCritical,
		DedupeKey:         &dedupeA,
		CreatedAt:         time.Now(),
	}

	txA, err := db.Pool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	idA, err := repo.UpsertActiveSellerAlertTx(ctx, txA, &nA)
	if err != nil {
		t.Fatal(err)
	}
	if err := txA.Commit(ctx); err != nil {
		t.Fatal(err)
	}

	// Initial: active + unread
	var statusA string
	var readAtA, resolvedAtA *time.Time
	err = db.Pool.QueryRow(ctx, "SELECT status, read_at, resolved_at FROM notifications WHERE id = $1", idA).Scan(&statusA, &readAtA, &resolvedAtA)
	if err != nil {
		t.Fatal(err)
	}
	if statusA != "active" || readAtA != nil || resolvedAtA != nil {
		t.Fatalf("expected initial active + unread, got status=%s, read_at=%v, resolved_at=%v", statusA, readAtA, resolvedAtA)
	}

	// Step: mark read -> active + read
	if err := repo.MarkRead(ctx, idA, nil, &sellerID, notifications.RecipientKindSeller); err != nil {
		t.Fatal(err)
	}
	err = db.Pool.QueryRow(ctx, "SELECT status, read_at, resolved_at FROM notifications WHERE id = $1", idA).Scan(&statusA, &readAtA, &resolvedAtA)
	if err != nil {
		t.Fatal(err)
	}
	if statusA != "active" || readAtA == nil || resolvedAtA != nil {
		t.Fatalf("expected active + read, got status=%s, read_at=%v, resolved_at=%v", statusA, readAtA, resolvedAtA)
	}
	savedReadAt := *readAtA

	// Step: resolve -> resolved + read (read_at UNCHANGED)
	txAResolve, err := db.Pool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if err := repo.ResolveSellerAlertTx(ctx, txAResolve, sellerID, dedupeA); err != nil {
		t.Fatal(err)
	}
	if err := txAResolve.Commit(ctx); err != nil {
		t.Fatal(err)
	}

	err = db.Pool.QueryRow(ctx, "SELECT status, read_at, resolved_at FROM notifications WHERE id = $1", idA).Scan(&statusA, &readAtA, &resolvedAtA)
	if err != nil {
		t.Fatal(err)
	}
	if statusA != "resolved" {
		t.Fatalf("expected status 'resolved', got %s", statusA)
	}
	if resolvedAtA == nil {
		t.Fatal("expected resolved_at to be set on resolution")
	}
	if readAtA == nil || !readAtA.Equal(savedReadAt) {
		t.Fatalf("READ != RESOLVED contract violated: read_at changed during resolution (was %v, now %v)", savedReadAt, readAtA)
	}

	// Step: resolve twice is idempotent
	txAIdempotent, err := db.Pool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if err := repo.ResolveSellerAlertTx(ctx, txAIdempotent, sellerID, dedupeA); err != nil {
		t.Fatalf("second resolution must be idempotent, got error: %v", err)
	}
	if err := txAIdempotent.Commit(ctx); err != nil {
		t.Fatal(err)
	}

	// --- Case B: active + unread -> resolve -> resolved + unread ---
	dedupeB := "unread_then_resolve_" + uuid.New().String()
	nB := notifications.Notification{
		ID:                uuid.New(),
		RecipientSellerID: &sellerID,
		RecipientKind:     notifications.RecipientKindSeller,
		Type:              "stock_critical",
		Title:             "Alert B",
		Body:              "Body B",
		Kind:              notifications.KindAlert,
		Severity:          notifications.SeverityCritical,
		DedupeKey:         &dedupeB,
		CreatedAt:         time.Now(),
	}

	txB, err := db.Pool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	idB, err := repo.UpsertActiveSellerAlertTx(ctx, txB, &nB)
	if err != nil {
		t.Fatal(err)
	}
	if err := txB.Commit(ctx); err != nil {
		t.Fatal(err)
	}

	// Resolve directly without reading
	txBResolve, err := db.Pool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if err := repo.ResolveSellerAlertTx(ctx, txBResolve, sellerID, dedupeB); err != nil {
		t.Fatal(err)
	}
	if err := txBResolve.Commit(ctx); err != nil {
		t.Fatal(err)
	}

	var statusB string
	var readAtB, resolvedAtB *time.Time
	err = db.Pool.QueryRow(ctx, "SELECT status, read_at, resolved_at FROM notifications WHERE id = $1", idB).Scan(&statusB, &readAtB, &resolvedAtB)
	if err != nil {
		t.Fatal(err)
	}
	if statusB != "resolved" {
		t.Fatalf("expected status 'resolved', got %s", statusB)
	}
	if resolvedAtB == nil {
		t.Fatal("expected resolved_at to be set")
	}
	if readAtB != nil {
		t.Fatalf("READ != RESOLVED contract violated: resolution must not mark alert as read, got read_at=%v", readAtB)
	}
}

func TestResolutionAndRecurrence(t *testing.T) {
	db := getTestDB(t)

	sellerID := createTestSeller(t, db, "Recurrence Seller")
	t.Cleanup(func() {
		cleanupTestFixtures(t, db, []uuid.UUID{sellerID})
	})

	repo := notifications.NewRepository(db)
	ctx := context.Background()

	dedupeKey := "recurrence_test_" + uuid.New().String()

	// 1. First occurrence of problem
	n1 := notifications.Notification{
		ID:                uuid.New(),
		RecipientSellerID: &sellerID,
		RecipientKind:     notifications.RecipientKindSeller,
		Type:              "stock_critical",
		Title:             "Critical Stock - Outage 1",
		Body:              "Free stock < 2",
		Kind:              notifications.KindAlert,
		Severity:          notifications.SeverityCritical,
		DedupeKey:         &dedupeKey,
		CreatedAt:         time.Now(),
	}

	tx1, err := db.Pool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	id1, err := repo.UpsertActiveSellerAlertTx(ctx, tx1, &n1)
	if err != nil {
		t.Fatal(err)
	}
	if err := tx1.Commit(ctx); err != nil {
		t.Fatal(err)
	}

	// 2. Problem resolved (e.g. FBO receiving occurred)
	txResolve, err := db.Pool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if err := repo.ResolveSellerAlertTx(ctx, txResolve, sellerID, dedupeKey); err != nil {
		t.Fatal(err)
	}
	if err := txResolve.Commit(ctx); err != nil {
		t.Fatal(err)
	}

	// 3. Weeks later, problem recurs (free stock drops < 2 again)
	n2 := notifications.Notification{
		ID:                uuid.New(),
		RecipientSellerID: &sellerID,
		RecipientKind:     notifications.RecipientKindSeller,
		Type:              "stock_critical",
		Title:             "Critical Stock - Outage 2",
		Body:              "Free stock < 2 again",
		Kind:              notifications.KindAlert,
		Severity:          notifications.SeverityCritical,
		DedupeKey:         &dedupeKey,
		CreatedAt:         time.Now(),
	}

	tx2, err := db.Pool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	id2, err := repo.UpsertActiveSellerAlertTx(ctx, tx2, &n2)
	if err != nil {
		t.Fatalf("recurrence after resolution must succeed, got: %v", err)
	}
	if err := tx2.Commit(ctx); err != nil {
		t.Fatal(err)
	}

	// Recurrence must create a NEW active alert occurrence
	if id2 == id1 {
		t.Fatalf("expected new row/ID %s on recurrence after resolution, got old ID %s", n2.ID, id2)
	}

	// Query all alerts for dedupe key: exactly 2 rows (1 resolved, 1 active)
	rows, err := db.Pool.Query(ctx, `
		SELECT id, status FROM notifications
		WHERE recipient_seller_id = $1 AND dedupe_key = $2
		ORDER BY created_at ASC
	`, sellerID, dedupeKey)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()

	type alertSummary struct {
		id     uuid.UUID
		status string
	}
	var summaries []alertSummary
	for rows.Next() {
		var s alertSummary
		if err := rows.Scan(&s.id, &s.status); err != nil {
			t.Fatal(err)
		}
		summaries = append(summaries, s)
	}

	if len(summaries) != 2 {
		t.Fatalf("expected exactly 2 occurrences, got %d", len(summaries))
	}
	if summaries[0].id != id1 || summaries[0].status != "resolved" {
		t.Errorf("expected first occurrence to be resolved id1=%s, got: %+v", id1, summaries[0])
	}
	if summaries[1].id != id2 || summaries[1].status != "active" {
		t.Errorf("expected second occurrence to be active id2=%s, got: %+v", id2, summaries[1])
	}
}

func TestSellerAVsSellerB(t *testing.T) {
	db := getTestDB(t)

	sellerA := createTestSeller(t, db, "Seller Alpha")
	sellerB := createTestSeller(t, db, "Seller Beta")
	t.Cleanup(func() {
		cleanupTestFixtures(t, db, []uuid.UUID{sellerA, sellerB})
	})

	repo := notifications.NewRepository(db)
	ctx := context.Background()

	sameDedupeKey := "shared_logical_dedupe_product_123"

	// Seller A creates alert
	nA := notifications.Notification{
		ID:                uuid.New(),
		RecipientSellerID: &sellerA,
		RecipientKind:     notifications.RecipientKindSeller,
		Type:              "stock_critical",
		Title:             "Alpha Stock Alert",
		Body:              "Alpha stock low",
		Kind:              notifications.KindAlert,
		Severity:          notifications.SeverityCritical,
		DedupeKey:         &sameDedupeKey,
		CreatedAt:         time.Now(),
	}
	txA, err := db.Pool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	idA, err := repo.UpsertActiveSellerAlertTx(ctx, txA, &nA)
	if err != nil {
		t.Fatalf("Seller A insert failed: %v", err)
	}
	if err := txA.Commit(ctx); err != nil {
		t.Fatal(err)
	}

	// Seller B creates alert with the SAME dedupe key
	nB := notifications.Notification{
		ID:                uuid.New(),
		RecipientSellerID: &sellerB,
		RecipientKind:     notifications.RecipientKindSeller,
		Type:              "stock_critical",
		Title:             "Beta Stock Alert",
		Body:              "Beta stock low",
		Kind:              notifications.KindAlert,
		Severity:          notifications.SeverityCritical,
		DedupeKey:         &sameDedupeKey,
		CreatedAt:         time.Now(),
	}
	txB, err := db.Pool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	idB, err := repo.UpsertActiveSellerAlertTx(ctx, txB, &nB)
	if err != nil {
		t.Fatalf("Seller B insert with same dedupe_key failed: %v", err)
	}
	if err := txB.Commit(ctx); err != nil {
		t.Fatal(err)
	}

	// Both alerts must be independent and distinct
	if idA == idB {
		t.Fatalf("Seller A and Seller B must have independent alerts, got same ID: %s", idA)
	}

	// Both must be active simultaneously
	var statusA, statusB string
	if err := db.Pool.QueryRow(ctx, "SELECT status FROM notifications WHERE id = $1", idA).Scan(&statusA); err != nil {
		t.Fatal(err)
	}
	if err := db.Pool.QueryRow(ctx, "SELECT status FROM notifications WHERE id = $1", idB).Scan(&statusB); err != nil {
		t.Fatal(err)
	}
	if statusA != "active" || statusB != "active" {
		t.Fatalf("expected both alerts to be active, got A=%s, B=%s", statusA, statusB)
	}
}

func TestEventBackwardCompatibility(t *testing.T) {
	db := getTestDB(t)

	sellerID := createTestSeller(t, db, "Event Backward Compat Seller")
	t.Cleanup(func() {
		cleanupTestFixtures(t, db, []uuid.UUID{sellerID})
	})

	repo := notifications.NewRepository(db)
	svc := notifications.NewService(repo, nil, nil)
	ctx := context.Background()

	// Existing legacy producer creates event without kind/severity
	eventNotif := notifications.Notification{
		ID:                uuid.New(),
		RecipientSellerID: &sellerID,
		RecipientKind:     notifications.RecipientKindSeller,
		Type:              notifications.TypeProductApproved,
		Title:             "Товар одобрен",
		Body:              "Ваш товар прошел модерацию",
		EntityType:        "product",
		EntityID:          uuid.New(),
		Metadata:          map[string]interface{}{"sku": "TSHIRT-1"},
		CreatedAt:         time.Now(),
	}

	tx, err := db.Pool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if err := svc.CreateNotificationTx(ctx, tx, eventNotif); err != nil {
		t.Fatalf("legacy producer CreateNotificationTx failed: %v", err)
	}
	if err := tx.Commit(ctx); err != nil {
		t.Fatal(err)
	}

	// Verify defaults injected
	var kind, severity string
	var status *string
	var resolvedAt *time.Time
	err = db.Pool.QueryRow(ctx, `
		SELECT kind, severity, status, resolved_at
		FROM notifications
		WHERE id = $1
	`, eventNotif.ID).Scan(&kind, &severity, &status, &resolvedAt)
	if err != nil {
		t.Fatalf("failed to query event notification: %v", err)
	}

	if kind != notifications.KindEvent {
		t.Errorf("expected default kind 'event', got %q", kind)
	}
	if severity != notifications.SeverityInfo {
		t.Errorf("expected default severity 'info', got %q", severity)
	}
	if status != nil {
		t.Errorf("expected event status = nil, got %v", *status)
	}
	if resolvedAt != nil {
		t.Errorf("expected event resolved_at = nil, got %v", resolvedAt)
	}

	// Verify readable and markable as read
	if err := repo.MarkRead(ctx, eventNotif.ID, nil, &sellerID, notifications.RecipientKindSeller); err != nil {
		t.Fatalf("marking event as read failed: %v", err)
	}

	list, total, err := repo.ListNotifications(ctx, nil, &sellerID, notifications.RecipientKindSeller, 10, 0)
	if err != nil {
		t.Fatal(err)
	}
	if total != 1 || len(list) != 1 {
		t.Fatalf("expected 1 notification in list, got total=%d, len=%d", total, len(list))
	}
	if list[0].ReadAt == nil {
		t.Error("expected ReadAt to be populated after MarkRead")
	}
	if list[0].Kind != notifications.KindEvent {
		t.Errorf("expected kind 'event' in listed item, got %s", list[0].Kind)
	}
	if list[0].Status != nil {
		t.Errorf("expected status nil in listed item, got %v", *list[0].Status)
	}
}
