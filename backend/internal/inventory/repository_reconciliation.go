package inventory

import (
	"context"
	"errors"
	"fmt"


	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

func (r *Repository) StartReconciliationSession(ctx context.Context, sessionID, variantID, startedBy uuid.UUID) error {
	// CTE to atomically insert and snapshot
	_, err := r.db.Exec(ctx, `
		WITH ins_session AS (
			INSERT INTO inventory_reconciliation_sessions (id, product_variant_id, status, started_by, started_at)
			VALUES ($1, $2, 'in_progress', $3, NOW())
			RETURNING id
		)
		INSERT INTO inventory_reconciliation_expected_units (session_id, inventory_unit_id, expected_status)
		SELECT s.id, iu.id, iu.status
		FROM ins_session s, inventory_units iu
		WHERE iu.product_variant_id = $2 AND iu.status = 'warehouse'
	`, sessionID, variantID, startedBy)

	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			return ErrReconciliationAlreadyActive
		}
		return fmt.Errorf("failed to start reconciliation session: %w", err)
	}
	return nil
}

func (r *Repository) populateSessionVariantInfo(ctx context.Context, session *ReconciliationSessionDTO) error {
	item, err := r.GetAdminInventoryItemRichByVariantID(ctx, session.VariantID)
	if err != nil {
		return err
	}
	session.VariantTitle = item.Product.Title
	session.VariantSize = item.Variant.Size
	session.VariantColor = item.Variant.Color
	if item.Variant.SKU != "" {
		session.VariantSKU = item.Variant.SKU
	}
	if item.Variant.Barcode != "" {
		session.VariantBarcode = item.Variant.Barcode
	}
	session.AccountingMode = item.AccountingMode
	session.LegacyOnHand = item.Legacy.OnHand
	return nil
}

func (r *Repository) GetActiveReconciliationSession(ctx context.Context, variantID uuid.UUID) (*ReconciliationSessionDTO, error) {
	var session ReconciliationSessionDTO
	err := r.db.QueryRow(ctx, `
		SELECT id, product_variant_id, status, started_by, started_at
		FROM inventory_reconciliation_sessions
		WHERE product_variant_id = $1 AND status IN ('in_progress', 'review')
	`, variantID).Scan(&session.ID, &session.VariantID, &session.Status, &session.StartedBy, &session.StartedAt)

	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to query active session: %w", err)
	}

	counters, err := r.GetReconciliationCounters(ctx, session.ID)
	if err != nil {
		return nil, err
	}
	session.ExpectedCount = counters.ExpectedCount
	session.FoundExpectedCount = counters.FoundExpectedCount
	session.UnexpectedCount = counters.UnexpectedCount
	session.ProblemsCount = counters.ProblemsCount

	if err := r.populateSessionVariantInfo(ctx, &session); err != nil {
		return nil, err
	}

	return &session, nil
}

func (r *Repository) GetReconciliationSessionByID(ctx context.Context, sessionID uuid.UUID) (*ReconciliationSessionDTO, error) {
	var session ReconciliationSessionDTO
	err := r.db.QueryRow(ctx, `
		SELECT id, product_variant_id, status, started_by, started_at
		FROM inventory_reconciliation_sessions
		WHERE id = $1
	`, sessionID).Scan(&session.ID, &session.VariantID, &session.Status, &session.StartedBy, &session.StartedAt)

	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to query session: %w", err)
	}

	counters, err := r.GetReconciliationCounters(ctx, session.ID)
	if err != nil {
		return nil, err
	}
	session.ExpectedCount = counters.ExpectedCount
	session.FoundExpectedCount = counters.FoundExpectedCount
	session.UnexpectedCount = counters.UnexpectedCount
	session.ProblemsCount = counters.ProblemsCount

	if err := r.populateSessionVariantInfo(ctx, &session); err != nil {
		return nil, err
	}

	return &session, nil
}

func (r *Repository) GetReconciliationCounters(ctx context.Context, sessionID uuid.UUID) (ReconciliationSessionDTO, error) {
	var counters ReconciliationSessionDTO
	err := r.db.QueryRow(ctx, `
		WITH expected AS (
			SELECT count(*) as cnt FROM inventory_reconciliation_expected_units WHERE session_id = $1
		),
		scans AS (
			SELECT classification, count(*) as cnt
			FROM inventory_reconciliation_scans
			WHERE session_id = $1
			GROUP BY classification
		)
		SELECT
			(SELECT cnt FROM expected),
			COALESCE((SELECT cnt FROM scans WHERE classification = 'expected_found'), 0),
			COALESCE((SELECT cnt FROM scans WHERE classification = 'unexpected_found'), 0),
			COALESCE((SELECT SUM(cnt) FROM scans WHERE classification IN ('wrong_variant', 'unknown_code')), 0)
	`, sessionID).Scan(
		&counters.ExpectedCount,
		&counters.FoundExpectedCount,
		&counters.UnexpectedCount,
		&counters.ProblemsCount,
	)
	if err != nil {
		return counters, fmt.Errorf("failed to fetch counters: %w", err)
	}
	return counters, nil
}

func (r *Repository) ProcessReconciliationScan(ctx context.Context, sessionID uuid.UUID, rawCode string, scannedBy uuid.UUID) (*ScanReconciliationResponse, error) {
	// 1. Check session status
	var sessionVariantID uuid.UUID
	var status string
	err := r.db.QueryRow(ctx, `
		SELECT product_variant_id, status
		FROM inventory_reconciliation_sessions
		WHERE id = $1
	`, sessionID).Scan(&sessionVariantID, &status)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrReconciliationNotFound
		}
		return nil, err
	}
	if status != "in_progress" {
		return nil, ErrReconciliationNotInProgress
	}

	// 2. Query unit identity with variant and product details
	var unitID uuid.UUID
	var unitVariantID uuid.UUID
	var unitStatus string
	var pTitle string
	var vSize, vColor, vSKU, vBarcode string

	err = r.db.QueryRow(ctx, `
		SELECT iu.id, iu.product_variant_id, iu.status, p.title,
		       COALESCE(v.size, ''), COALESCE(v.color, ''), COALESCE(v.sku, ''), COALESCE(v.barcode, '')
		FROM inventory_units iu
		JOIN product_variants v ON iu.product_variant_id = v.id
		JOIN products p ON v.product_id = p.id
		WHERE iu.unit_code = $1
	`, rawCode).Scan(&unitID, &unitVariantID, &unitStatus, &pTitle, &vSize, &vColor, &vSKU, &vBarcode)

	var classification string
	var classificationUnitID *uuid.UUID

	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			classification = "unknown_code"
			classificationUnitID = nil
		} else {
			return nil, err
		}
	} else {
		classificationUnitID = &unitID
		if unitVariantID != sessionVariantID {
			classification = "wrong_variant"
		} else {
			var expected bool
			err = r.db.QueryRow(ctx, `
				SELECT EXISTS(
					SELECT 1 FROM inventory_reconciliation_expected_units
					WHERE session_id = $1 AND inventory_unit_id = $2
				)
			`, sessionID, unitID).Scan(&expected)
			if err != nil {
				return nil, err
			}

			if expected {
				classification = "expected_found"
			} else {
				classification = "unexpected_found"
			}
		}
	}

	scanID := uuid.New()
	if classificationUnitID != nil {
		// Atomic idempotent insert guarded by session.status = 'in_progress'
		res, err := r.db.Exec(ctx, `
			INSERT INTO inventory_reconciliation_scans (id, session_id, inventory_unit_id, raw_code, classification, scanned_by, scanned_at)
			SELECT $1, s.id, $3, $4, $5, $6, NOW()
			FROM inventory_reconciliation_sessions s
			WHERE s.id = $2 AND s.status = 'in_progress'
			ON CONFLICT (session_id, inventory_unit_id) WHERE inventory_unit_id IS NOT NULL DO NOTHING
		`, scanID, sessionID, classificationUnitID, rawCode, classification, scannedBy)
		if err != nil {
			return nil, fmt.Errorf("failed to insert scan: %w", err)
		}
		if res.RowsAffected() == 0 {
			// Check if duplicate or if session transitioned concurrently
			var curStatus string
			err = r.db.QueryRow(ctx, `SELECT status FROM inventory_reconciliation_sessions WHERE id = $1`, sessionID).Scan(&curStatus)
			if err == nil && curStatus != "in_progress" {
				return nil, ErrReconciliationNotInProgress
			}
			classification = "duplicate"
		}
	} else {
		// Unknown code has NULL inventory_unit_id
		res, err := r.db.Exec(ctx, `
			INSERT INTO inventory_reconciliation_scans (id, session_id, inventory_unit_id, raw_code, classification, scanned_by, scanned_at)
			SELECT $1, s.id, $3, $4, $5, $6, NOW()
			FROM inventory_reconciliation_sessions s
			WHERE s.id = $2 AND s.status = 'in_progress'
		`, scanID, sessionID, classificationUnitID, rawCode, classification, scannedBy)
		if err != nil {
			return nil, fmt.Errorf("failed to insert unknown code scan: %w", err)
		}
		if res.RowsAffected() == 0 {
			return nil, ErrReconciliationNotInProgress
		}
	}


	sessionPtr, err := r.GetReconciliationSessionByID(ctx, sessionID)
	if err != nil || sessionPtr == nil {
		return nil, err
	}

	resObj := &ScanReconciliationResponse{
		Classification: classification,
		Session:        *sessionPtr,
	}

	if classificationUnitID != nil {
		resObj.Unit = &AdminInventoryPhysicalUnit{
			ID:       *classificationUnitID,
			UnitCode: rawCode,
			Status:   unitStatus,
		}
		resObj.UnitContext = &ReconciliationScanUnitContext{
			UnitCode:     rawCode,
			ProductTitle: pTitle,
			Size:         vSize,
			Color:        vColor,
			SKU:          vSKU,
			Barcode:      vBarcode,
			Status:       unitStatus,
		}
	}

	return resObj, nil
}

func (r *Repository) ListReconciliationSessionsByVariant(ctx context.Context, variantID uuid.UUID, limit int) ([]ReconciliationSessionDTO, error) {
	if limit <= 0 || limit > 50 {
		limit = 10
	}
	query := `
		WITH expected AS (
			SELECT session_id, count(*) as expected_count
			FROM inventory_reconciliation_expected_units
			GROUP BY session_id
		),
		scans AS (
			SELECT
				session_id,
				count(*) FILTER (WHERE classification = 'expected_found') as found_expected,
				count(*) FILTER (WHERE classification = 'unexpected_found') as unexpected_count,
				count(*) FILTER (WHERE classification IN ('wrong_variant', 'unknown_code')) as problems_count
			FROM inventory_reconciliation_scans
			GROUP BY session_id
		)
		SELECT
			s.id, s.product_variant_id, s.status, s.started_by, s.started_at,
			s.completed_by, s.completed_at, s.cancelled_by, s.cancelled_at, COALESCE(s.notes, ''),
			COALESCE(e.expected_count, 0),
			COALESCE(sc.found_expected, 0),
			COALESCE(sc.unexpected_count, 0),
			COALESCE(sc.problems_count, 0)
		FROM inventory_reconciliation_sessions s
		LEFT JOIN expected e ON e.session_id = s.id
		LEFT JOIN scans sc ON sc.session_id = s.id
		WHERE s.product_variant_id = $1
		ORDER BY s.started_at DESC
		LIMIT $2
	`
	rows, err := r.db.Query(ctx, query, variantID, limit)
	if err != nil {
		return nil, fmt.Errorf("failed to list reconciliation sessions: %w", err)
	}
	defer rows.Close()

	var list []ReconciliationSessionDTO
	for rows.Next() {
		var s ReconciliationSessionDTO
		err := rows.Scan(
			&s.ID, &s.VariantID, &s.Status, &s.StartedBy, &s.StartedAt,
			&s.CompletedBy, &s.CompletedAt, &s.CancelledBy, &s.CancelledAt, &s.Notes,
			&s.ExpectedCount,
			&s.FoundExpectedCount,
			&s.UnexpectedCount,
			&s.ProblemsCount,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan session row: %w", err)
		}
		list = append(list, s)
	}
	if list == nil {
		list = []ReconciliationSessionDTO{}
	}
	return list, nil
}


func (r *Repository) ChangeReconciliationStatus(ctx context.Context, sessionID uuid.UUID, oldStatus, newStatus string, by uuid.UUID) error {
	isValid := (oldStatus == "in_progress" && newStatus == "review") ||
		(oldStatus == "review" && newStatus == "completed") ||
		(oldStatus == "in_progress" && newStatus == "cancelled") ||
		(oldStatus == "review" && newStatus == "cancelled")
	if !isValid {
		return ErrInvalidReconciliationState
	}

	res, err := r.db.Exec(ctx, `
		UPDATE inventory_reconciliation_sessions
		SET status = $3::varchar,
		    completed_by = CASE WHEN $3::text = 'completed' THEN $4 ELSE completed_by END,
		    completed_at = CASE WHEN $3::text = 'completed' THEN NOW() ELSE completed_at END,
		    cancelled_by = CASE WHEN $3::text = 'cancelled' THEN $4 ELSE cancelled_by END,
		    cancelled_at = CASE WHEN $3::text = 'cancelled' THEN NOW() ELSE cancelled_at END
		WHERE id = $1 AND status = $2
	`, sessionID, oldStatus, newStatus, by)
	if err != nil {
		return err
	}
	if res.RowsAffected() == 0 {
		return ErrInvalidReconciliationState
	}
	return nil
}

func (r *Repository) GetReconciliationReview(ctx context.Context, sessionID uuid.UUID) (*ReconciliationReviewDTO, error) {
	review := &ReconciliationReviewDTO{
		ExpectedFound:      []ReconciliationReviewItemDTO{},
		Missing:            []ReconciliationReviewItemDTO{},
		UnexpectedFound:    []ReconciliationReviewItemDTO{},
		ChangedDuringCount: []ReconciliationReviewItemDTO{},
	}

	rows, err := r.db.Query(ctx, `
		SELECT
			COALESCE(e.inventory_unit_id, s.inventory_unit_id) as unit_id,
			iu.unit_code,
			e.expected_status,
			iu.status as current_status,
			s.classification,
			s.scanned_at
		FROM inventory_reconciliation_expected_units e
		FULL OUTER JOIN inventory_reconciliation_scans s
		  ON e.session_id = s.session_id AND e.inventory_unit_id = s.inventory_unit_id
		JOIN inventory_units iu ON iu.id = COALESCE(e.inventory_unit_id, s.inventory_unit_id)
		WHERE (e.session_id = $1 OR s.session_id = $1)
		  AND (s.classification IS NULL OR s.classification IN ('expected_found', 'unexpected_found'))
	`, sessionID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	for rows.Next() {
		var item ReconciliationReviewItemDTO
		var snapStatus *string
		var classification *string

		if err := rows.Scan(&item.UnitID, &item.UnitCode, &snapStatus, &item.CurrentStatus, &classification, &item.ScannedAt); err != nil {
			return nil, err
		}

		if snapStatus != nil {
			item.SnapshotStatus = *snapStatus
		}
		if classification != nil {
			item.Classification = *classification
		}

		if classification != nil && *classification == "expected_found" {
			if item.SnapshotStatus != item.CurrentStatus {
				review.ChangedDuringCount = append(review.ChangedDuringCount, item)
			} else {
				review.ExpectedFound = append(review.ExpectedFound, item)
			}
		} else if classification != nil && *classification == "unexpected_found" {
			review.UnexpectedFound = append(review.UnexpectedFound, item)
		} else if classification == nil {
			// Expected but not found
			if item.SnapshotStatus != item.CurrentStatus {
				review.ChangedDuringCount = append(review.ChangedDuringCount, item)
			} else {
				review.Missing = append(review.Missing, item)
			}
		}
	}
	return review, nil
}
