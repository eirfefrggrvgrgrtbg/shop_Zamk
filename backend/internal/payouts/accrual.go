package payouts

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

var (
	ErrCommissionSnapshotMismatch = errors.New("promotion snapshot commission rate differs from frozen fulfillment rate")
	ErrPromotionSnapshotInvalid   = errors.New("promotion snapshot is inconsistent with order item")
)

// accrualLine is the frozen checkout economics of one order item used for seller ledger accrual.
//
// Per-line invariant (all values in kopecks):
//
//	commission_base = base_gross - seller_discount
//	customer_paid   = commission_base - zamk_subsidy
//	seller_earning  = commission_base - commission
//
// The seller-funded discount reduces the seller's accrual base; the ZAMK-funded subsidy does not.
type accrualLine struct {
	OrderItemID         uuid.UUID
	OrderID             uuid.UUID
	SellerID            uuid.UUID
	BaseGrossCents      int64
	SellerDiscountCents int64
	ZamkSubsidyCents    int64
	CustomerPaidCents   int64
	CommissionBaseCents int64
	CommissionBps       int
	CommissionCents     int64
	CampaignID          *uuid.UUID
	PromoCodeID         *uuid.UUID
	CommissionSource    string
}

// Commission rate sources recorded in the sale_gross metadata.
const (
	commissionSourceFrozen = "frozen_at_checkout"
	commissionSourceActive = "active_rule_at_delivery"
)

func (l accrualLine) SellerEarningCents() int64 {
	return l.CommissionBaseCents - l.CommissionCents
}

func (l accrualLine) metadata() ([]byte, error) {
	meta := map[string]interface{}{
		"pricing_basis":         "standard",
		"base_gross_cents":      l.BaseGrossCents,
		"seller_discount_cents": l.SellerDiscountCents,
		"zamk_subsidy_cents":    l.ZamkSubsidyCents,
		"customer_paid_cents":   l.CustomerPaidCents,
		"commission_base_cents": l.CommissionBaseCents,
		"commission_bps":        l.CommissionBps,
		"commission_source":     l.CommissionSource,
	}
	if l.PromoCodeID != nil {
		meta["pricing_basis"] = "promotion"
		meta["promo_code_id"] = l.PromoCodeID.String()
		meta["campaign_id"] = l.CampaignID.String()
	}
	return json.Marshal(meta)
}

// accrueOrderItemsTx writes sale_gross, zamk_commission and seller_earning for each order item
// selected by filterSQL ($1 = filter value). It locks the order items, skips items that were
// already accrued, and derives amounts only from data frozen at checkout: order_items prices,
// order_fulfillments.commission_bps and the order_item_promotions snapshot.
func (s *Service) accrueOrderItemsTx(ctx context.Context, tx pgx.Tx, filterSQL string, filterValue uuid.UUID) error {
	query := fmt.Sprintf(`
		SELECT oi.id, oi.order_id, oi.seller_id, oi.price_cents, oi.quantity,
		       f.commission_bps, f.commission_frozen,
		       oip.base_unit_price_cents, oip.total_seller_discount_cents, oip.total_zamk_subsidy_cents,
		       oip.total_customer_paid_cents, oip.total_commission_base_cents,
		       oip.commission_rate_bps, oip.total_commission_charged_cents,
		       oip.campaign_id, oip.promo_code_id,
		       EXISTS (
		           SELECT 1 FROM seller_ledger_entries sle
		           WHERE sle.order_item_id = oi.id AND sle.type = 'sale_gross'
		       ) AS already_accrued
		FROM order_items oi
		LEFT JOIN order_fulfillments f ON f.id = oi.order_fulfillment_id
		LEFT JOIN order_item_promotions oip ON oip.order_item_id = oi.id
		WHERE %s = $1
		ORDER BY oi.id
		FOR UPDATE OF oi
	`, filterSQL)

	rows, err := tx.Query(ctx, query, filterValue)
	if err != nil {
		return fmt.Errorf("failed to fetch order items for accrual: %w", err)
	}

	type rawItem struct {
		ID, OrderID, SellerID uuid.UUID
		PriceCents            int64
		Quantity              int
		FrozenBps             *int
		CommissionFrozen      *bool
		PromoBaseUnit         *int64
		PromoSellerDiscount   *int64
		PromoZamkSubsidy      *int64
		PromoCustomerPaid     *int64
		PromoCommissionBase   *int64
		PromoCommissionBps    *int
		PromoCommission       *int64
		CampaignID            *uuid.UUID
		PromoCodeID           *uuid.UUID
		AlreadyAccrued        bool
	}
	var raws []rawItem
	for rows.Next() {
		var r rawItem
		if err := rows.Scan(&r.ID, &r.OrderID, &r.SellerID, &r.PriceCents, &r.Quantity,
			&r.FrozenBps, &r.CommissionFrozen,
			&r.PromoBaseUnit, &r.PromoSellerDiscount, &r.PromoZamkSubsidy,
			&r.PromoCustomerPaid, &r.PromoCommissionBase,
			&r.PromoCommissionBps, &r.PromoCommission,
			&r.CampaignID, &r.PromoCodeID,
			&r.AlreadyAccrued,
		); err != nil {
			rows.Close()
			return fmt.Errorf("failed to scan order item for accrual: %w", err)
		}
		raws = append(raws, r)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return fmt.Errorf("order items rows err: %w", err)
	}

	for _, r := range raws {
		if r.AlreadyAccrued {
			continue // idempotent: an order item is accrued exactly once
		}

		// Fulfillments created by checkout carry the rate frozen at order time. Legacy rows
		// (commission_frozen = false, or no fulfillment at all) only hold the column default /
		// marketplace config rate, so their rate is chosen at delivery as before M0.1.
		frozen := r.FrozenBps != nil && r.CommissionFrozen != nil && *r.CommissionFrozen
		var bps int
		source := commissionSourceFrozen
		if frozen {
			bps = *r.FrozenBps
		} else {
			source = commissionSourceActive
			bps, err = s.GetActiveCommissionRateBPS(ctx, r.SellerID)
			if err != nil {
				return fmt.Errorf("failed to get commission rate: %w", err)
			}
		}

		baseGross := r.PriceCents * int64(r.Quantity)
		line := accrualLine{
			OrderItemID:         r.ID,
			OrderID:             r.OrderID,
			SellerID:            r.SellerID,
			BaseGrossCents:      baseGross,
			CustomerPaidCents:   baseGross,
			CommissionBaseCents: baseGross,
			CommissionBps:       bps,
			CommissionCents:     (baseGross * int64(bps)) / 10000,
			CommissionSource:    source,
		}

		if r.PromoCodeID != nil {
			// The snapshot rate and charged commission of legacy promo orders were computed with the
			// marketplace default rate, so they are only compared and reused for frozen fulfillments.
			if frozen && *r.PromoCommissionBps != bps {
				return fmt.Errorf("%w: order item %s snapshot %d bps, fulfillment %d bps",
					ErrCommissionSnapshotMismatch, r.ID, *r.PromoCommissionBps, bps)
			}
			if *r.PromoBaseUnit != r.PriceCents ||
				*r.PromoCommissionBase != baseGross-*r.PromoSellerDiscount ||
				*r.PromoCustomerPaid != *r.PromoCommissionBase-*r.PromoZamkSubsidy {
				return fmt.Errorf("%w: order item %s", ErrPromotionSnapshotInvalid, r.ID)
			}
			line.SellerDiscountCents = *r.PromoSellerDiscount
			line.ZamkSubsidyCents = *r.PromoZamkSubsidy
			line.CustomerPaidCents = *r.PromoCustomerPaid
			line.CommissionBaseCents = *r.PromoCommissionBase
			if frozen {
				line.CommissionCents = *r.PromoCommission
			} else {
				line.CommissionCents = (*r.PromoCommissionBase * int64(bps)) / 10000
			}
			line.CampaignID = r.CampaignID
			line.PromoCodeID = r.PromoCodeID
		}

		if err := s.writeAccrualTx(ctx, tx, line); err != nil {
			return err
		}
	}
	return nil
}

func (s *Service) writeAccrualTx(ctx context.Context, tx pgx.Tx, line accrualLine) error {
	meta, err := line.metadata()
	if err != nil {
		return fmt.Errorf("failed to encode accrual metadata: %w", err)
	}
	now := time.Now()
	orderID := line.OrderID
	itemID := line.OrderItemID

	entries := []SellerLedgerEntry{
		{Type: "sale_gross", AmountCents: line.CommissionBaseCents, Metadata: meta},
		{Type: "zamk_commission", AmountCents: -line.CommissionCents},
		{Type: "seller_earning", AmountCents: line.SellerEarningCents()},
	}
	for i := range entries {
		e := &entries[i]
		e.ID = uuid.New()
		e.SellerID = line.SellerID
		e.OrderID = &orderID
		e.OrderItemID = &itemID
		e.Currency = "RUB"
		e.CreatedAt = now
		if err := s.repo.CreateLedgerEntryTx(ctx, tx, e); err != nil {
			return err
		}
	}
	return nil
}
