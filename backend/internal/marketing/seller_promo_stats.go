package marketing

// MARKETING.M1.6 — read-only effectiveness statistics of a seller's promo codes.
//
// Everything is derived from existing rows (promo_code_usages, order_item_promotions, orders, returns);
// nothing is written and no ledger entries are created. Outcomes are kept apart:
//
//	reserved                 order created, not paid yet (the code is held)
//	paid                     usage consumed and the order is not cancelled (the only "sale")
//	cancelled_after_payment  usage consumed but the order was cancelled afterwards
//	released / expired       never paid; the hold was given back
//
// Sales and seller-discount totals cover paid orders only and are shown before returns.
// They are not "extra revenue": there is no attribution of what the customer would have bought without the code.

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/eirfefrggrvgrgrtbg/shop-zamk/backend/internal/auth"
)

const (
	PromoOutcomeReserved              = "reserved"
	PromoOutcomePaid                  = "paid"
	PromoOutcomeCancelledAfterPayment = "cancelled_after_payment"
	PromoOutcomeReleased              = "released"
	PromoOutcomeExpired               = "expired"
)

// PromoStats are the per-code counters. Order counts are numbers of orders, not of units.
type PromoStats struct {
	PromoCodeID                 uuid.UUID `json:"promoCodeId"`
	ReservedOrders              int       `json:"reservedOrders"`
	PaidOrders                  int       `json:"paidOrders"`
	CancelledAfterPaymentOrders int       `json:"cancelledAfterPaymentOrders"`
	ReleasedOrders              int       `json:"releasedOrders"`
	ReturnedOrders              int       `json:"returnedOrders"`
	SalesCents                  int64     `json:"salesCents"`
	SellerDiscountCents         int64     `json:"sellerDiscountCents"`
}

// PromoStatsOrder is one order that used the code. It deliberately carries no buyer data.
type PromoStatsOrder struct {
	OrderID             uuid.UUID `json:"orderId"`
	OrderNumber         *string   `json:"orderNumber,omitempty"`
	Outcome             string    `json:"outcome"`
	At                  time.Time `json:"at"`
	SalesCents          int64     `json:"salesCents"`
	SellerDiscountCents int64     `json:"sellerDiscountCents"`
	Returned            bool      `json:"returned"`
}

type SellerPromoStatsResponse struct {
	Items []PromoStats `json:"items"`
}

type SellerPromoStatsDetail struct {
	PromoStats
	RecentOrders []PromoStatsOrder `json:"recentOrders"`
}

const promoReturnedExists = `EXISTS (
	SELECT 1 FROM returns r
	JOIN return_items ri ON ri.return_id = r.id
	JOIN order_item_promotions oip2 ON oip2.order_item_id = ri.order_item_id
	WHERE r.order_id = u.order_id AND oip2.promo_code_id = p.id AND r.status IN ('refunded', 'completed')
)`

// ListSellerPromoStats returns counters for the seller's promo codes (all of them, or only promoID when set).
// Promo codes without any usage are returned with zeros. Foreign sellers' codes never match.
func (r *Repository) ListSellerPromoStats(ctx context.Context, sellerID uuid.UUID, promoID *uuid.UUID) ([]PromoStats, error) {
	query := `
		WITH lines AS (
			SELECT order_id, promo_code_id,
			       SUM(total_customer_paid_cents) AS sales, SUM(total_seller_discount_cents) AS disc
			FROM order_item_promotions
			WHERE seller_id = $1
			GROUP BY order_id, promo_code_id
		)
		SELECT p.id,
		       COUNT(u.id) FILTER (WHERE u.status = 'reserved'),
		       COUNT(u.id) FILTER (WHERE u.status = 'consumed' AND o.status <> 'cancelled'),
		       COUNT(u.id) FILTER (WHERE u.status = 'consumed' AND o.status = 'cancelled'),
		       COUNT(u.id) FILTER (WHERE u.status IN ('released', 'expired')),
		       COUNT(u.id) FILTER (WHERE u.status = 'consumed' AND o.status <> 'cancelled' AND ` + promoReturnedExists + `),
		       COALESCE(SUM(l.sales) FILTER (WHERE u.status = 'consumed' AND o.status <> 'cancelled'), 0)::bigint,
		       COALESCE(SUM(l.disc)  FILTER (WHERE u.status = 'consumed' AND o.status <> 'cancelled'), 0)::bigint
		FROM promo_codes p
		LEFT JOIN promo_code_usages u ON u.promo_code_id = p.id
		LEFT JOIN orders o ON o.id = u.order_id
		LEFT JOIN lines l ON l.order_id = u.order_id AND l.promo_code_id = p.id
		WHERE p.seller_id = $1 AND ($2::uuid IS NULL OR p.id = $2)
		GROUP BY p.id
		ORDER BY p.created_at DESC`
	rows, err := r.pool.Query(ctx, query, sellerID, promoID)
	if err != nil {
		return nil, fmt.Errorf("failed to list seller promo stats: %w", err)
	}
	defer rows.Close()

	out := []PromoStats{}
	for rows.Next() {
		var s PromoStats
		if err := rows.Scan(&s.PromoCodeID, &s.ReservedOrders, &s.PaidOrders, &s.CancelledAfterPaymentOrders,
			&s.ReleasedOrders, &s.ReturnedOrders, &s.SalesCents, &s.SellerDiscountCents); err != nil {
			return nil, fmt.Errorf("failed to scan promo stats: %w", err)
		}
		out = append(out, s)
	}
	return out, rows.Err()
}

// ListSellerPromoStatsOrders returns the latest orders that used the seller's promo code.
func (r *Repository) ListSellerPromoStatsOrders(ctx context.Context, sellerID, promoID uuid.UUID, limit int) ([]PromoStatsOrder, error) {
	query := `
		WITH lines AS (
			SELECT order_id, SUM(total_customer_paid_cents) AS sales, SUM(total_seller_discount_cents) AS disc
			FROM order_item_promotions
			WHERE seller_id = $1 AND promo_code_id = $2
			GROUP BY order_id
		)
		SELECT u.order_id, o.order_number, u.status, o.status, u.reserved_at, u.consumed_at, u.released_at,
		       COALESCE(l.sales, 0)::bigint, COALESCE(l.disc, 0)::bigint, ` + promoReturnedExists + `
		FROM promo_code_usages u
		JOIN promo_codes p ON p.id = u.promo_code_id
		JOIN orders o ON o.id = u.order_id
		LEFT JOIN lines l ON l.order_id = u.order_id
		WHERE p.id = $2 AND p.seller_id = $1
		ORDER BY u.reserved_at DESC
		LIMIT $3`
	rows, err := r.pool.Query(ctx, query, sellerID, promoID, limit)
	if err != nil {
		return nil, fmt.Errorf("failed to list promo stats orders: %w", err)
	}
	defer rows.Close()

	out := []PromoStatsOrder{}
	for rows.Next() {
		var (
			o                        PromoStatsOrder
			usageStatus, orderStatus string
			reservedAt               time.Time
			consumedAt, releasedAt   *time.Time
		)
		if err := rows.Scan(&o.OrderID, &o.OrderNumber, &usageStatus, &orderStatus, &reservedAt, &consumedAt, &releasedAt,
			&o.SalesCents, &o.SellerDiscountCents, &o.Returned); err != nil {
			return nil, fmt.Errorf("failed to scan promo stats order: %w", err)
		}
		o.At = reservedAt
		switch usageStatus {
		case "consumed":
			o.Outcome = PromoOutcomePaid
			if orderStatus == "cancelled" {
				o.Outcome = PromoOutcomeCancelledAfterPayment
			}
			if consumedAt != nil {
				o.At = *consumedAt
			}
		case "released":
			o.Outcome = PromoOutcomeReleased
			if releasedAt != nil {
				o.At = *releasedAt
			}
		case "expired":
			o.Outcome = PromoOutcomeExpired
			if releasedAt != nil {
				o.At = *releasedAt
			}
		default:
			o.Outcome = PromoOutcomeReserved
		}
		// Only paid orders count as sales; for every other outcome the amounts are zeroed so they
		// cannot be mistaken for revenue.
		if o.Outcome != PromoOutcomePaid {
			o.SalesCents, o.SellerDiscountCents, o.Returned = 0, 0, false
		}
		out = append(out, o)
	}
	return out, rows.Err()
}

func (r *Repository) sellerOwnsPromo(ctx context.Context, sellerID, promoID uuid.UUID) (bool, error) {
	var one int
	err := r.pool.QueryRow(ctx, `SELECT 1 FROM promo_codes WHERE id = $1 AND seller_id = $2`, promoID, sellerID).Scan(&one)
	if err == pgx.ErrNoRows {
		return false, nil
	}
	return err == nil, err
}

func (s *Service) ListSellerPromoStats(ctx context.Context, sellerID uuid.UUID) ([]PromoStats, error) {
	return s.repo.ListSellerPromoStats(ctx, sellerID, nil)
}

// GetSellerPromoStats returns ErrPromoCodeNotFound for unknown and foreign promo codes alike.
func (s *Service) GetSellerPromoStats(ctx context.Context, sellerID, promoID uuid.UUID) (*SellerPromoStatsDetail, error) {
	owns, err := s.repo.sellerOwnsPromo(ctx, sellerID, promoID)
	if err != nil {
		return nil, err
	}
	if !owns {
		return nil, ErrPromoCodeNotFound
	}
	stats, err := s.repo.ListSellerPromoStats(ctx, sellerID, &promoID)
	if err != nil {
		return nil, err
	}
	if len(stats) != 1 {
		return nil, ErrPromoCodeNotFound
	}
	orders, err := s.repo.ListSellerPromoStatsOrders(ctx, sellerID, promoID, 20)
	if err != nil {
		return nil, err
	}
	return &SellerPromoStatsDetail{PromoStats: stats[0], RecentOrders: orders}, nil
}

// ListSellerPromoStats handles GET /api/seller/promotions/stats.
func (h *Handler) ListSellerPromoStats(w http.ResponseWriter, r *http.Request) {
	sellerID, err := auth.GetSellerID(r.Context())
	if err != nil || sellerID == uuid.Nil {
		h.writeError(w, http.StatusUnauthorized, "unauthorized", "Seller authorization required")
		return
	}
	items, err := h.service.ListSellerPromoStats(r.Context(), sellerID)
	if err != nil {
		h.logger.Error("failed to list seller promo stats", "sellerId", sellerID, "error", err)
		h.writeError(w, http.StatusInternalServerError, "internal_error", "Failed to load promo statistics")
		return
	}
	h.writeJSON(w, http.StatusOK, SellerPromoStatsResponse{Items: items})
}

// GetSellerPromoStats handles GET /api/seller/promotions/{id}/stats.
func (h *Handler) GetSellerPromoStats(w http.ResponseWriter, r *http.Request) {
	sellerID, err := auth.GetSellerID(r.Context())
	if err != nil || sellerID == uuid.Nil {
		h.writeError(w, http.StatusUnauthorized, "unauthorized", "Seller authorization required")
		return
	}
	promoID, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		h.writeError(w, http.StatusNotFound, "not_found", "Promo code not found")
		return
	}
	detail, err := h.service.GetSellerPromoStats(r.Context(), sellerID, promoID)
	if err != nil {
		if err == ErrPromoCodeNotFound {
			h.writeError(w, http.StatusNotFound, "not_found", "Promo code not found")
			return
		}
		h.logger.Error("failed to load seller promo stats", "sellerId", sellerID, "promoId", promoID, "error", err)
		h.writeError(w, http.StatusInternalServerError, "internal_error", "Failed to load promo statistics")
		return
	}
	h.writeJSON(w, http.StatusOK, detail)
}
