package fulfillment

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

func (r *Repository) GetFulfillmentByCode(ctx context.Context, codeOrToken string) (*Fulfillment, error) {
	query := `
		SELECT 
			f.id, f.order_id, f.seller_id, f.status, f.subtotal_cents, f.commission_bps, f.seller_amount_cents, f.created_at, f.updated_at,
			f.receiving_code, f.receiving_qr_token, f.packed_at, f.accepted_at, f.accepted_by_staff_id, f.receiving_result, f.discrepancy_reason, f.discrepancy_comment, f.discrepancy_at,
			s.status as shipment_status, s.id as shipment_id,
			o.delivery_address, o.customer_name, o.customer_phone, o.order_number,
			sel.brand_name as seller_name
		FROM order_fulfillments f
		JOIN orders o ON o.id = f.order_id
		LEFT JOIN sellers sel ON sel.id = f.seller_id
		LEFT JOIN shipments s ON (s.fulfillment_id = f.id) OR (s.fulfillment_id IS NULL AND s.order_id = f.order_id AND (SELECT COUNT(*) FROM order_fulfillments WHERE order_id = f.order_id) = 1)
		WHERE f.receiving_code = $1 OR f.receiving_qr_token = $1 OR f.id::text = $1
	`
	var f Fulfillment
	err := r.db.QueryRow(ctx, query, codeOrToken).Scan(
		&f.ID, &f.OrderID, &f.SellerID, &f.Status, &f.SubtotalCents, &f.CommissionBps, &f.SellerAmountCents, &f.CreatedAt, &f.UpdatedAt,
		&f.ReceivingCode, &f.ReceivingQRToken, &f.PackedAt, &f.AcceptedAt, &f.AcceptedByStaffID, &f.ReceivingResult, &f.DiscrepancyReason, &f.DiscrepancyComment, &f.DiscrepancyAt,
		&f.ShipmentStatus, &f.ShipmentID,
		&f.DeliveryAddress, &f.CustomerName, &f.CustomerPhone, &f.OrderNumber,
		&f.SellerName,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrFulfillmentNotFound
		}
		return nil, err
	}

	f.Items, err = r.GetFulfillmentItems(ctx, f.ID)
	if err != nil {
		return nil, err
	}
	return &f, nil
}

func generateSecureToken() string {
	b := make([]byte, 16)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}

func (r *Repository) EnsureReceivingCodeTx(ctx context.Context, tx pgx.Tx, fulfillmentID uuid.UUID) (string, string, error) {
	var code, token *string
	err := tx.QueryRow(ctx, `SELECT receiving_code, receiving_qr_token FROM order_fulfillments WHERE id = $1 FOR UPDATE`, fulfillmentID).Scan(&code, &token)
	if err != nil {
		return "", "", err
	}

	newCode := ""
	if code != nil && *code != "" {
		newCode = *code
	} else {
		var seqVal int64
		err := tx.QueryRow(ctx, `SELECT nextval('fulfillment_receiving_code_seq')`).Scan(&seqVal)
		if err != nil {
			return "", "", err
		}
		newCode = fmt.Sprintf("FUL-%d-%06d", time.Now().Year(), seqVal)
	}

	newToken := ""
	if token != nil && *token != "" {
		newToken = *token
	} else {
		newToken = generateSecureToken()
	}

	now := time.Now()
	_, err = tx.Exec(ctx, `
		UPDATE order_fulfillments 
		SET receiving_code = $1, receiving_qr_token = $2, packed_at = COALESCE(packed_at, $3), updated_at = now()
		WHERE id = $4
	`, newCode, newToken, now, fulfillmentID)
	if err != nil {
		return "", "", err
	}

	return newCode, newToken, nil
}

func (r *Repository) GetActiveReceivingSession(ctx context.Context, fulfillmentID uuid.UUID) (*ReceivingSession, error) {
	query := `
		SELECT id, fulfillment_id, status, version, started_at, started_by_staff_id, completed_at, created_at, updated_at
		FROM fulfillment_receiving_sessions
		WHERE fulfillment_id = $1 AND status = 'active'
	`
	var sess ReceivingSession
	err := r.db.QueryRow(ctx, query, fulfillmentID).Scan(
		&sess.ID, &sess.FulfillmentID, &sess.Status, &sess.Version, &sess.StartedAt, &sess.StartedByStaffID, &sess.CompletedAt, &sess.CreatedAt, &sess.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrReceivingNotStarted
		}
		return nil, err
	}

	items, err := r.GetReceivingItems(ctx, sess.ID)
	if err != nil {
		return nil, err
	}
	sess.Items = items
	sess.CanConfirm = checkSessionCanConfirm(items)

	return &sess, nil
}

func (r *Repository) GetReceivingItems(ctx context.Context, sessionID uuid.UUID) ([]ReceivingItem, error) {
	query := `
		SELECT id, session_id, fulfillment_item_id, variant_id, sku, barcode, product_title, expected_quantity, scanned_quantity, damaged_quantity, unexpected_quantity, created_at, updated_at
		FROM fulfillment_receiving_items
		WHERE session_id = $1
		ORDER BY created_at ASC
	`
	rows, err := r.db.Query(ctx, query, sessionID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var items []ReceivingItem
	for rows.Next() {
		var it ReceivingItem
		err := rows.Scan(
			&it.ID, &it.SessionID, &it.FulfillmentItemID, &it.VariantID, &it.SKU, &it.Barcode, &it.ProductTitle, &it.ExpectedQuantity, &it.ScannedQuantity, &it.DamagedQuantity, &it.UnexpectedQuantity, &it.CreatedAt, &it.UpdatedAt,
		)
		if err != nil {
			return nil, err
		}
		items = append(items, it)
	}
	return items, nil
}

func (r *Repository) GetReceivingItemsTx(ctx context.Context, tx pgx.Tx, sessionID uuid.UUID) ([]ReceivingItem, error) {
	query := `
		SELECT id, session_id, fulfillment_item_id, variant_id, sku, barcode, product_title, expected_quantity, scanned_quantity, damaged_quantity, unexpected_quantity, created_at, updated_at
		FROM fulfillment_receiving_items
		WHERE session_id = $1
		ORDER BY created_at ASC
	`
	rows, err := tx.Query(ctx, query, sessionID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var items []ReceivingItem
	for rows.Next() {
		var it ReceivingItem
		err := rows.Scan(
			&it.ID, &it.SessionID, &it.FulfillmentItemID, &it.VariantID, &it.SKU, &it.Barcode, &it.ProductTitle, &it.ExpectedQuantity, &it.ScannedQuantity, &it.DamagedQuantity, &it.UnexpectedQuantity, &it.CreatedAt, &it.UpdatedAt,
		)
		if err != nil {
			return nil, err
		}
		items = append(items, it)
	}
	return items, nil
}

func checkSessionCanConfirm(items []ReceivingItem) bool {
	if len(items) == 0 {
		return false
	}
	for _, it := range items {
		if it.ScannedQuantity != it.ExpectedQuantity || it.DamagedQuantity > 0 || it.UnexpectedQuantity > 0 {
			return false
		}
	}
	return true
}

func (r *Repository) StartReceivingSessionTx(ctx context.Context, tx pgx.Tx, fulfillmentID uuid.UUID, staffID *uuid.UUID) (*ReceivingSession, error) {
	var status string
	err := tx.QueryRow(ctx, `SELECT status FROM order_fulfillments WHERE id = $1 FOR UPDATE`, fulfillmentID).Scan(&status)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrFulfillmentNotFound
		}
		return nil, err
	}

	if status == "accepted" || status == "discrepancy" {
		return nil, ErrFulfillmentAlreadyReceived
	}
	if status != "packed" && status != "assembling" {
		return nil, ErrFulfillmentNotPacked
	}

	// Query existing active session
	var sess ReceivingSession
	err = tx.QueryRow(ctx, `
		SELECT id, fulfillment_id, status, version, started_at, started_by_staff_id, completed_at, created_at, updated_at
		FROM fulfillment_receiving_sessions
		WHERE fulfillment_id = $1 AND status = 'active'
		FOR UPDATE
	`, fulfillmentID).Scan(
		&sess.ID, &sess.FulfillmentID, &sess.Status, &sess.Version, &sess.StartedAt, &sess.StartedByStaffID, &sess.CompletedAt, &sess.CreatedAt, &sess.UpdatedAt,
	)

	if err == nil {
		items, err := r.GetReceivingItemsTx(ctx, tx, sess.ID)
		if err != nil {
			return nil, err
		}
		sess.Items = items
		sess.CanConfirm = checkSessionCanConfirm(items)
		return &sess, nil
	} else if !errors.Is(err, pgx.ErrNoRows) {
		return nil, err
	}

	// Create new active session
	sessionID := uuid.New()
	now := time.Now()
	_, err = tx.Exec(ctx, `
		INSERT INTO fulfillment_receiving_sessions (id, fulfillment_id, status, version, started_at, started_by_staff_id, created_at, updated_at)
		VALUES ($1, $2, 'active', 1, $3, $4, $3, $3)
	`, sessionID, fulfillmentID, now, staffID)
	if err != nil {
		return nil, err
	}

	// Populate receiving items from order_items & product_variants
	itemRows, err := tx.Query(ctx, `
		SELECT oi.id, oi.product_variant_id, oi.sku, pv.barcode, oi.title, oi.quantity
		FROM order_items oi
		LEFT JOIN product_variants pv ON pv.id = oi.product_variant_id
		WHERE oi.order_fulfillment_id = $1
	`, fulfillmentID)
	if err != nil {
		return nil, err
	}
	defer itemRows.Close()

	type rawOrderItem struct {
		orderItemID uuid.UUID
		variantID   uuid.UUID
		sku         string
		barcode     *string
		title       string
		qty         int
	}

	var rawItems []rawOrderItem
	for itemRows.Next() {
		var it rawOrderItem
		if err := itemRows.Scan(&it.orderItemID, &it.variantID, &it.sku, &it.barcode, &it.title, &it.qty); err != nil {
			itemRows.Close()
			return nil, err
		}
		rawItems = append(rawItems, it)
	}
	itemRows.Close()

	var receivingItems []ReceivingItem
	for _, it := range rawItems {
		recItemID := uuid.New()
		_, err := tx.Exec(ctx, `
			INSERT INTO fulfillment_receiving_items 
			(id, session_id, fulfillment_item_id, variant_id, sku, barcode, product_title, expected_quantity, scanned_quantity, created_at, updated_at)
			VALUES ($1, $2, $3, $4, $5, $6, $7, $8, 0, $9, $9)
		`, recItemID, sessionID, it.orderItemID, it.variantID, it.sku, it.barcode, it.title, it.qty, now)
		if err != nil {
			return nil, err
		}

		receivingItems = append(receivingItems, ReceivingItem{
			ID:                 recItemID,
			SessionID:          sessionID,
			FulfillmentItemID:  &it.orderItemID,
			VariantID:          &it.variantID,
			SKU:                it.sku,
			Barcode:            it.barcode,
			ProductTitle:       it.title,
			ExpectedQuantity:   it.qty,
			ScannedQuantity:    0,
			DamagedQuantity:    0,
			UnexpectedQuantity: 0,
			CreatedAt:          now,
			UpdatedAt:          now,
		})
	}

	sess = ReceivingSession{
		ID:               sessionID,
		FulfillmentID:    fulfillmentID,
		Status:           "active",
		Version:          1,
		StartedAt:        now,
		StartedByStaffID: staffID,
		CreatedAt:        now,
		UpdatedAt:        now,
		Items:            receivingItems,
		CanConfirm:       checkSessionCanConfirm(receivingItems),
	}
	return &sess, nil
}

func (r *Repository) ScanReceivingItemTx(ctx context.Context, tx pgx.Tx, fulfillmentID uuid.UUID, barcode string, expectedVersion int, idempotencyKey string) (*ReceivingSession, error) {
	// SELECT FOR UPDATE active session
	var sess ReceivingSession
	err := tx.QueryRow(ctx, `
		SELECT id, fulfillment_id, status, version, started_at, started_by_staff_id, completed_at, created_at, updated_at
		FROM fulfillment_receiving_sessions
		WHERE fulfillment_id = $1 AND status = 'active'
		FOR UPDATE
	`, fulfillmentID).Scan(
		&sess.ID, &sess.FulfillmentID, &sess.Status, &sess.Version, &sess.StartedAt, &sess.StartedByStaffID, &sess.CompletedAt, &sess.CreatedAt, &sess.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrReceivingNotStarted
		}
		return nil, err
	}

	if sess.Version != expectedVersion {
		return nil, ErrVersionConflict
	}

	// Check idempotency key if provided
	if idempotencyKey != "" {
		var exists bool
		err := tx.QueryRow(ctx, `
			SELECT EXISTS(SELECT 1 FROM fulfillment_receiving_scans WHERE session_id = $1 AND idempotency_key = $2)
		`, sess.ID, idempotencyKey).Scan(&exists)
		if err != nil {
			return nil, err
		}
		if exists {
			// Idempotent retry: return current session state without double-increment
			items, err := r.GetReceivingItemsTx(ctx, tx, sess.ID)
			if err != nil {
				return nil, err
			}
			sess.Items = items
			sess.CanConfirm = checkSessionCanConfirm(items)
			return &sess, nil
		}
	}

	// Find matching item by barcode or SKU
	var matchedItem ReceivingItem
	err = tx.QueryRow(ctx, `
		SELECT id, session_id, fulfillment_item_id, variant_id, sku, barcode, product_title, expected_quantity, scanned_quantity, damaged_quantity, unexpected_quantity, created_at, updated_at
		FROM fulfillment_receiving_items
		WHERE session_id = $1 AND (barcode = $2 OR sku = $2)
		LIMIT 1
		FOR UPDATE
	`, sess.ID, barcode).Scan(
		&matchedItem.ID, &matchedItem.SessionID, &matchedItem.FulfillmentItemID, &matchedItem.VariantID, &matchedItem.SKU, &matchedItem.Barcode, &matchedItem.ProductTitle, &matchedItem.ExpectedQuantity, &matchedItem.ScannedQuantity, &matchedItem.DamagedQuantity, &matchedItem.UnexpectedQuantity, &matchedItem.CreatedAt, &matchedItem.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrInvalidBarcode
		}
		return nil, err
	}

	if matchedItem.ScannedQuantity+1 > matchedItem.ExpectedQuantity {
		return nil, ErrExcessQuantity
	}

	// Atomically increment scanned quantity
	_, err = tx.Exec(ctx, `
		UPDATE fulfillment_receiving_items 
		SET scanned_quantity = scanned_quantity + 1, updated_at = NOW()
		WHERE id = $1
	`, matchedItem.ID)
	if err != nil {
		return nil, err
	}

	// Record idempotency scan if key provided
	if idempotencyKey != "" {
		_, err = tx.Exec(ctx, `
			INSERT INTO fulfillment_receiving_scans (id, session_id, idempotency_key, barcode, scanned_at)
			VALUES ($1, $2, $3, $4, NOW())
		`, uuid.New(), sess.ID, idempotencyKey, barcode)
		if err != nil {
			return nil, err
		}
	}

	// Increment session version
	newVersion := sess.Version + 1
	_, err = tx.Exec(ctx, `
		UPDATE fulfillment_receiving_sessions
		SET version = $1, updated_at = NOW()
		WHERE id = $2
	`, newVersion, sess.ID)
	if err != nil {
		return nil, err
	}
	sess.Version = newVersion

	items, err := r.GetReceivingItemsTx(ctx, tx, sess.ID)
	if err != nil {
		return nil, err
	}
	sess.Items = items
	sess.CanConfirm = checkSessionCanConfirm(items)

	return &sess, nil
}

func (r *Repository) ConfirmReceivingSessionTx(ctx context.Context, tx pgx.Tx, fulfillmentID uuid.UUID, sessionID uuid.UUID, expectedVersion int, staffID uuid.UUID, comment *string) (*Shipment, error) {
	// SELECT FOR UPDATE order_fulfillments
	var fulStatus string
	var orderID uuid.UUID
	err := tx.QueryRow(ctx, `SELECT status, order_id FROM order_fulfillments WHERE id = $1 FOR UPDATE`, fulfillmentID).Scan(&fulStatus, &orderID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrFulfillmentNotFound
		}
		return nil, err
	}
	if fulStatus == "accepted" {
		return nil, ErrFulfillmentAlreadyReceived
	}
	if fulStatus != "packed" && fulStatus != "assembling" {
		return nil, ErrFulfillmentNotPacked
	}

	// SELECT FOR UPDATE active receiving session
	var sess ReceivingSession
	err = tx.QueryRow(ctx, `
		SELECT id, fulfillment_id, status, version, started_at, started_by_staff_id, completed_at, created_at, updated_at
		FROM fulfillment_receiving_sessions
		WHERE id = $1 AND fulfillment_id = $2 AND status = 'active'
		FOR UPDATE
	`, sessionID, fulfillmentID).Scan(
		&sess.ID, &sess.FulfillmentID, &sess.Status, &sess.Version, &sess.StartedAt, &sess.StartedByStaffID, &sess.CompletedAt, &sess.CreatedAt, &sess.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrReceivingNotStarted
		}
		return nil, err
	}

	if sess.Version != expectedVersion {
		return nil, ErrVersionConflict
	}

	// Validate items match expected 100%
	items, err := r.GetReceivingItemsTx(ctx, tx, sess.ID)
	if err != nil {
		return nil, err
	}
	if !checkSessionCanConfirm(items) {
		return nil, errors.New("cannot confirm receiving: scanned items do not match expected items 100%")
	}

	now := time.Now()
	// Update fulfillment status = accepted
	_, err = tx.Exec(ctx, `
		UPDATE order_fulfillments 
		SET status = 'accepted', accepted_at = $1, accepted_by_staff_id = $2, updated_at = $1
		WHERE id = $3
	`, now, staffID, fulfillmentID)
	if err != nil {
		return nil, err
	}

	// Complete session
	_, err = tx.Exec(ctx, `
		UPDATE fulfillment_receiving_sessions
		SET status = 'accepted', completed_at = $1, updated_at = $1
		WHERE id = $2
	`, now, sessionID)
	if err != nil {
		return nil, err
	}

	// Create Shipment (unique constraint idx_shipments_unique_fulfillment guarantees 1 Shipment per fulfillment)
	shipmentID := uuid.New()
	trackingNumber := fmt.Sprintf("TRK-%d-%s", now.Year(), shipmentID.String()[:8])
	queryShipment := `
		INSERT INTO shipments (id, order_id, fulfillment_id, status, carrier, tracking_number, created_at, updated_at)
		VALUES ($1, $2, $3, 'pending', 'ZAMK Express', $4, $5, $5)
		RETURNING id, order_id, fulfillment_id, status, carrier, tracking_number, created_at, updated_at
	`
	var sh Shipment
	err = tx.QueryRow(ctx, queryShipment, shipmentID, orderID, fulfillmentID, trackingNumber, now).Scan(
		&sh.ID, &sh.OrderID, &sh.FulfillmentID, &sh.Status, &sh.Carrier, &sh.TrackingNumber, &sh.CreatedAt, &sh.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}

	return &sh, nil
}

func (r *Repository) RecordReceivingDiscrepancySessionTx(ctx context.Context, tx pgx.Tx, fulfillmentID uuid.UUID, sessionID uuid.UUID, reason, comment string, staffID uuid.UUID) error {
	now := time.Now()
	// Update fulfillment status = discrepancy
	_, err := tx.Exec(ctx, `
		UPDATE order_fulfillments
		SET status = 'discrepancy', discrepancy_reason = $1, discrepancy_comment = $2, discrepancy_at = $3, updated_at = $3
		WHERE id = $4
	`, reason, comment, now, fulfillmentID)
	if err != nil {
		return err
	}

	// Complete session as discrepancy
	if sessionID != uuid.Nil {
		_, err = tx.Exec(ctx, `
			UPDATE fulfillment_receiving_sessions
			SET status = 'discrepancy', completed_at = $1, updated_at = $1
			WHERE id = $2
		`, now, sessionID)
		if err != nil {
			return err
		}
	}
	return nil
}
