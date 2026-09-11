package products

import (
	"context"
	"errors"
	"fmt"

	"github.com/eirfefrggrvgrgrtbg/shop-zamk/backend/internal/notifications"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

// StockAlertReconciler defines the interface to reconcile critical stock alerts.
type StockAlertReconciler interface {
	SyncCriticalStockAlertForProductTx(ctx context.Context, tx pgx.Tx, productID uuid.UUID) error
}

// CriticalStockDedupeKey returns the canonical dedupe key for a product's critical stock alert.
func CriticalStockDedupeKey(productID uuid.UUID) string {
	return fmt.Sprintf("stock:critical:%s", productID.String())
}

// SyncCriticalStockAlertForProductTx reconciles the canonical critical stock alert for a product.
// It loads canonical product/seller/publication state, calculates canonical free stock using CAT.1A definitions,
// checks eligibility, and creates/updates or resolves the seller alert.
func (s *Service) SyncCriticalStockAlertForProductTx(ctx context.Context, tx pgx.Tx, productID uuid.UUID) error {
	if productID == uuid.Nil {
		return nil
	}

	queryProduct := `
		SELECT p.id, p.seller_id, p.title, p.status, s.status, p.price_cents,
		       (SELECT COUNT(*) FROM product_variants pv WHERE pv.product_id = p.id AND pv.is_active = true) AS active_variants_count
		FROM products p
		JOIN sellers s ON p.seller_id = s.id
		WHERE p.id = $1
	`
	var (
		pID                 uuid.UUID
		sellerID            uuid.UUID
		title               string
		pStatus             string
		sellerStatus        string
		priceCents          int
		activeVariantsCount int
	)

	err := tx.QueryRow(ctx, queryProduct, productID).Scan(
		&pID, &sellerID, &title, &pStatus, &sellerStatus, &priceCents, &activeVariantsCount,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil
		}
		return fmt.Errorf("failed to load product for stock alert sync: %w", err)
	}

	dedupeKey := CriticalStockDedupeKey(productID)

	// Eligibility rule:
	// Only create the alert when the product is otherwise storefront-eligible
	// and stock < 2 is the relevant reason it is absent.
	// Ineligible states: draft, pending moderation, in review, rejected, hidden, blocked, inactive seller.
	isEligible := (pStatus == StatusPublished) &&
		sellerStatus == "active" &&
		priceCents > 0 &&
		activeVariantsCount > 0

	if !isEligible {
		// Product is not storefront-eligible. Stock shortage is NOT the reason for absence.
		// If an alert was active previously, resolve it so Seller isn't alerted about stock on draft/rejected products.
		if s.notifs != nil {
			_ = s.notifs.ResolveActiveSellerAlertTx(ctx, tx, sellerID, dedupeKey)
		}
		return nil
	}

	// Calculate canonical free sellable stock using CAT.1A canonical definition
	queryFreeStock := fmt.Sprintf("SELECT %s", CanonicalProductFreeStockSQL("$1"))
	var freeStock int
	if err := tx.QueryRow(ctx, queryFreeStock, productID).Scan(&freeStock); err != nil {
		return fmt.Errorf("failed to calculate canonical product free stock: %w", err)
	}

	if freeStock >= MinStorefrontFreeSellableUnits {
		// Sufficient stock: product is visible in Shop. Resolve active alert if present.
		if s.notifs != nil {
			if err := s.notifs.ResolveActiveSellerAlertTx(ctx, tx, sellerID, dedupeKey); err != nil {
				return fmt.Errorf("failed to resolve active stock alert: %w", err)
			}
		}
		return nil
	}

	// Stock < MinStorefrontFreeSellableUnits (1 or 0):
	// Product is hidden from Shop due to critical stock!
	if s.notifs == nil {
		return nil
	}

	var body string
	if freeStock == 1 {
		body = "Свободный остаток — 1 шт. Для показа товара в магазине необходимо минимум 2 свободные единицы."
	} else {
		body = "Свободного остатка нет. Карточка скрыта из магазина до пополнения товара."
	}

	actionURL := "/supplies/new"
	metadata := map[string]interface{}{
		"productId":            productID.String(),
		"freeSellableStock":    freeStock,
		"minimumRequiredStock": MinStorefrontFreeSellableUnits,
		"productTitle":         title,
	}

	alertNotif := notifications.Notification{
		RecipientKind:     notifications.RecipientKindSeller,
		RecipientSellerID: &sellerID,
		Kind:              notifications.KindAlert,
		Severity:          notifications.SeverityCritical,
		Type:              notifications.TypeStockCriticalHidden,
		Title:             "Карточка скрыта из магазина",
		Body:              body,
		ActionURL:         &actionURL,
		DedupeKey:         &dedupeKey,
		EntityType:        "product",
		EntityID:          productID,
		Metadata:          metadata,
	}

	_, err = s.notifs.CreateOrUpdateActiveSellerAlertTx(ctx, tx, alertNotif)
	if err != nil {
		return fmt.Errorf("failed to upsert active stock alert: %w", err)
	}

	return nil
}

// SyncCriticalStockAlertForProduct reconciles the stock alert in a new transaction.
func (s *Service) SyncCriticalStockAlertForProduct(ctx context.Context, productID uuid.UUID) error {
	if s.dbPool == nil {
		return nil
	}
	return s.dbPool.RunInTx(ctx, func(tx pgx.Tx) error {
		return s.SyncCriticalStockAlertForProductTx(ctx, tx, productID)
	})
}

// SyncCriticalStockAlertsForSellerTx reconciles critical stock alerts for all published
// products belonging to a seller. Used when seller status changes (e.g. active -> blocked or blocked -> active).
func (s *Service) SyncCriticalStockAlertsForSellerTx(ctx context.Context, tx pgx.Tx, sellerID uuid.UUID) error {
	if sellerID == uuid.Nil {
		return nil
	}

	rows, err := tx.Query(ctx, `
		SELECT id
		FROM products
		WHERE seller_id = $1 AND status = $2
	`, sellerID, StatusPublished)
	if err != nil {
		return fmt.Errorf("failed to query seller products for alert sync: %w", err)
	}
	defer rows.Close()

	var productIDs []uuid.UUID
	for rows.Next() {
		var id uuid.UUID
		if err := rows.Scan(&id); err != nil {
			return fmt.Errorf("failed to scan product id: %w", err)
		}
		productIDs = append(productIDs, id)
	}
	if err := rows.Err(); err != nil {
		return fmt.Errorf("error iterating product ids: %w", err)
	}

	for _, pid := range productIDs {
		if err := s.SyncCriticalStockAlertForProductTx(ctx, tx, pid); err != nil {
			return err
		}
	}

	return nil
}

// SyncCriticalStockAlertsForSeller reconciles critical stock alerts for a seller in a new transaction.
func (s *Service) SyncCriticalStockAlertsForSeller(ctx context.Context, sellerID uuid.UUID) error {
	if s.dbPool == nil {
		return nil
	}
	return s.dbPool.RunInTx(ctx, func(tx pgx.Tx) error {
		return s.SyncCriticalStockAlertsForSellerTx(ctx, tx, sellerID)
	})
}
