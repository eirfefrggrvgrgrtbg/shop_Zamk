package marketing

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

type DBExecutor interface {
	Exec(ctx context.Context, sql string, arguments ...any) (pgconn.CommandTag, error)
	Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error)
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
}

type Repository struct {
	pool *pgxpool.Pool
}

func NewRepository(pool *pgxpool.Pool) *Repository {
	return &Repository{pool: pool}
}

func (r *Repository) CreateCampaign(ctx context.Context, c *MarketingCampaign) error {
	return r.CreateCampaignTx(ctx, r.pool, c)
}

func (r *Repository) CreateCampaignTx(ctx context.Context, db DBExecutor, c *MarketingCampaign) error {
	if c.ID == uuid.Nil {
		c.ID = uuid.New()
	}
	now := time.Now().UTC()
	c.CreatedAt = now
	c.UpdatedAt = now

	query := `
		INSERT INTO marketing_campaigns (
			id, seller_id, title, description, funding_mode, status, discount_type,
			seller_discount_bps, seller_discount_fixed_cents,
			requested_zamk_share_bps, requested_zamk_budget_cap_cents,
			approved_zamk_share_bps, approved_zamk_budget_cap_cents,
			zamk_reserved_cents, zamk_spent_cents,
			rejection_reason, admin_comment,
			starts_at, ends_at, submitted_at, decided_at, decided_by_staff_id,
			created_at, updated_at
		) VALUES (
			$1, $2, $3, $4, $5, $6, $7,
			$8, $9,
			$10, $11,
			$12, $13,
			$14, $15,
			$16, $17,
			$18, $19, $20, $21, $22,
			$23, $24
		)
	`
	_, err := db.Exec(ctx, query,
		c.ID, c.SellerID, c.Title, c.Description, string(c.FundingMode), string(c.Status), string(c.DiscountType),
		c.SellerDiscountBps, c.SellerDiscountFixedCents,
		c.RequestedZamkShareBps, c.RequestedZamkBudgetCapCents,
		c.ApprovedZamkShareBps, c.ApprovedZamkBudgetCapCents,
		c.ZamkReservedCents, c.ZamkSpentCents,
		c.RejectionReason, c.AdminComment,
		c.StartsAt, c.EndsAt, c.SubmittedAt, c.DecidedAt, c.DecidedByStaffID,
		c.CreatedAt, c.UpdatedAt,
	)
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) {
			if pgErr.ConstraintName == "uq_open_platform_funding_per_seller" {
				return ErrOpenPlatformFundingExists
			}
		}
		return fmt.Errorf("failed to create marketing campaign: %w", err)
	}
	return nil
}

func (r *Repository) GetCampaignByID(ctx context.Context, id uuid.UUID) (*MarketingCampaign, error) {
	query := `
		SELECT
			id, seller_id, title, description, funding_mode, status, discount_type,
			seller_discount_bps, seller_discount_fixed_cents,
			requested_zamk_share_bps, requested_zamk_budget_cap_cents,
			approved_zamk_share_bps, approved_zamk_budget_cap_cents,
			zamk_reserved_cents, zamk_spent_cents,
			rejection_reason, admin_comment,
			starts_at, ends_at, submitted_at, decided_at, decided_by_staff_id,
			created_at, updated_at
		FROM marketing_campaigns
		WHERE id = $1
	`
	var c MarketingCampaign
	var fMode, status, dType string
	err := r.pool.QueryRow(ctx, query, id).Scan(
		&c.ID, &c.SellerID, &c.Title, &c.Description, &fMode, &status, &dType,
		&c.SellerDiscountBps, &c.SellerDiscountFixedCents,
		&c.RequestedZamkShareBps, &c.RequestedZamkBudgetCapCents,
		&c.ApprovedZamkShareBps, &c.ApprovedZamkBudgetCapCents,
		&c.ZamkReservedCents, &c.ZamkSpentCents,
		&c.RejectionReason, &c.AdminComment,
		&c.StartsAt, &c.EndsAt, &c.SubmittedAt, &c.DecidedAt, &c.DecidedByStaffID,
		&c.CreatedAt, &c.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrCampaignNotFound
		}
		return nil, fmt.Errorf("failed to get marketing campaign: %w", err)
	}
	c.FundingMode = FundingMode(fMode)
	c.Status = CampaignStatus(status)
	c.DiscountType = DiscountType(dType)
	return &c, nil
}

func (r *Repository) CountSuccessfulSellerSales(ctx context.Context, sellerID uuid.UUID) (int, error) {
	query := `
		SELECT COUNT(*)
		FROM order_fulfillments
		WHERE seller_id = $1
		  AND status IN (
		    'paid', 'assembling', 'packed', 'accepted',
		    'discrepancy', 'shipped', 'delivered',
		    'returned', 'refunded'
		  )
	`
	var count int
	err := r.pool.QueryRow(ctx, query, sellerID).Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("failed to count seller successful paid sales: %w", err)
	}
	return count, nil
}

func (r *Repository) HasOpenPlatformCampaign(ctx context.Context, sellerID uuid.UUID) (bool, error) {
	query := `
		SELECT EXISTS (
			SELECT 1 FROM marketing_campaigns
			WHERE seller_id = $1
			  AND funding_mode IN ('zamk', 'cofunded')
			  AND status IN ('submitted', 'counter_offered', 'approved', 'active')
		)
	`
	var exists bool
	err := r.pool.QueryRow(ctx, query, sellerID).Scan(&exists)
	return exists, err
}

func (r *Repository) SubmitCampaign(ctx context.Context, campaignID uuid.UUID, sellerID uuid.UUID) error {
	now := time.Now().UTC()
	query := `
		UPDATE marketing_campaigns
		SET status = 'submitted', submitted_at = $1, updated_at = $1
		WHERE id = $2 AND seller_id = $3 AND status = 'draft'
	`
	tag, err := r.pool.Exec(ctx, query, now, campaignID, sellerID)
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) {
			if pgErr.ConstraintName == "uq_open_platform_funding_per_seller" {
				return ErrOpenPlatformFundingExists
			}
		}
		return fmt.Errorf("failed to submit campaign: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrCampaignNotFound
	}
	return nil
}

func (r *Repository) AdminApproveCampaign(ctx context.Context, campaignID uuid.UUID, staffUserID uuid.UUID, approvedShareBps int, approvedBudgetCapCents int64, adminComment *string) error {
	now := time.Now().UTC()
	query := `
		UPDATE marketing_campaigns
		SET status = 'approved',
		    approved_zamk_share_bps = $1,
		    approved_zamk_budget_cap_cents = $2,
		    admin_comment = $3,
		    decided_at = $4,
		    decided_by_staff_id = $5,
		    updated_at = $4
		WHERE id = $6 AND status = 'submitted'
	`
	tag, err := r.pool.Exec(ctx, query, approvedShareBps, approvedBudgetCapCents, adminComment, now, staffUserID, campaignID)
	if err != nil {
		return fmt.Errorf("failed to approve campaign: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrCampaignNotFound
	}
	return nil
}

func (r *Repository) AdminCounterOfferCampaign(ctx context.Context, campaignID uuid.UUID, staffUserID uuid.UUID, counterShareBps int, counterBudgetCapCents int64, adminComment *string) error {
	now := time.Now().UTC()
	query := `
		UPDATE marketing_campaigns
		SET status = 'counter_offered',
		    approved_zamk_share_bps = $1,
		    approved_zamk_budget_cap_cents = $2,
		    admin_comment = $3,
		    decided_at = $4,
		    decided_by_staff_id = $5,
		    updated_at = $4
		WHERE id = $6 AND status = 'submitted'
	`
	tag, err := r.pool.Exec(ctx, query, counterShareBps, counterBudgetCapCents, adminComment, now, staffUserID, campaignID)
	if err != nil {
		return fmt.Errorf("failed to counter-offer campaign: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrCampaignNotFound
	}
	return nil
}

func (r *Repository) AdminRejectCampaign(ctx context.Context, campaignID uuid.UUID, staffUserID uuid.UUID, reason string, comment *string) error {
	now := time.Now().UTC()
	query := `
		UPDATE marketing_campaigns
		SET status = 'rejected',
		    rejection_reason = $1,
		    admin_comment = $2,
		    decided_at = $3,
		    decided_by_staff_id = $4,
		    updated_at = $3
		WHERE id = $5 AND status = 'submitted'
	`
	tag, err := r.pool.Exec(ctx, query, reason, comment, now, staffUserID, campaignID)
	if err != nil {
		return fmt.Errorf("failed to reject campaign: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrCampaignNotFound
	}
	return nil
}

func (r *Repository) SellerAcceptCounterOffer(ctx context.Context, campaignID uuid.UUID, sellerID uuid.UUID) error {
	now := time.Now().UTC()
	query := `
		UPDATE marketing_campaigns
		SET status = 'approved', updated_at = $1
		WHERE id = $2 AND seller_id = $3 AND status = 'counter_offered'
	`
	tag, err := r.pool.Exec(ctx, query, now, campaignID, sellerID)
	if err != nil {
		return fmt.Errorf("failed to accept counter offer: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrCounterOfferInvalidState
	}
	return nil
}

func (r *Repository) SellerRejectCounterOffer(ctx context.Context, campaignID uuid.UUID, sellerID uuid.UUID) error {
	now := time.Now().UTC()
	query := `
		UPDATE marketing_campaigns
		SET status = 'cancelled', updated_at = $1
		WHERE id = $2 AND seller_id = $3 AND status = 'counter_offered'
	`
	tag, err := r.pool.Exec(ctx, query, now, campaignID, sellerID)
	if err != nil {
		return fmt.Errorf("failed to reject counter offer: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrCounterOfferInvalidState
	}
	return nil
}

func (r *Repository) CreatePromoCode(ctx context.Context, p *PromoCode) error {
	return r.CreatePromoCodeTx(ctx, r.pool, p)
}

func (r *Repository) CreatePromoCodeTx(ctx context.Context, db DBExecutor, p *PromoCode) error {
	if p.ID == uuid.Nil {
		p.ID = uuid.New()
	}
	now := time.Now().UTC()
	p.CreatedAt = now
	p.UpdatedAt = now
	p.Code = strings.ToUpper(strings.TrimSpace(p.Code))

	if p.ProductScope == "" {
		p.ProductScope = ProductScopeEntireStore
	}

	query := `
		INSERT INTO promo_codes (
			id, campaign_id, seller_id, code, discount_type,
			discount_value_bps, discount_value_fixed_cents,
			min_order_subtotal_cents, global_usage_limit, per_customer_usage_limit,
			audience_type, product_scope, max_discount_cents,
			min_eligible_quantity, min_distinct_products,
			is_active, starts_at, ends_at,
			created_at, updated_at
		) VALUES (
			$1, $2, $3, $4, $5,
			$6, $7,
			$8, $9, $10,
			$11, $12, $13,
			$14, $15,
			$16, $17, $18,
			$19, $20
		)
	`
	if p.AudienceType == "" {
		p.AudienceType = AudienceAllCustomers
	}
	_, err := db.Exec(ctx, query,
		p.ID, p.CampaignID, p.SellerID, p.Code, string(p.DiscountType),
		p.DiscountValueBps, p.DiscountValueFixedCents,
		p.MinOrderSubtotalCents, p.GlobalUsageLimit, p.PerCustomerUsageLimit,
		string(p.AudienceType), string(p.ProductScope), p.MaxDiscountCents,
		p.MinEligibleQuantity, p.MinDistinctProducts,
		p.IsActive, p.StartsAt, p.EndsAt,
		p.CreatedAt, p.UpdatedAt,
	)
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) {
			if pgErr.ConstraintName == "uq_promo_codes_normalized_code" {
				return ErrPromoCodeDuplicate
			}
		}
		return fmt.Errorf("failed to create promo code: %w", err)
	}
	return nil
}

func (r *Repository) GetPromoCodeByCode(ctx context.Context, code string) (*PromoCode, error) {
	normalized := strings.TrimSpace(code)
	query := `
		SELECT
			id, campaign_id, seller_id, code, discount_type,
			discount_value_bps, discount_value_fixed_cents,
			min_order_subtotal_cents, global_usage_limit, per_customer_usage_limit,
			audience_type, product_scope, max_discount_cents,
			min_eligible_quantity, min_distinct_products,
			is_active, starts_at, ends_at,
			created_at, updated_at
		FROM promo_codes
		WHERE LOWER(code) = LOWER($1)
	`
	var p PromoCode
	var dType, pScope, aType string
	err := r.pool.QueryRow(ctx, query, normalized).Scan(
		&p.ID, &p.CampaignID, &p.SellerID, &p.Code, &dType,
		&p.DiscountValueBps, &p.DiscountValueFixedCents,
		&p.MinOrderSubtotalCents, &p.GlobalUsageLimit, &p.PerCustomerUsageLimit,
		&aType, &pScope, &p.MaxDiscountCents,
		&p.MinEligibleQuantity, &p.MinDistinctProducts,
		&p.IsActive, &p.StartsAt, &p.EndsAt,
		&p.CreatedAt, &p.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrPromoCodeNotFound
		}
		return nil, fmt.Errorf("failed to get promo code: %w", err)
	}
	p.DiscountType = DiscountType(dType)
	p.ProductScope = ProductScope(pScope)
	p.AudienceType = AudienceType(aType)
	return &p, nil
}

func (r *Repository) RecordPriceChange(ctx context.Context, h *ProductPriceHistory) error {
	return r.RecordPriceChangeTx(ctx, r.pool, h)
}

func (r *Repository) RecordPriceChangeTx(ctx context.Context, db DBExecutor, h *ProductPriceHistory) error {
	if h.ID == uuid.Nil {
		h.ID = uuid.New()
	}
	if h.CreatedAt.IsZero() {
		h.CreatedAt = time.Now().UTC()
	}

	query := `
		INSERT INTO product_price_history (
			id, product_id, product_variant_id, old_price_cents, new_price_cents,
			changed_by_user_id, source, reason, created_at
		) VALUES (
			$1, $2, $3, $4, $5,
			$6, $7, $8, $9
		)
	`
	_, err := db.Exec(ctx, query,
		h.ID, h.ProductID, h.ProductVariantID, h.OldPriceCents, h.NewPriceCents,
		h.ChangedByUserID, string(h.Source), h.Reason, h.CreatedAt,
	)
	if err != nil {
		return fmt.Errorf("failed to record product price history: %w", err)
	}
	return nil
}

func (r *Repository) ListProductPriceHistory(ctx context.Context, productID uuid.UUID, limit int) ([]ProductPriceHistory, error) {
	if limit <= 0 {
		limit = 50
	}
	query := `
		SELECT
			id, product_id, product_variant_id, old_price_cents, new_price_cents,
			changed_by_user_id, source, reason, created_at
		FROM product_price_history
		WHERE product_id = $1
		ORDER BY created_at DESC
		LIMIT $2
	`
	rows, err := r.pool.Query(ctx, query, productID, limit)
	if err != nil {
		return nil, fmt.Errorf("failed to query price history: %w", err)
	}
	defer rows.Close()

	var history []ProductPriceHistory
	for rows.Next() {
		var h ProductPriceHistory
		var source string
		if err := rows.Scan(
			&h.ID, &h.ProductID, &h.ProductVariantID, &h.OldPriceCents, &h.NewPriceCents,
			&h.ChangedByUserID, &source, &h.Reason, &h.CreatedAt,
		); err != nil {
			return nil, fmt.Errorf("failed to scan price history: %w", err)
		}
		h.Source = PriceChangeSource(source)
		history = append(history, h)
	}
	return history, rows.Err()
}

func (r *Repository) CreateOrderItemPromotion(ctx context.Context, p *OrderItemPromotion) error {
	return r.CreateOrderItemPromotionTx(ctx, r.pool, p)
}

func (r *Repository) CreateOrderItemPromotionTx(ctx context.Context, db DBExecutor, p *OrderItemPromotion) error {
	if p.ID == uuid.Nil {
		p.ID = uuid.New()
	}
	if p.CreatedAt.IsZero() {
		p.CreatedAt = time.Now().UTC()
	}

	query := `
		INSERT INTO order_item_promotions (
			id, order_item_id, order_id, seller_id, campaign_id, promo_code_id,
			base_unit_price_cents, seller_discount_unit_cents, zamk_subsidy_unit_cents, customer_paid_unit_price_cents,
			commission_base_unit_cents, quantity, total_seller_discount_cents, total_zamk_subsidy_cents, total_customer_paid_cents,
			total_commission_base_cents, commission_rate_bps, total_commission_charged_cents,
			created_at
		) VALUES (
			$1, $2, $3, $4, $5, $6,
			$7, $8, $9, $10,
			$11, $12, $13, $14, $15,
			$16, $17, $18,
			$19
		)
	`
	_, err := db.Exec(ctx, query,
		p.ID, p.OrderItemID, p.OrderID, p.SellerID, p.CampaignID, p.PromoCodeID,
		p.BaseUnitPriceCents, p.SellerDiscountUnitCents, p.ZamkSubsidyUnitCents, p.CustomerPaidUnitPriceCents,
		p.CommissionBaseUnitCents, p.Quantity, p.TotalSellerDiscountCents, p.TotalZamkSubsidyCents, p.TotalCustomerPaidCents,
		p.TotalCommissionBaseCents, p.CommissionRateBps, p.TotalCommissionChargedCents,
		p.CreatedAt,
	)
	if err != nil {
		return fmt.Errorf("failed to create order item promotion: %w", err)
	}
	return nil
}

func (r *Repository) GetOrderItemPromotion(ctx context.Context, orderItemID uuid.UUID) (*OrderItemPromotion, error) {
	query := `
		SELECT
			id, order_item_id, order_id, seller_id, campaign_id, promo_code_id,
			base_unit_price_cents, seller_discount_unit_cents, zamk_subsidy_unit_cents, customer_paid_unit_price_cents,
			commission_base_unit_cents, quantity, total_seller_discount_cents, total_zamk_subsidy_cents, total_customer_paid_cents,
			total_commission_base_cents, commission_rate_bps, total_commission_charged_cents,
			created_at
		FROM order_item_promotions
		WHERE order_item_id = $1
	`
	var p OrderItemPromotion
	err := r.pool.QueryRow(ctx, query, orderItemID).Scan(
		&p.ID, &p.OrderItemID, &p.OrderID, &p.SellerID, &p.CampaignID, &p.PromoCodeID,
		&p.BaseUnitPriceCents, &p.SellerDiscountUnitCents, &p.ZamkSubsidyUnitCents, &p.CustomerPaidUnitPriceCents,
		&p.CommissionBaseUnitCents, &p.Quantity, &p.TotalSellerDiscountCents, &p.TotalZamkSubsidyCents, &p.TotalCustomerPaidCents,
		&p.TotalCommissionBaseCents, &p.CommissionRateBps, &p.TotalCommissionChargedCents,
		&p.CreatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to get order item promotion: %w", err)
	}
	return &p, nil
}

// GetPromoCodeByCodeTx queries promo code within a transaction.
func (r *Repository) GetPromoCodeByCodeTx(ctx context.Context, db DBExecutor, code string) (*PromoCode, error) {
	normalized := strings.TrimSpace(code)
	query := `
		SELECT
			id, campaign_id, seller_id, code, discount_type,
			discount_value_bps, discount_value_fixed_cents,
			min_order_subtotal_cents, global_usage_limit, per_customer_usage_limit,
			audience_type, product_scope, max_discount_cents,
			min_eligible_quantity, min_distinct_products,
			is_active, starts_at, ends_at,
			created_at, updated_at
		FROM promo_codes
		WHERE LOWER(code) = LOWER($1)
	`
	var p PromoCode
	var dType, pScope, aType string
	err := db.QueryRow(ctx, query, normalized).Scan(
		&p.ID, &p.CampaignID, &p.SellerID, &p.Code, &dType,
		&p.DiscountValueBps, &p.DiscountValueFixedCents,
		&p.MinOrderSubtotalCents, &p.GlobalUsageLimit, &p.PerCustomerUsageLimit,
		&aType, &pScope, &p.MaxDiscountCents,
		&p.MinEligibleQuantity, &p.MinDistinctProducts,
		&p.IsActive, &p.StartsAt, &p.EndsAt,
		&p.CreatedAt, &p.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrPromoCodeNotFound
		}
		return nil, fmt.Errorf("failed to get promo code in tx: %w", err)
	}
	p.DiscountType = DiscountType(dType)
	p.ProductScope = ProductScope(pScope)
	p.AudienceType = AudienceType(aType)
	return &p, nil
}

// GetPromoCodeForUpdateTx selects and locks a promo code row FOR UPDATE.
func (r *Repository) GetPromoCodeForUpdateTx(ctx context.Context, db DBExecutor, id uuid.UUID) (*PromoCode, error) {
	query := `
		SELECT
			id, campaign_id, seller_id, code, discount_type,
			discount_value_bps, discount_value_fixed_cents,
			min_order_subtotal_cents, global_usage_limit, per_customer_usage_limit,
			audience_type, product_scope, max_discount_cents,
			min_eligible_quantity, min_distinct_products,
			is_active, starts_at, ends_at,
			created_at, updated_at
		FROM promo_codes
		WHERE id = $1
		FOR UPDATE
	`
	var p PromoCode
	var dType, pScope, aType string
	err := db.QueryRow(ctx, query, id).Scan(
		&p.ID, &p.CampaignID, &p.SellerID, &p.Code, &dType,
		&p.DiscountValueBps, &p.DiscountValueFixedCents,
		&p.MinOrderSubtotalCents, &p.GlobalUsageLimit, &p.PerCustomerUsageLimit,
		&aType, &pScope, &p.MaxDiscountCents,
		&p.MinEligibleQuantity, &p.MinDistinctProducts,
		&p.IsActive, &p.StartsAt, &p.EndsAt,
		&p.CreatedAt, &p.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrPromoCodeNotFound
		}
		return nil, fmt.Errorf("failed to lock promo code for update: %w", err)
	}
	p.DiscountType = DiscountType(dType)
	p.ProductScope = ProductScope(pScope)
	p.AudienceType = AudienceType(aType)
	return &p, nil
}

// GetCampaignForUpdateTx selects and locks a marketing campaign row FOR UPDATE.
func (r *Repository) GetCampaignForUpdateTx(ctx context.Context, db DBExecutor, id uuid.UUID) (*MarketingCampaign, error) {
	query := `
		SELECT
			id, seller_id, title, description, funding_mode, status, discount_type,
			seller_discount_bps, seller_discount_fixed_cents,
			requested_zamk_share_bps, requested_zamk_budget_cap_cents,
			approved_zamk_share_bps, approved_zamk_budget_cap_cents,
			zamk_reserved_cents, zamk_spent_cents,
			rejection_reason, admin_comment,
			starts_at, ends_at, submitted_at, decided_at, decided_by_staff_id,
			created_at, updated_at
		FROM marketing_campaigns
		WHERE id = $1
		FOR UPDATE
	`
	var c MarketingCampaign
	var fMode, status, dType string
	err := db.QueryRow(ctx, query, id).Scan(
		&c.ID, &c.SellerID, &c.Title, &c.Description, &fMode, &status, &dType,
		&c.SellerDiscountBps, &c.SellerDiscountFixedCents,
		&c.RequestedZamkShareBps, &c.RequestedZamkBudgetCapCents,
		&c.ApprovedZamkShareBps, &c.ApprovedZamkBudgetCapCents,
		&c.ZamkReservedCents, &c.ZamkSpentCents,
		&c.RejectionReason, &c.AdminComment,
		&c.StartsAt, &c.EndsAt, &c.SubmittedAt, &c.DecidedAt, &c.DecidedByStaffID,
		&c.CreatedAt, &c.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrCampaignNotFound
		}
		return nil, fmt.Errorf("failed to lock marketing campaign for update: %w", err)
	}
	c.FundingMode = FundingMode(fMode)
	c.Status = CampaignStatus(status)
	c.DiscountType = DiscountType(dType)
	return &c, nil
}

// CountPromoUsageGlobalTx counts active reservations and consumed usages for a promo code.
func (r *Repository) CountPromoUsageGlobalTx(ctx context.Context, db DBExecutor, promoID uuid.UUID) (int, error) {
	query := `
		SELECT COUNT(*)
		FROM promo_code_usages
		WHERE promo_code_id = $1 AND status IN ('consumed', 'reserved')
	`
	var count int
	err := db.QueryRow(ctx, query, promoID).Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("failed to count global promo usages: %w", err)
	}
	return count, nil
}

// CountPromoUsageCustomerTx counts active reservations and consumed usages for a specific customer and promo code.
func (r *Repository) CountPromoUsageCustomerTx(ctx context.Context, db DBExecutor, promoID, customerID uuid.UUID) (int, error) {
	query := `
		SELECT COUNT(*)
		FROM promo_code_usages
		WHERE promo_code_id = $1 AND user_id = $2 AND status IN ('consumed', 'reserved')
	`
	var count int
	err := db.QueryRow(ctx, query, promoID, customerID).Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("failed to count customer promo usages: %w", err)
	}
	return count, nil
}

// CountCustomerSuccessfullyPaidOrdersTx counts how many distinct orders have ever been successfully paid
// by this customer (backed strictly by payments.status = 'succeeded', excluding an optional current order).
func CountCustomerSuccessfullyPaidOrdersTx(ctx context.Context, db DBExecutor, customerID, excludeOrderID uuid.UUID) (int, error) {
	var query string
	var count int
	var err error
	if excludeOrderID != uuid.Nil {
		query = `
			SELECT COUNT(DISTINCT o.id)
			FROM orders o
			WHERE o.user_id = $1
			  AND o.id != $2
			  AND EXISTS (
				SELECT 1
				FROM payments p
				WHERE p.order_id = o.id
				  AND p.status = 'succeeded'
			  )
		`
		err = db.QueryRow(ctx, query, customerID, excludeOrderID).Scan(&count)
	} else {
		query = `
			SELECT COUNT(DISTINCT o.id)
			FROM orders o
			WHERE o.user_id = $1
			  AND EXISTS (
				SELECT 1
				FROM payments p
				WHERE p.order_id = o.id
				  AND p.status = 'succeeded'
			  )
		`
		err = db.QueryRow(ctx, query, customerID).Scan(&count)
	}
	if err != nil {
		return 0, fmt.Errorf("failed to count customer successfully paid orders: %w", err)
	}
	return count, nil
}

// CountCustomerSuccessfullyPaidOrdersTx is the repository method forwarding to the canonical function.
func (r *Repository) CountCustomerSuccessfullyPaidOrdersTx(ctx context.Context, db DBExecutor, customerID, excludeOrderID uuid.UUID) (int, error) {
	return CountCustomerSuccessfullyPaidOrdersTx(ctx, db, customerID, excludeOrderID)
}

// CountCustomerPaidOrdersTx is retained as an alias to the canonical CountCustomerSuccessfullyPaidOrdersTx.
func (r *Repository) CountCustomerPaidOrdersTx(ctx context.Context, db DBExecutor, customerID, excludeOrderID uuid.UUID) (int, error) {
	return CountCustomerSuccessfullyPaidOrdersTx(ctx, db, customerID, excludeOrderID)
}

// HasActiveFirstOrderReservationTx checks if the customer already holds an active first-order reservation.
func (r *Repository) HasActiveFirstOrderReservationTx(ctx context.Context, db DBExecutor, customerID, excludeOrderID uuid.UUID) (bool, error) {
	query := `
		SELECT EXISTS (
			SELECT 1 FROM promo_code_usages
			WHERE user_id = $1 AND status = 'reserved' AND is_first_order = true AND order_id != $2
		)
	`
	var exists bool
	err := db.QueryRow(ctx, query, customerID, excludeOrderID).Scan(&exists)
	if err != nil {
		return false, fmt.Errorf("failed to check active first order reservation: %w", err)
	}
	return exists, nil
}

// ReserveZamkBudgetTx atomically checks remaining budget and increments zamk_reserved_cents.
func (r *Repository) ReserveZamkBudgetTx(ctx context.Context, db DBExecutor, campaignID uuid.UUID, subsidyCents int64) (bool, error) {
	if subsidyCents <= 0 {
		return true, nil
	}
	query := `
		UPDATE marketing_campaigns
		SET zamk_reserved_cents = zamk_reserved_cents + $1, updated_at = NOW()
		WHERE id = $2
		  AND (zamk_spent_cents + zamk_reserved_cents + $1 <= approved_zamk_budget_cap_cents)
	`
	tag, err := db.Exec(ctx, query, subsidyCents, campaignID)
	if err != nil {
		return false, fmt.Errorf("failed to reserve ZAMK budget: %w", err)
	}
	return tag.RowsAffected() > 0, nil
}

// ConsumeZamkBudgetTx atomically decrements zamk_reserved_cents and increments zamk_spent_cents on payment success.
func (r *Repository) ConsumeZamkBudgetTx(ctx context.Context, db DBExecutor, campaignID uuid.UUID, subsidyCents int64) error {
	if subsidyCents <= 0 {
		return nil
	}
	query := `
		UPDATE marketing_campaigns
		SET zamk_reserved_cents = GREATEST(0, zamk_reserved_cents - $1),
		    zamk_spent_cents = zamk_spent_cents + $1,
		    updated_at = NOW()
		WHERE id = $2
	`
	_, err := db.Exec(ctx, query, subsidyCents, campaignID)
	if err != nil {
		return fmt.Errorf("failed to consume ZAMK budget: %w", err)
	}
	return nil
}

// ReleaseZamkBudgetTx atomically decrements zamk_reserved_cents when a reservation fails or is cancelled.
func (r *Repository) ReleaseZamkBudgetTx(ctx context.Context, db DBExecutor, campaignID uuid.UUID, subsidyCents int64) error {
	if subsidyCents <= 0 {
		return nil
	}
	query := `
		UPDATE marketing_campaigns
		SET zamk_reserved_cents = GREATEST(0, zamk_reserved_cents - $1),
		    updated_at = NOW()
		WHERE id = $2
	`
	_, err := db.Exec(ctx, query, subsidyCents, campaignID)
	if err != nil {
		return fmt.Errorf("failed to release ZAMK budget: %w", err)
	}
	return nil
}

// CreatePromoCodeUsageTx inserts a new promo_code_usages record in a transaction.
func (r *Repository) CreatePromoCodeUsageTx(ctx context.Context, db DBExecutor, u *PromoUsage) error {
	if u.ID == uuid.Nil {
		u.ID = uuid.New()
	}
	if u.ReservedAt.IsZero() {
		u.ReservedAt = time.Now().UTC()
	}
	query := `
		INSERT INTO promo_code_usages (
			id, promo_code_id, campaign_id, order_id, user_id,
			status, subsidy_cents, seller_discount_cents,
			reserved_at, expires_at, is_first_order
		) VALUES (
			$1, $2, $3, $4, $5,
			$6, $7, $8,
			$9, $10, $11
		)
	`
	_, err := db.Exec(ctx, query,
		u.ID, u.PromoCodeID, u.CampaignID, u.OrderID, u.UserID,
		string(u.Status), u.SubsidyCents, u.SellerDiscountCents,
		u.ReservedAt, u.ExpiresAt, u.IsFirstOrder,
	)
	if err != nil {
		return fmt.Errorf("failed to create promo code usage: %w", err)
	}
	return nil
}

// GetPromoCodeUsageByOrderIDForUpdateTx selects and locks a promo_code_usages row FOR UPDATE.
func (r *Repository) GetPromoCodeUsageByOrderIDForUpdateTx(ctx context.Context, db DBExecutor, orderID uuid.UUID) (*PromoUsage, error) {
	query := `
		SELECT
			id, promo_code_id, campaign_id, order_id, user_id,
			status, subsidy_cents, seller_discount_cents,
			reserved_at, consumed_at, released_at, expires_at, is_first_order
		FROM promo_code_usages
		WHERE order_id = $1
		FOR UPDATE
	`
	var u PromoUsage
	var status string
	err := db.QueryRow(ctx, query, orderID).Scan(
		&u.ID, &u.PromoCodeID, &u.CampaignID, &u.OrderID, &u.UserID,
		&status, &u.SubsidyCents, &u.SellerDiscountCents,
		&u.ReservedAt, &u.ConsumedAt, &u.ReleasedAt, &u.ExpiresAt, &u.IsFirstOrder,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil // No promo for this order
		}
		return nil, fmt.Errorf("failed to lock promo code usage for order: %w", err)
	}
	u.Status = UsageStatus(status)
	return &u, nil
}

// SetPromoCodeUsageStatusTx atomically transitions usage from expected status to new status.
func (r *Repository) SetPromoCodeUsageStatusTx(ctx context.Context, db DBExecutor, usageID uuid.UUID, fromStatus, toStatus UsageStatus, timestamp time.Time) (bool, error) {
	var query string
	if toStatus == UsageStatusConsumed {
		query = `
			UPDATE promo_code_usages
			SET status = $2, consumed_at = $3
			WHERE id = $1 AND status = $4
		`
	} else if toStatus == UsageStatusReleased || toStatus == UsageStatusExpired {
		query = `
			UPDATE promo_code_usages
			SET status = $2, released_at = $3
			WHERE id = $1 AND status = $4
		`
	} else {
		query = `
			UPDATE promo_code_usages
			SET status = $2
			WHERE id = $1 AND status = $3
		`
		tag, err := db.Exec(ctx, query, usageID, string(toStatus), string(fromStatus))
		if err != nil {
			return false, fmt.Errorf("failed to transition promo usage status: %w", err)
		}
		return tag.RowsAffected() > 0, nil
	}
	tag, err := db.Exec(ctx, query, usageID, string(toStatus), timestamp, string(fromStatus))
	if err != nil {
		return false, fmt.Errorf("failed to transition promo usage status: %w", err)
	}
	return tag.RowsAffected() > 0, nil
}

// ListSellerPromos fetches all promo codes for an authenticated seller with aggregated usage counts and targets.
func (r *Repository) ListSellerPromos(ctx context.Context, sellerID uuid.UUID) ([]SellerPromoItem, error) {
	query := `
		SELECT
			p.id, p.campaign_id, p.seller_id, p.code, p.discount_type,
			p.discount_value_bps, p.discount_value_fixed_cents,
			p.min_order_subtotal_cents, p.global_usage_limit, p.per_customer_usage_limit,
			p.audience_type, p.product_scope, p.max_discount_cents,
			p.min_eligible_quantity, p.min_distinct_products,
			p.is_active, p.starts_at, p.ends_at,
			p.created_at, p.updated_at,
			COALESCE(COUNT(CASE WHEN u.status = 'reserved' THEN 1 END), 0) AS reserved_count,
			COALESCE(COUNT(CASE WHEN u.status = 'consumed' THEN 1 END), 0) AS consumed_count
		FROM promo_codes p
		LEFT JOIN promo_code_usages u ON u.promo_code_id = p.id AND u.status IN ('reserved', 'consumed')
		WHERE p.seller_id = $1
		GROUP BY p.id
		ORDER BY p.created_at DESC
	`
	rows, err := r.pool.Query(ctx, query, sellerID)
	if err != nil {
		return nil, fmt.Errorf("failed to list seller promo codes: %w", err)
	}
	defer rows.Close()

	var items []SellerPromoItem
	var promoIDs []uuid.UUID
	for rows.Next() {
		var it SellerPromoItem
		var dType, pScope, aType string
		if err := rows.Scan(
			&it.ID, &it.CampaignID, &it.SellerID, &it.Code, &dType,
			&it.DiscountValueBps, &it.DiscountValueFixedCents,
			&it.MinOrderSubtotalCents, &it.GlobalUsageLimit, &it.PerCustomerUsageLimit,
			&aType, &pScope, &it.MaxDiscountCents,
			&it.MinEligibleQuantity, &it.MinDistinctProducts,
			&it.IsActive, &it.StartsAt, &it.EndsAt,
			&it.CreatedAt, &it.UpdatedAt,
			&it.ReservedCount, &it.ConsumedCount,
		); err != nil {
			return nil, fmt.Errorf("failed to scan seller promo item: %w", err)
		}
		it.DiscountType = DiscountType(dType)
		it.ProductScope = ProductScope(pScope)
		it.AudienceType = AudienceType(aType)
		it.IncludedProductIDs = []uuid.UUID{}
		it.ExcludedProductIDs = []uuid.UUID{}
		it.IncludedCategoryIDs = []uuid.UUID{}
		it.ExcludedCategoryIDs = []uuid.UUID{}
		items = append(items, it)
		promoIDs = append(promoIDs, it.ID)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	if len(promoIDs) > 0 {
		targetsQuery := `
			SELECT promo_code_id, product_id, target_type
			FROM promo_code_product_targets
			WHERE promo_code_id = ANY($1)
			ORDER BY created_at ASC
		`
		tRows, err := r.pool.Query(ctx, targetsQuery, promoIDs)
		if err != nil {
			return nil, fmt.Errorf("failed to query promo targets: %w", err)
		}
		defer tRows.Close()

		targetsMap := make(map[uuid.UUID]struct {
			inc []uuid.UUID
			exc []uuid.UUID
		})
		for tRows.Next() {
			var pID, prodID uuid.UUID
			var tType string
			if err := tRows.Scan(&pID, &prodID, &tType); err != nil {
				return nil, fmt.Errorf("failed to scan promo target: %w", err)
			}
			entry := targetsMap[pID]
			if TargetType(tType) == TargetTypeInclude {
				entry.inc = append(entry.inc, prodID)
			} else if TargetType(tType) == TargetTypeExclude {
				entry.exc = append(entry.exc, prodID)
			}
			targetsMap[pID] = entry
		}
		if err := tRows.Err(); err != nil {
			return nil, err
		}

		catTargetsQuery := `
			SELECT promo_code_id, category_id, target_type
			FROM promo_code_category_targets
			WHERE promo_code_id = ANY($1)
			ORDER BY created_at ASC
		`
		ctRows, err := r.pool.Query(ctx, catTargetsQuery, promoIDs)
		if err != nil {
			return nil, fmt.Errorf("failed to query promo category targets: %w", err)
		}
		defer ctRows.Close()

		catTargetsMap := make(map[uuid.UUID]struct {
			inc []uuid.UUID
			exc []uuid.UUID
		})
		for ctRows.Next() {
			var pID, catID uuid.UUID
			var tType string
			if err := ctRows.Scan(&pID, &catID, &tType); err != nil {
				return nil, fmt.Errorf("failed to scan promo category target: %w", err)
			}
			entry := catTargetsMap[pID]
			if TargetType(tType) == TargetTypeInclude {
				entry.inc = append(entry.inc, catID)
			} else if TargetType(tType) == TargetTypeExclude {
				entry.exc = append(entry.exc, catID)
			}
			catTargetsMap[pID] = entry
		}
		if err := ctRows.Err(); err != nil {
			return nil, err
		}

		for i := range items {
			if entry, ok := targetsMap[items[i].ID]; ok {
				if len(entry.inc) > 0 {
					items[i].IncludedProductIDs = entry.inc
				}
				if len(entry.exc) > 0 {
					items[i].ExcludedProductIDs = entry.exc
				}
			}
			if catEntry, ok := catTargetsMap[items[i].ID]; ok {
				if len(catEntry.inc) > 0 {
					items[i].IncludedCategoryIDs = catEntry.inc
				}
				if len(catEntry.exc) > 0 {
					items[i].ExcludedCategoryIDs = catEntry.exc
				}
			}
		}
	}

	return items, nil
}

// GetSellerPromoForUpdateTx locks a promo code row belonging strictly to sellerID for update.
func (r *Repository) GetSellerPromoForUpdateTx(ctx context.Context, db DBExecutor, promoID, sellerID uuid.UUID) (*PromoCode, error) {
	query := `
		SELECT
			id, campaign_id, seller_id, code, discount_type,
			discount_value_bps, discount_value_fixed_cents,
			min_order_subtotal_cents, global_usage_limit, per_customer_usage_limit,
			audience_type, product_scope, max_discount_cents,
			min_eligible_quantity, min_distinct_products,
			is_active, starts_at, ends_at,
			created_at, updated_at
		FROM promo_codes
		WHERE id = $1 AND seller_id = $2
		FOR UPDATE
	`
	var p PromoCode
	var dType, pScope, aType string
	err := db.QueryRow(ctx, query, promoID, sellerID).Scan(
		&p.ID, &p.CampaignID, &p.SellerID, &p.Code, &dType,
		&p.DiscountValueBps, &p.DiscountValueFixedCents,
		&p.MinOrderSubtotalCents, &p.GlobalUsageLimit, &p.PerCustomerUsageLimit,
		&aType, &pScope, &p.MaxDiscountCents,
		&p.MinEligibleQuantity, &p.MinDistinctProducts,
		&p.IsActive, &p.StartsAt, &p.EndsAt,
		&p.CreatedAt, &p.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrPromoCodeNotFound
		}
		return nil, fmt.Errorf("failed to lock seller promo code for update: %w", err)
	}
	p.DiscountType = DiscountType(dType)
	p.ProductScope = ProductScope(pScope)
	p.AudienceType = AudienceType(aType)
	return &p, nil
}

// CreatePromoCodeProductTargetsTx inserts target entries for a promo code.
func (r *Repository) CreatePromoCodeProductTargetsTx(ctx context.Context, db DBExecutor, targets []PromoCodeProductTarget) error {
	if len(targets) == 0 {
		return nil
	}
	query := `
		INSERT INTO promo_code_product_targets (id, promo_code_id, product_id, target_type, created_at)
		VALUES ($1, $2, $3, $4, $5)
	`
	now := time.Now().UTC()
	for _, t := range targets {
		id := t.ID
		if id == uuid.Nil {
			id = uuid.New()
		}
		createdAt := t.CreatedAt
		if createdAt.IsZero() {
			createdAt = now
		}
		if _, err := db.Exec(ctx, query, id, t.PromoCodeID, t.ProductID, string(t.TargetType), createdAt); err != nil {
			return fmt.Errorf("failed to insert promo code product target: %w", err)
		}
	}
	return nil
}

// GetPromoCodeProductTargetsTx loads all product targets for a promo code within a transaction.
func (r *Repository) GetPromoCodeProductTargetsTx(ctx context.Context, db DBExecutor, promoID uuid.UUID) ([]PromoCodeProductTarget, error) {
	query := `
		SELECT id, promo_code_id, product_id, target_type, created_at
		FROM promo_code_product_targets
		WHERE promo_code_id = $1
		ORDER BY created_at ASC
	`
	rows, err := db.Query(ctx, query, promoID)
	if err != nil {
		return nil, fmt.Errorf("failed to query promo code product targets: %w", err)
	}
	defer rows.Close()

	var targets []PromoCodeProductTarget
	for rows.Next() {
		var t PromoCodeProductTarget
		var tType string
		if err := rows.Scan(&t.ID, &t.PromoCodeID, &t.ProductID, &tType, &t.CreatedAt); err != nil {
			return nil, fmt.Errorf("failed to scan promo code product target: %w", err)
		}
		t.TargetType = TargetType(tType)
		targets = append(targets, t)
	}
	return targets, rows.Err()
}

// GetPromoCodeProductTargets loads all product targets for a promo code on the pool.
func (r *Repository) GetPromoCodeProductTargets(ctx context.Context, promoID uuid.UUID) ([]PromoCodeProductTarget, error) {
	return r.GetPromoCodeProductTargetsTx(ctx, r.pool, promoID)
}

// CreatePromoCodeCategoryTargetsTx inserts target entries for a promo code.
func (r *Repository) CreatePromoCodeCategoryTargetsTx(ctx context.Context, db DBExecutor, targets []PromoCodeCategoryTarget) error {
	if len(targets) == 0 {
		return nil
	}
	query := `
		INSERT INTO promo_code_category_targets (id, promo_code_id, category_id, target_type, created_at)
		VALUES ($1, $2, $3, $4, $5)
	`
	now := time.Now().UTC()
	for _, t := range targets {
		id := t.ID
		if id == uuid.Nil {
			id = uuid.New()
		}
		createdAt := t.CreatedAt
		if createdAt.IsZero() {
			createdAt = now
		}
		if _, err := db.Exec(ctx, query, id, t.PromoCodeID, t.CategoryID, string(t.TargetType), createdAt); err != nil {
			return fmt.Errorf("failed to insert promo code category target: %w", err)
		}
	}
	return nil
}

// GetPromoCodeCategoryTargetsTx loads all category targets for a promo code within a transaction.
func (r *Repository) GetPromoCodeCategoryTargetsTx(ctx context.Context, db DBExecutor, promoID uuid.UUID) ([]PromoCodeCategoryTarget, error) {
	query := `
		SELECT id, promo_code_id, category_id, target_type, created_at
		FROM promo_code_category_targets
		WHERE promo_code_id = $1
		ORDER BY created_at ASC
	`
	rows, err := db.Query(ctx, query, promoID)
	if err != nil {
		return nil, fmt.Errorf("failed to query promo code category targets: %w", err)
	}
	defer rows.Close()

	var targets []PromoCodeCategoryTarget
	for rows.Next() {
		var t PromoCodeCategoryTarget
		var tType string
		if err := rows.Scan(&t.ID, &t.PromoCodeID, &t.CategoryID, &tType, &t.CreatedAt); err != nil {
			return nil, fmt.Errorf("failed to scan promo code category target: %w", err)
		}
		t.TargetType = TargetType(tType)
		targets = append(targets, t)
	}
	return targets, rows.Err()
}

// GetPromoCodeCategoryTargets loads all category targets for a promo code on the pool.
func (r *Repository) GetPromoCodeCategoryTargets(ctx context.Context, promoID uuid.UUID) ([]PromoCodeCategoryTarget, error) {
	return r.GetPromoCodeCategoryTargetsTx(ctx, r.pool, promoID)
}

// ValidateCategoriesExistTx checks that all provided category IDs exist and are active.
func (r *Repository) ValidateCategoriesExistTx(ctx context.Context, db DBExecutor, categoryIDs []uuid.UUID) (bool, error) {
	if len(categoryIDs) == 0 {
		return true, nil
	}
	uniqueMap := make(map[uuid.UUID]struct{}, len(categoryIDs))
	for _, id := range categoryIDs {
		uniqueMap[id] = struct{}{}
	}
	uniqueIDs := make([]uuid.UUID, 0, len(uniqueMap))
	for id := range uniqueMap {
		uniqueIDs = append(uniqueIDs, id)
	}

	query := `SELECT COUNT(*) FROM categories WHERE id = ANY($1) AND is_active = true`
	var count int
	if err := db.QueryRow(ctx, query, uniqueIDs).Scan(&count); err != nil {
		return false, fmt.Errorf("failed to validate categories existence: %w", err)
	}
	return count == len(uniqueIDs), nil
}

// GetProductCategoryIDsTx retrieves category IDs for a slice of product IDs.
func (r *Repository) GetProductCategoryIDsTx(ctx context.Context, db DBExecutor, productIDs []uuid.UUID) (map[uuid.UUID]uuid.UUID, error) {
	if len(productIDs) == 0 {
		return map[uuid.UUID]uuid.UUID{}, nil
	}
	uniqueMap := make(map[uuid.UUID]struct{}, len(productIDs))
	for _, id := range productIDs {
		uniqueMap[id] = struct{}{}
	}
	uniqueIDs := make([]uuid.UUID, 0, len(uniqueMap))
	for id := range uniqueMap {
		uniqueIDs = append(uniqueIDs, id)
	}

	query := `SELECT id, category_id FROM products WHERE id = ANY($1)`
	rows, err := db.Query(ctx, query, uniqueIDs)
	if err != nil {
		return nil, fmt.Errorf("failed to query product category IDs: %w", err)
	}
	defer rows.Close()

	result := make(map[uuid.UUID]uuid.UUID, len(uniqueIDs))
	for rows.Next() {
		var pID, catID uuid.UUID
		if err := rows.Scan(&pID, &catID); err != nil {
			return nil, fmt.Errorf("failed to scan product category ID: %w", err)
		}
		result[pID] = catID
	}
	return result, rows.Err()
}

// ResolveCategorySubtreeIDsTx resolves category IDs and all their descendants in a single recursive CTE.
func (r *Repository) ResolveCategorySubtreeIDsTx(ctx context.Context, db DBExecutor, categoryIDs []uuid.UUID, activeOnly bool) ([]uuid.UUID, error) {
	if len(categoryIDs) == 0 {
		return []uuid.UUID{}, nil
	}
	uniqueMap := make(map[uuid.UUID]struct{}, len(categoryIDs))
	for _, id := range categoryIDs {
		uniqueMap[id] = struct{}{}
	}
	uniqueIDs := make([]uuid.UUID, 0, len(uniqueMap))
	for id := range uniqueMap {
		uniqueIDs = append(uniqueIDs, id)
	}

	query := `
		WITH RECURSIVE cat_tree AS (
			SELECT id
			FROM categories
			WHERE id = ANY($1) AND ($2::boolean = false OR is_active = true)
			UNION
			SELECT c.id
			FROM categories c
			JOIN cat_tree ct ON c.parent_id = ct.id
			WHERE ($2::boolean = false OR c.is_active = true)
		)
		SELECT id FROM cat_tree
	`
	rows, err := db.Query(ctx, query, uniqueIDs, activeOnly)
	if err != nil {
		return nil, fmt.Errorf("failed to resolve category subtree IDs: %w", err)
	}
	defer rows.Close()

	var result []uuid.UUID
	for rows.Next() {
		var id uuid.UUID
		if err := rows.Scan(&id); err != nil {
			return nil, fmt.Errorf("failed to scan category subtree ID: %w", err)
		}
		result = append(result, id)
	}
	return result, rows.Err()
}

// ValidateProductsBelongToSellerTx checks that all provided product IDs exist and belong strictly to sellerID.
func (r *Repository) ValidateProductsBelongToSellerTx(ctx context.Context, db DBExecutor, sellerID uuid.UUID, productIDs []uuid.UUID) (bool, error) {
	if len(productIDs) == 0 {
		return true, nil
	}
	uniqueMap := make(map[uuid.UUID]struct{}, len(productIDs))
	for _, id := range productIDs {
		uniqueMap[id] = struct{}{}
	}
	uniqueIDs := make([]uuid.UUID, 0, len(uniqueMap))
	for id := range uniqueMap {
		uniqueIDs = append(uniqueIDs, id)
	}

	query := `SELECT COUNT(*) FROM products WHERE seller_id = $1 AND id = ANY($2)`
	var count int
	if err := db.QueryRow(ctx, query, sellerID, uniqueIDs).Scan(&count); err != nil {
		return false, fmt.Errorf("failed to validate product ownership: %w", err)
	}
	return count == len(uniqueIDs), nil
}

// CountPromoUsageCommittedTx returns both reserved and consumed usage counts for a promo.
func (r *Repository) CountPromoUsageCommittedTx(ctx context.Context, db DBExecutor, promoID uuid.UUID) (int, int, error) {
	query := `
		SELECT
			COALESCE(COUNT(CASE WHEN status = 'reserved' THEN 1 END), 0),
			COALESCE(COUNT(CASE WHEN status = 'consumed' THEN 1 END), 0)
		FROM promo_code_usages
		WHERE promo_code_id = $1 AND status IN ('reserved', 'consumed')
	`
	var reserved, consumed int
	err := db.QueryRow(ctx, query, promoID).Scan(&reserved, &consumed)
	if err != nil {
		return 0, 0, fmt.Errorf("failed to count committed promo usages: %w", err)
	}
	return reserved, consumed, nil
}

// GetMaxCustomerCommittedUsageTx returns the maximum active usage count by any single customer for a promo.
func (r *Repository) GetMaxCustomerCommittedUsageTx(ctx context.Context, db DBExecutor, promoID uuid.UUID) (int, error) {
	query := `
		SELECT COALESCE(MAX(cnt), 0)
		FROM (
			SELECT COUNT(*) AS cnt
			FROM promo_code_usages
			WHERE promo_code_id = $1 AND status IN ('reserved', 'consumed')
			GROUP BY user_id
		) s
	`
	var maxCount int
	err := db.QueryRow(ctx, query, promoID).Scan(&maxCount)
	if err != nil {
		return 0, fmt.Errorf("failed to get max customer usage: %w", err)
	}
	return maxCount, nil
}

// UpdatePromoCodeMutableFieldsTx saves updated mutable fields to promo_codes and syncs backing campaign dates.
func (r *Repository) UpdatePromoCodeMutableFieldsTx(ctx context.Context, db DBExecutor, p *PromoCode) error {
	now := time.Now().UTC()
	p.UpdatedAt = now

	query := `
		UPDATE promo_codes
		SET is_active = $1, starts_at = $2, ends_at = $3,
		    global_usage_limit = $4, per_customer_usage_limit = $5,
		    updated_at = $6
		WHERE id = $7 AND seller_id = $8
	`
	tag, err := db.Exec(ctx, query,
		p.IsActive, p.StartsAt, p.EndsAt,
		p.GlobalUsageLimit, p.PerCustomerUsageLimit,
		now, p.ID, p.SellerID,
	)
	if err != nil {
		return fmt.Errorf("failed to update promo code: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrPromoCodeNotFound
	}

	campaignQuery := `
		UPDATE marketing_campaigns
		SET starts_at = $1, ends_at = $2, updated_at = $3
		WHERE id = $4 AND seller_id = $5
	`
	_, err = db.Exec(ctx, campaignQuery, p.StartsAt, p.EndsAt, now, p.CampaignID, p.SellerID)
	if err != nil {
		return fmt.Errorf("failed to sync campaign dates: %w", err)
	}
	return nil
}
