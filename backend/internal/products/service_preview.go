package products

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"os"
	"sync"
	"time"

	"github.com/eirfefrggrvgrgrtbg/shop-zamk/backend/internal/platform/redis"
	"github.com/google/uuid"
)

type previewTokenItem struct {
	ProductID uuid.UUID
	ExpiresAt time.Time
}

var previewMemoryStore sync.Map

func (s *Service) WithRedis(r *redis.Client) *Service {
	s.rdb = r
	return s
}

func (s *Service) StartProductReview(ctx context.Context, adminID uuid.UUID, productID uuid.UUID) error {
	p, err := s.repo.GetProductByID(ctx, productID)
	if err != nil {
		return err
	}
	if p.Status != StatusPendingModeration {
		return ErrInvalidStatusTransition
	}
	now := time.Now()
	p.Status = StatusInReview
	p.AssignedAdminUserID = &adminID
	p.ReviewStartedAt = &now
	err = s.repo.UpdateProductStatus(ctx, p)
	return err
}

func isMemoryFallbackAllowed() bool {
	env := os.Getenv("APP_ENV")
	if env == "production" || env == "prod" {
		return os.Getenv("ALLOW_PREVIEW_MEMORY_FALLBACK") == "true"
	}
	return true // dev/test: always allowed
}

func (s *Service) CreateProductPreviewLink(ctx context.Context, adminID uuid.UUID, productID uuid.UUID) (string, error) {
	bytes := make([]byte, 32)
	if _, err := rand.Read(bytes); err != nil {
		return "", err
	}
	token := hex.EncodeToString(bytes) // 64 hex characters

	ttl := 15 * time.Minute
	expiresAt := time.Now().Add(ttl)

	redisSaved := false
	if s.rdb != nil && s.rdb.Client != nil {
		err := s.rdb.Client.Set(ctx, "preview:"+token, productID.String(), ttl).Err()
		if err == nil {
			redisSaved = true
		}
	}

	if !redisSaved && !isMemoryFallbackAllowed() {
		return "", ErrRedisUnavailable
	}

	// Save to thread-safe memory store in dev/test/fallback
	previewMemoryStore.Store(token, previewTokenItem{
		ProductID: productID,
		ExpiresAt: expiresAt,
	})

	return token, nil
}

func (s *Service) GetProductPreviewByToken(ctx context.Context, token string) (*PublicProduct, error) {
	// Validate token format (MUST be exactly 64 hex characters)
	if len(token) != 64 {
		return nil, ErrInvalidPreviewToken
	}
	if _, err := hex.DecodeString(token); err != nil {
		return nil, ErrInvalidPreviewToken
	}

	var targetProductID uuid.UUID
	found := false
	redisAttempted := false

	// Check Redis first if available
	if s.rdb != nil && s.rdb.Client != nil {
		redisAttempted = true
		val, err := s.rdb.Client.Get(ctx, "preview:"+token).Result()
		if err == nil && val != "" {
			if parsed, pErr := uuid.Parse(val); pErr == nil {
				targetProductID = parsed
				found = true
			}
		}
	}

	// Fallback to memory store if allowed
	if !found && (isMemoryFallbackAllowed() || !redisAttempted) {
		if val, ok := previewMemoryStore.Load(token); ok {
			item := val.(previewTokenItem)
			if time.Now().Before(item.ExpiresAt) {
				targetProductID = item.ProductID
				found = true
			} else {
				previewMemoryStore.Delete(token)
			}
		}
	}

	if !found {
		if redisAttempted && !isMemoryFallbackAllowed() {
			return nil, ErrPreviewUnavailable
		}
		return nil, ErrPreviewUnavailable
	}

	p, err := s.repo.GetProductByID(ctx, targetProductID)
	if err != nil {
		return nil, ErrProductNotFound
	}

	sellerName := ""
	if p.SellerName != nil {
		sellerName = *p.SellerName
	}
	sellerSlug := ""
	if p.SellerSlug != nil {
		sellerSlug = *p.SellerSlug
	}

	pub := &PublicProduct{
		ID:            p.ID,
		Title:         p.Title,
		Slug:          p.Slug,
		Description:   p.Description,
		PriceCents:    p.PriceCents,
		OldPriceCents: p.OldPriceCents,
		Currency:      p.Currency,
		MainImageURL:  p.MainImageURL,
		AverageRating: p.AverageRating,
		ReviewsCount:  p.ReviewsCount,
		CreatedAt:     p.CreatedAt,
		SellerID:      p.SellerID,
		SellerName:    sellerName,
		SellerSlug:    sellerSlug,
		CategoryID:    p.CategoryID,
		BrandID:       p.BrandID,
		Status:        p.Status,
		MaterialComposition: p.MaterialComposition,
		SizeChart:           p.SizeChart,

	}

	variants, err := s.repo.GetProductVariants(ctx, p.ID)
	if err == nil {
		for _, v := range variants {
			pub.Variants = append(pub.Variants, PublicProductVariant{
				ID:         v.ID,
				ProductID:  v.ProductID,
				Size:       v.Size,
				Color:      v.Color,
				SellerSKU:   v.SellerSKU,
				ColorID:     v.ColorID,
				SizeValueID: v.SizeValueID,
				ColorName: v.ColorName,
				ColorHex: v.ColorHex,
				ShadeName:   v.ShadeName,
				OptionValues: v.OptionValues,
				PriceCents: v.PriceCents,
				IsActive:   v.IsActive,
				InStock:    v.InStock,
			})
		}
	}

	images, err := s.repo.GetProductImages(ctx, p.ID)
	if err == nil {
		for _, img := range images {
			pub.Images = append(pub.Images, PublicProductImage{
				ID:        img.ID,
				ProductID: img.ProductID,
				ImageURL:  img.ImageURL,
				AltText:   img.AltText,
				SortOrder: img.SortOrder,
				ColorID: img.ColorID,
			})
		}
	}

	return pub, nil
}
