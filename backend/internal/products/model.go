package products

import (
	"time"

	"github.com/google/uuid"
)

const (
	StatusDraft             = "draft"
	StatusPendingModeration = "pending_moderation"
	StatusInReview          = "in_review"
	StatusApproved          = "approved"
	StatusPublished         = "published"
	StatusRejected          = "rejected"
	StatusHidden            = "hidden"
	StatusBlocked           = "blocked"
	StatusOutOfStock        = "out_of_stock"
)

type Product struct {
	ID                  uuid.UUID  `json:"id"`
	SellerID            uuid.UUID  `json:"sellerId"`
	SellerSlug          *string    `json:"sellerSlug,omitempty"`
	SellerName          *string    `json:"sellerName,omitempty"`
	SellerOwnerName     *string    `json:"sellerOwnerName,omitempty"`
	SellerOwnerEmail    *string    `json:"sellerOwnerEmail,omitempty"`
	CategoryID          *uuid.UUID `json:"categoryId,omitempty"`
	CategoryName        *string    `json:"categoryName,omitempty"`
	BrandID             *uuid.UUID `json:"brandId,omitempty"`
	BrandName           *string    `json:"brandName,omitempty"`
	Title               string     `json:"title"`
	Slug                string     `json:"slug"`
	Description         *string    `json:"description,omitempty"`
	Status              string     `json:"status"`
	Source              string     `json:"source"`
	Gender              *string    `json:"gender,omitempty"`
	Color               *string    `json:"color,omitempty"`
	Material            *string    `json:"material,omitempty"`
	CareInstructions    *string    `json:"careInstructions,omitempty"`
	PriceCents          int64      `json:"priceCents"`
	OldPriceCents       *int64     `json:"oldPriceCents,omitempty"`
	Currency            string     `json:"currency"`
	MainImageURL        *string    `json:"mainImageUrl,omitempty"`
	AverageRating       float64    `json:"averageRating"`
	ReviewsCount        int        `json:"reviewsCount"`
	MainImageObjectKey  *string    `json:"mainImageObjectKey,omitempty"`
	AssignedAdminUserID *uuid.UUID `json:"assignedAdminUserId,omitempty"`
	AssignedAdminName   *string    `json:"assignedAdminName,omitempty"`
	ReviewStartedAt     *time.Time `json:"reviewStartedAt,omitempty"`
	CreatedAt           time.Time  `json:"createdAt"`
	UpdatedAt           time.Time  `json:"updatedAt"`
	SubmittedAt         *time.Time `json:"submittedAt,omitempty"`
	ApprovedAt          *time.Time `json:"approvedAt,omitempty"`
	PublishedAt         *time.Time `json:"publishedAt,omitempty"`
	RejectedAt          *time.Time `json:"rejectedAt,omitempty"`
	ModerationComment   *string    `json:"moderationComment,omitempty"`
	LiveRevisionID    *uuid.UUID `json:"liveRevisionId,omitempty"`
	InStock             *bool      `json:"inStock,omitempty"`

	// Aggregated DTO fields for Admin & Visibility Rules Engine
	SellerStatus        *string    `json:"sellerStatus,omitempty"`
	SellerIsActive      bool       `json:"sellerIsActive"`
	VariantsCount       int        `json:"variantsCount"`
	ActiveVariantsCount int        `json:"activeVariantsCount"`
	TotalStock          int        `json:"totalStock"`
	ReservedStock       int        `json:"reservedStock"`
	AvailableStock      int        `json:"availableStock"`
	HasInventoryRecord  bool       `json:"hasInventoryRecord"`
	MinPriceCents       *int64     `json:"minPriceCents,omitempty"`
	MaxPriceCents       *int64     `json:"maxPriceCents,omitempty"`
	ActualVisibility    bool       `json:"actualVisibility"`
	VisibilityReasons   []string   `json:"visibilityReasons"`
	StorefrontURL       *string    `json:"storefrontUrl"`

	// Associations
	Variants []ProductVariant `json:"variants,omitempty"`
	Images   []ProductImage   `json:"images,omitempty"`

	// Metrics
	Rating *RatingSummary `json:"rating,omitempty"`
	Attributes []ProductAttributeValue `json:"attributes,omitempty"`
	MaterialComposition []ProductMaterialComposition `json:"materialComposition,omitempty"`
	SizeChart *ProductSizeChart `json:"sizeChart,omitempty"`
}

type RatingSummary struct {
	Average float64 `json:"average"`
	Count   int     `json:"count"`
}

type ProductVariant struct {
	ID                 uuid.UUID `json:"id"`
	ProductID          uuid.UUID `json:"productId"`
	SKU                *string   `json:"sku,omitempty"`
	Size               *string   `json:"size,omitempty"`
	Color              *string   `json:"color,omitempty"`
	OptionValues       map[string]interface{} `json:"optionValues,omitempty"`
	SellerSKU          *string   `json:"sellerSku,omitempty"`
	ColorID            *uuid.UUID `json:"colorId,omitempty"`
	SizeValueID        *uuid.UUID `json:"sizeValueId,omitempty"`
	ShadeName          *string   `json:"shadeName,omitempty"`
	Barcode            *string   `json:"barcode,omitempty"`
	PriceCents         *int64    `json:"priceCents,omitempty"`
	IsActive           bool      `json:"isActive"`
	InStock            *bool     `json:"inStock,omitempty"`
	InitialStock       *int      `json:"initialStock,omitempty"`
	HasInventoryRecord bool      `json:"hasInventoryRecord"`
	TotalStock         int       `json:"totalStock"`
	ReservedStock      int       `json:"reservedStock"`
	AvailableStock     int       `json:"availableStock"`
	CreatedAt          time.Time `json:"createdAt"`
	UpdatedAt          time.Time `json:"updatedAt"`
	Attributes []VariantAttributeValue `json:"attributes,omitempty"`
}

type ProductImage struct {
	ID        uuid.UUID `json:"id"`
	ProductID uuid.UUID `json:"productId"`
	ImageURL  string    `json:"imageUrl"`
	ObjectKey *string   `json:"objectKey,omitempty"`
	AltText   *string   `json:"altText,omitempty"`
	SortOrder int       `json:"sortOrder"`
	ColorID   *uuid.UUID `json:"colorId,omitempty"`
	CreatedAt time.Time `json:"createdAt"`
}

type ProductModerationLog struct {
	ID          uuid.UUID  `json:"id"`
	ProductID   uuid.UUID  `json:"productId"`
	AdminUserID *uuid.UUID `json:"adminUserId,omitempty"`
	AdminName   *string    `json:"adminName,omitempty"`
	FromStatus  *string    `json:"fromStatus,omitempty"`
	ToStatus    string     `json:"toStatus"`
	Comment     *string    `json:"comment,omitempty"`
	CreatedAt   time.Time  `json:"createdAt"`
}

type Color struct {
	ID        uuid.UUID `json:"id"`
	Code      string    `json:"code"`
	NameRU    string    `json:"nameRu"`
	Hex       *string   `json:"hex,omitempty"`
	SortOrder int       `json:"sortOrder"`
	IsActive  bool      `json:"isActive"`
}

type Material struct {
	ID        uuid.UUID `json:"id"`
	Code      string    `json:"code"`
	NameRU    string    `json:"nameRu"`
	SortOrder int       `json:"sortOrder"`
	IsActive  bool      `json:"isActive"`
}

type SizeSystem struct {
	ID       uuid.UUID `json:"id"`
	Code     string    `json:"code"`
	Name     string    `json:"name"`
	IsActive bool      `json:"isActive"`
}

type SizeValue struct {
	ID           uuid.UUID `json:"id"`
	SizeSystemID uuid.UUID `json:"sizeSystemId"`
	Value        string    `json:"value"`
	SortOrder    int       `json:"sortOrder"`
	IsActive     bool      `json:"isActive"`
}

type ProductMaterialComposition struct {
	ProductID  uuid.UUID `json:"productId"`
	MaterialID uuid.UUID `json:"materialId"`
	Percentage float64   `json:"percentage"`
}

type ProductRevision struct {
	ID              uuid.UUID              `json:"id"`
	ProductID       uuid.UUID              `json:"productId"`
	Status          string                 `json:"status"`
	ContentSnapshot map[string]interface{} `json:"contentSnapshot"`
	CreatedAt       time.Time              `json:"createdAt"`
	UpdatedAt       time.Time              `json:"updatedAt"`
}

type AttributeDictionary struct {
	ID        uuid.UUID `json:"id"`
	Code      string    `json:"code"`
	NameRU    string    `json:"nameRu"`
	CreatedAt time.Time `json:"createdAt"`
	UpdatedAt time.Time `json:"updatedAt"`
}

type AttributeDictionaryValue struct {
	ID              uuid.UUID              `json:"id"`
	DictionaryID    uuid.UUID              `json:"dictionaryId"`
	Code            string                 `json:"code"`
	NameRU          string                 `json:"nameRu"`
	DisplayMetadata map[string]interface{} `json:"displayMetadata,omitempty"`
	SortOrder       int                    `json:"sortOrder"`
	IsActive        bool                   `json:"isActive"`
	CreatedAt       time.Time              `json:"createdAt"`
	UpdatedAt       time.Time              `json:"updatedAt"`
}

type ProductAttributeValue struct {
	ID                    uuid.UUID  `json:"id"`
	ProductID             uuid.UUID  `json:"productId"`
	AttributeDefinitionID uuid.UUID  `json:"attributeDefinitionId"`
	EnumValueID           *uuid.UUID `json:"enumValueId,omitempty"`
	TextValue             *string    `json:"textValue,omitempty"`
	NumberValue           *float64   `json:"numberValue,omitempty"`
	BoolValue             *bool      `json:"boolValue,omitempty"`
	CreatedAt             time.Time  `json:"createdAt"`
	UpdatedAt             time.Time  `json:"updatedAt"`
}

type VariantAttributeValue struct {
	ID                    uuid.UUID  `json:"id"`
	ProductVariantID      uuid.UUID  `json:"productVariantId"`
	AttributeDefinitionID uuid.UUID  `json:"attributeDefinitionId"`
	EnumValueID           *uuid.UUID `json:"enumValueId,omitempty"`
	TextValue             *string    `json:"textValue,omitempty"`
	NumberValue           *float64   `json:"numberValue,omitempty"`
	BoolValue             *bool      `json:"boolValue,omitempty"`
	CreatedAt             time.Time  `json:"createdAt"`
	UpdatedAt             time.Time  `json:"updatedAt"`
}

type ProductSizeChart struct {
	ID         uuid.UUID             `json:"id"`
	ProductID  uuid.UUID             `json:"productId"`
	CategoryID uuid.UUID             `json:"categoryId"`
	Rows       []ProductSizeChartRow `json:"rows"`
}

type ProductSizeChartRow struct {
	SizeChartID  uuid.UUID              `json:"sizeChartId"`
	SizeValueID  uuid.UUID              `json:"sizeValueId"`
	Measurements map[string]interface{} `json:"measurements"`
}

type CategorySizeSystem struct {
	CategoryID   uuid.UUID `json:"categoryId"`
	SizeSystemID uuid.UUID `json:"sizeSystemId"`
	IsDefault    bool      `json:"isDefault"`
	SortOrder    int       `json:"sortOrder"`
}
