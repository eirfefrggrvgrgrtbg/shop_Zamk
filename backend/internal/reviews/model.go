package reviews

import (
	"time"

	"github.com/google/uuid"
)

type ProductReview struct {
	ID                uuid.UUID  `json:"id" db:"id"`
	ProductID         uuid.UUID  `json:"productId" db:"product_id"`
	ProductVariantID  *uuid.UUID `json:"productVariantId,omitempty" db:"product_variant_id"`
	OrderID           uuid.UUID  `json:"orderId" db:"order_id"`
	OrderItemID       uuid.UUID  `json:"orderItemId" db:"order_item_id"`
	UserID            uuid.UUID  `json:"userId" db:"user_id"`
	SellerID          uuid.UUID  `json:"sellerId" db:"seller_id"`
	Rating            int        `json:"rating" db:"rating"`
	Title             *string    `json:"title,omitempty" db:"title"`
	Comment           *string    `json:"comment,omitempty" db:"comment"`
	Status            string     `json:"status" db:"status"`
	CreatedAt         time.Time  `json:"createdAt" db:"created_at"`
	UpdatedAt         time.Time  `json:"updatedAt" db:"updated_at"`
	PublishedAt       *time.Time `json:"publishedAt,omitempty" db:"published_at"`
	RejectedAt        *time.Time `json:"rejectedAt,omitempty" db:"rejected_at"`
	ModerationComment *string    `json:"moderationComment,omitempty" db:"moderation_comment"`
}

type ProductReviewModerationLog struct {
	ID          uuid.UUID  `json:"id" db:"id"`
	ReviewID    uuid.UUID  `json:"reviewId" db:"review_id"`
	AdminUserID *uuid.UUID `json:"adminUserId,omitempty" db:"admin_user_id"`
	FromStatus  *string    `json:"fromStatus,omitempty" db:"from_status"`
	ToStatus    string     `json:"toStatus" db:"to_status"`
	Comment     *string    `json:"comment,omitempty" db:"comment"`
	CreatedAt   time.Time  `json:"createdAt" db:"created_at"`
}
