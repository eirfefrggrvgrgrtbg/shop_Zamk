package merchandising

import (
	"context"
	"errors"
	"time"

	"github.com/eirfefrggrvgrgrtbg/shop-zamk/backend/internal/products"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

var (
	ErrDuplicateProductID = errors.New("duplicate product IDs in request")
	ErrInvalidDates       = errors.New("starts_at must be before ends_at")
)

type Service struct {
	repo *Repository
	pool *pgxpool.Pool
}

func NewService(repo *Repository, pool *pgxpool.Pool) *Service {
	return &Service{
		repo: repo,
		pool: pool,
	}
}

func (s *Service) CreateCollection(ctx context.Context, req *CreateCollectionRequest) (*Collection, error) {
	if req.StartsAt != nil && req.EndsAt != nil && !req.StartsAt.Before(*req.EndsAt) {
		return nil, ErrInvalidDates
	}

	c := &Collection{
		ID:            uuid.New(),
		Slug:          req.Slug,
		Title:         req.Title,
		Description:   req.Description,
		CoverImageURL: req.CoverImageURL,
		IsActive:      req.IsActive,
		StartsAt:      req.StartsAt,
		EndsAt:        req.EndsAt,
		CreatedAt:     time.Now(),
		UpdatedAt:     time.Now(),
	}

	if err := s.repo.CreateCollection(ctx, c); err != nil {
		return nil, err
	}
	return c, nil
}

func (s *Service) UpdateCollection(ctx context.Context, id uuid.UUID, req *UpdateCollectionRequest) (*Collection, error) {
	if req.StartsAt != nil && req.EndsAt != nil && !req.StartsAt.Before(*req.EndsAt) {
		return nil, ErrInvalidDates
	}

	c, err := s.repo.GetCollectionByID(ctx, id)
	if err != nil {
		return nil, err
	}

	c.Slug = req.Slug
	c.Title = req.Title
	c.Description = req.Description
	c.CoverImageURL = req.CoverImageURL
	c.IsActive = req.IsActive
	c.StartsAt = req.StartsAt
	c.EndsAt = req.EndsAt
	c.UpdatedAt = time.Now()

	if err := s.repo.UpdateCollection(ctx, c); err != nil {
		return nil, err
	}
	return c, nil
}

func (s *Service) ListAdminCollections(ctx context.Context) ([]Collection, error) {
	return s.repo.ListCollectionsAdmin(ctx)
}

func (s *Service) GetAdminCollectionDetail(ctx context.Context, id uuid.UUID) (*AdminCollectionDetailResponse, error) {
	c, err := s.repo.GetCollectionByID(ctx, id)
	if err != nil {
		return nil, err
	}

	items, err := s.repo.GetAdminCollectionItems(ctx, id)
	if err != nil {
		return nil, err
	}
	if items == nil {
		items = []AdminCollectionItem{}
	}

	return &AdminCollectionDetailResponse{
		Collection: *c,
		Items:      items,
	}, nil
}

func (s *Service) ReplaceCollectionItems(ctx context.Context, id uuid.UUID, productIDs []uuid.UUID) error {
	seen := make(map[uuid.UUID]bool)
	for _, pid := range productIDs {
		if seen[pid] {
			return ErrDuplicateProductID
		}
		seen[pid] = true
	}

	// Transactional replacement
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	txRepo := s.repo.WithTx(tx)
	if err := txRepo.ReplaceCollectionItems(ctx, id, productIDs); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

func (s *Service) ListPublicCollections(ctx context.Context, asOf ...time.Time) ([]Collection, error) {
	refTime := time.Now()
	if len(asOf) > 0 {
		refTime = asOf[0]
	}
	return s.repo.ListActiveCollectionsPublic(ctx, refTime)
}

func (s *Service) GetPublicCollectionDetail(ctx context.Context, slug string, asOf ...time.Time) (*PublicCollectionDetailResponse, error) {
	c, err := s.repo.GetCollectionBySlug(ctx, slug)
	if err != nil {
		return nil, err
	}

	refTime := time.Now()
	if len(asOf) > 0 {
		refTime = asOf[0]
	}

	// Canonical time-window eligibility:
	// is_active = true
	// AND (starts_at IS NULL OR starts_at <= refTime)
	// AND (ends_at IS NULL OR ends_at > refTime)
	if !c.IsActive {
		return nil, ErrCollectionNotFound
	}
	if c.StartsAt != nil && c.StartsAt.After(refTime) {
		return nil, ErrCollectionNotFound
	}
	if c.EndsAt != nil && !c.EndsAt.After(refTime) {
		return nil, ErrCollectionNotFound
	}

	prods, err := s.repo.GetPublicCollectionItems(ctx, c.ID)
	if err != nil {
		return nil, err
	}

	pubProds := make([]products.PublicProduct, 0, len(prods))
	for _, p := range prods {
		pubProds = append(pubProds, products.MapToPublicProduct(p))
	}

	return &PublicCollectionDetailResponse{
		ID:            c.ID,
		Slug:          c.Slug,
		Title:         c.Title,
		Description:   c.Description,
		CoverImageURL: c.CoverImageURL,
		Products:      pubProds,
	}, nil
}
