package auctions

import (
	"encoding/json"
	"time"

	"github.com/google/uuid"
)

type AuctionStatus string

const (
	AuctionStatusDraft     AuctionStatus = "draft"
	AuctionStatusScheduled AuctionStatus = "scheduled"
	AuctionStatusLive      AuctionStatus = "live"
	AuctionStatusEnded     AuctionStatus = "ended"
	AuctionStatusCancelled AuctionStatus = "cancelled"
	AuctionStatusPaused    AuctionStatus = "paused"
)

type LotStatus string

const (
	LotStatusDraft              LotStatus = "draft"
	LotStatusActive             LotStatus = "active"
	LotStatusEndedNoBids        LotStatus = "ended_no_bids"
	LotStatusWonPendingPayment  LotStatus = "won_pending_payment"
	LotStatusPaid               LotStatus = "paid"
	LotStatusUnpaidManualReview LotStatus = "unpaid_manual_review"
	LotStatusMovedToDirectSale  LotStatus = "moved_to_direct_sale"
	LotStatusCancelled          LotStatus = "cancelled"
)

type NoBidsPolicy string

const (
	NoBidsPolicyManualReview NoBidsPolicy = "manual_review"
	NoBidsPolicyAutoDirect   NoBidsPolicy = "auto_direct_sale"
)

type UnpaidWinnerPolicy string

const (
	UnpaidWinnerPolicyManualReview UnpaidWinnerPolicy = "manual_review"
	UnpaidWinnerPolicySecondBidder UnpaidWinnerPolicy = "offer_second_bidder"
)

type AuctionEvent struct {
	ID                                  uuid.UUID          `db:"id" json:"id"`
	Title                               string             `db:"title" json:"title"`
	Description                         *string            `db:"description" json:"description"`
	Status                              AuctionStatus      `db:"status" json:"status"`
	StartsAt                            time.Time          `db:"starts_at" json:"startsAt"`
	EndsAt                              time.Time          `db:"ends_at" json:"endsAt"`
	BidStepCents                        int64              `db:"bid_step_cents" json:"bidStepCents"`
	PaymentDeadlineHours                int                `db:"payment_deadline_hours" json:"paymentDeadlineHours"`
	AntiSnipingEnabled                  bool               `db:"anti_sniping_enabled" json:"antiSnipingEnabled"`
	AntiSnipingTriggerSeconds           int                `db:"anti_sniping_trigger_seconds" json:"antiSnipingTriggerSeconds"`
	AntiSnipingExtensionSeconds         int                `db:"anti_sniping_extension_seconds" json:"antiSnipingExtensionSeconds"`
	MaxBidsPerUserPerLotPerMinute       int                `db:"max_bids_per_user_per_lot_per_minute" json:"maxBidsPerUserPerLotPerMinute"`
	MaxRejectedBidsPerUserPerMinute     int                `db:"max_rejected_bids_per_user_per_minute" json:"maxRejectedBidsPerUserPerMinute"`
	NoBidsPolicy                        NoBidsPolicy       `db:"no_bids_policy" json:"noBidsPolicy"`
	UnpaidWinnerPolicy                  UnpaidWinnerPolicy `db:"unpaid_winner_policy" json:"unpaidWinnerPolicy"`
	IsPublic                            bool               `db:"is_public" json:"isPublic"`
	ShowOnHomepage                      bool               `db:"show_on_homepage" json:"showOnHomepage"`
	HighlightInNav                      bool               `db:"highlight_in_nav" json:"highlightInNav"`
	BiddingEnabled                      bool               `db:"bidding_enabled" json:"biddingEnabled"`
	CreatedBy                           *uuid.UUID         `db:"created_by" json:"createdBy"`
	CreatedAt                           time.Time          `db:"created_at" json:"createdAt"`
	UpdatedAt                           time.Time          `db:"updated_at" json:"updatedAt"`

	// Joined
	Lots []AuctionLot `json:"lots,omitempty"`
}

type AuctionLot struct {
	ID                    uuid.UUID  `db:"id" json:"id"`
	AuctionID             uuid.UUID  `db:"auction_id" json:"auctionId"`
	Title                 string     `db:"title" json:"title"`
	Description           *string    `db:"description" json:"description"`
	ImageURL              *string    `db:"image_url" json:"imageUrl"`
	StartPriceCents       int64      `db:"start_price_cents" json:"startPriceCents"`
	CurrentBidCents       *int64     `db:"current_bid_cents" json:"currentBidCents"`
	BidStepCents          int64      `db:"bid_step_cents" json:"bidStepCents"`
	CurrentWinnerUserID   *uuid.UUID `db:"current_winner_user_id" json:"currentWinnerUserId"`
	Status                LotStatus  `db:"status" json:"status"`
	OrderID               *uuid.UUID `db:"order_id" json:"orderId"`
	PaymentDeadlineAt     *time.Time `db:"payment_deadline_at" json:"paymentDeadlineAt"`
	CanRelaunch           bool       `db:"can_relaunch" json:"canRelaunch"`
	CanMoveToDirectSale   bool       `db:"can_move_to_direct_sale" json:"canMoveToDirectSale"`
	DirectSalePriceCents  *int64     `db:"direct_sale_price_cents" json:"directSalePriceCents"`
	DirectSaleProductID   *uuid.UUID `db:"direct_sale_product_id" json:"directSaleProductId"`
	AdminNote             *string    `db:"admin_note" json:"adminNote"`
	CreatedAt             time.Time  `db:"created_at" json:"createdAt"`
	UpdatedAt             time.Time  `db:"updated_at" json:"updatedAt"`

	// Joined
	Images     []AuctionLotImage     `json:"images,omitempty"`
	Attributes []AuctionLotAttribute `json:"attributes,omitempty"`
}

type AuctionLotImage struct {
	ID        uuid.UUID `db:"id" json:"id"`
	LotID     uuid.UUID `db:"lot_id" json:"lotId"`
	ImageURL  string    `db:"image_url" json:"imageUrl"`
	SortOrder int       `db:"sort_order" json:"sortOrder"`
	IsPrimary bool      `db:"is_primary" json:"isPrimary"`
	CreatedAt time.Time `db:"created_at" json:"createdAt"`
}

type AuctionLotAttribute struct {
	ID        uuid.UUID `db:"id" json:"id"`
	LotID     uuid.UUID `db:"lot_id" json:"lotId"`
	Name      string    `db:"name" json:"name"`
	Value     string    `db:"value" json:"value"`
	SortOrder int       `db:"sort_order" json:"sortOrder"`
}

type AuctionBid struct {
	ID             uuid.UUID  `db:"id" json:"id"`
	AuctionID      uuid.UUID  `db:"auction_id" json:"auctionId"`
	LotID          uuid.UUID  `db:"lot_id" json:"lotId"`
	UserID         uuid.UUID  `db:"user_id" json:"userId"`
	AmountCents    int64      `db:"amount_cents" json:"amountCents"`
	IdempotencyKey *string    `db:"idempotency_key" json:"idempotencyKey"`
	CreatedAt      time.Time  `db:"created_at" json:"createdAt"`
}

type AuctionLog struct {
	ID          uuid.UUID       `db:"id" json:"id"`
	AuctionID   *uuid.UUID      `db:"auction_id" json:"auctionId"`
	LotID       *uuid.UUID      `db:"lot_id" json:"lotId"`
	ActorUserID *uuid.UUID      `db:"actor_user_id" json:"actorUserId"`
	Action      string          `db:"action" json:"action"`
	Metadata    json.RawMessage `db:"metadata" json:"metadata"`
	CreatedAt   time.Time       `db:"created_at" json:"createdAt"`
}

type AuctionSuspiciousEvent struct {
	ID        uuid.UUID       `db:"id" json:"id"`
	AuctionID *uuid.UUID      `db:"auction_id" json:"auctionId"`
	LotID     *uuid.UUID      `db:"lot_id" json:"lotId"`
	UserID    *uuid.UUID      `db:"user_id" json:"userId"`
	Reason    string          `db:"reason" json:"reason"`
	Metadata  json.RawMessage `db:"metadata" json:"metadata"`
	CreatedAt time.Time       `db:"created_at" json:"createdAt"`
}
