package auctions

import (
	"time"
)

type BidRequest struct {
	AmountCents         *int64  `json:"amountCents"`
	IdempotencyKey      *string `json:"idempotencyKey"`
	ClientKnownBidCents *int64  `json:"clientKnownBidCents"`
}

type BidResponse struct {
	Success         bool      `json:"success"`
	NewCurrentBid   int64     `json:"newCurrentBid"`
	IsLeading       bool      `json:"isLeading"`
	LotStatus       LotStatus `json:"lotStatus"`
	EndsAt          time.Time `json:"endsAt"`
	ExtensionApplied bool     `json:"extensionApplied"`
}

type AdminCreateAuctionRequest struct {
	Title                               string             `json:"title"`
	Description                         *string            `json:"description"`
	StartsAt                            time.Time          `json:"startsAt"`
	EndsAt                              time.Time          `json:"endsAt"`
	BidStepCents                        int64              `json:"bidStepCents"`
	PaymentDeadlineHours                int                `json:"paymentDeadlineHours"`
	AntiSnipingEnabled                  bool               `json:"antiSnipingEnabled"`
	AntiSnipingTriggerSeconds           int                `json:"antiSnipingTriggerSeconds"`
	AntiSnipingExtensionSeconds         int                `json:"antiSnipingExtensionSeconds"`
	MaxBidsPerUserPerLotPerMinute       int                `json:"maxBidsPerUserPerLotPerMinute"`
	MaxRejectedBidsPerUserPerMinute     int                `json:"maxRejectedBidsPerUserPerMinute"`
	NoBidsPolicy                        NoBidsPolicy       `json:"noBidsPolicy"`
	UnpaidWinnerPolicy                  UnpaidWinnerPolicy `json:"unpaidWinnerPolicy"`
	IsPublic                            bool               `json:"isPublic"`
	ShowOnHomepage                      bool               `json:"showOnHomepage"`
	HighlightInNav                      bool               `json:"highlightInNav"`
	BiddingEnabled                      bool               `json:"biddingEnabled"`
}

type AdminUpdateAuctionRequest struct {
	Title                               *string             `json:"title"`
	Description                         *string             `json:"description"`
	StartsAt                            *time.Time          `json:"startsAt"`
	EndsAt                              *time.Time          `json:"endsAt"`
	BidStepCents                        *int64              `json:"bidStepCents"`
	PaymentDeadlineHours                *int                `json:"paymentDeadlineHours"`
	AntiSnipingEnabled                  *bool               `json:"antiSnipingEnabled"`
	AntiSnipingTriggerSeconds           *int                `json:"antiSnipingTriggerSeconds"`
	AntiSnipingExtensionSeconds         *int                `json:"antiSnipingExtensionSeconds"`
	MaxBidsPerUserPerLotPerMinute       *int                `json:"maxBidsPerUserPerLotPerMinute"`
	MaxRejectedBidsPerUserPerMinute     *int                `json:"maxRejectedBidsPerUserPerMinute"`
	NoBidsPolicy                        *NoBidsPolicy       `json:"noBidsPolicy"`
	UnpaidWinnerPolicy                  *UnpaidWinnerPolicy `json:"unpaidWinnerPolicy"`
	IsPublic                            *bool               `json:"isPublic"`
	ShowOnHomepage                      *bool               `json:"showOnHomepage"`
	HighlightInNav                      *bool               `json:"highlightInNav"`
	BiddingEnabled                      *bool               `json:"biddingEnabled"`
}

type AdminCreateLotRequest struct {
	Title                string                `json:"title"`
	Description          *string               `json:"description"`
	StartPriceCents      int64                 `json:"startPriceCents"`
	BidStepCents         int64                 `json:"bidStepCents"`
	CanRelaunch          bool                  `json:"canRelaunch"`
	CanMoveToDirectSale  bool                  `json:"canMoveToDirectSale"`
	DirectSalePriceCents *int64                `json:"directSalePriceCents"`
	AdminNote            *string               `json:"adminNote"`
	Images               []ImageDTO            `json:"images"`
	Attributes           []AttributeDTO        `json:"attributes"`
}

type AdminUpdateLotRequest struct {
	Title                *string               `json:"title"`
	Description          *string               `json:"description"`
	StartPriceCents      *int64                `json:"startPriceCents"`
	BidStepCents         *int64                `json:"bidStepCents"`
	CanRelaunch          *bool                 `json:"canRelaunch"`
	CanMoveToDirectSale  *bool                 `json:"canMoveToDirectSale"`
	DirectSalePriceCents *int64                `json:"directSalePriceCents"`
	AdminNote            *string               `json:"adminNote"`
}

type ImageDTO struct {
	ImageURL  string `json:"imageUrl"`
	SortOrder int    `json:"sortOrder"`
	IsPrimary bool   `json:"isPrimary"`
}

type AttributeDTO struct {
	Name      string `json:"name"`
	Value     string `json:"value"`
	SortOrder int    `json:"sortOrder"`
}
