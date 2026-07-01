package catalog

import (
	"context"
	"strings"
	"time"

	"github.com/google/uuid"
)

type Service struct {
	repo *Repository
}

func NewService(repo *Repository) *Service {
	return &Service{repo: repo}
}

func (s *Service) CreateCategory(ctx context.Context, req CreateCategoryRequest) (Category, error) {
	c := &Category{
		ID:          uuid.New(),
		ParentID:    req.ParentID,
		Name:        req.Name,
		Slug:        req.Slug,
		Description: req.Description,
		SortOrder:   req.SortOrder,
		IsActive:    true,
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
	}

	if err := s.repo.CreateCategory(ctx, c); err != nil {
		if strings.Contains(err.Error(), "SQLSTATE 23505") {
			return Category{}, ErrDuplicateSlug
		}
		return Category{}, err
	}
	return *c, nil
}

func (s *Service) ListCategories(ctx context.Context) (CategoryListResponse, error) {
	items, err := s.repo.ListCategories(ctx)
	if err != nil {
		return CategoryListResponse{}, err
	}
	if items == nil {
		items = []Category{}
	}
	return CategoryListResponse{Items: items}, nil
}

func (s *Service) CreateBrand(ctx context.Context, req CreateBrandRequest) (Brand, error) {
	b := &Brand{
		ID:          uuid.New(),
		Name:        req.Name,
		Slug:        req.Slug,
		Description: req.Description,
		LogoURL:     req.LogoURL,
		IsActive:    true,
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
	}

	if err := s.repo.CreateBrand(ctx, b); err != nil {
		if strings.Contains(err.Error(), "SQLSTATE 23505") {
			return Brand{}, ErrDuplicateSlug
		}
		return Brand{}, err
	}
	return *b, nil
}

func (s *Service) ListBrands(ctx context.Context) (BrandListResponse, error) {
	items, err := s.repo.ListBrands(ctx)
	if err != nil {
		return BrandListResponse{}, err
	}
	if items == nil {
		items = []Brand{}
	}
	return BrandListResponse{Items: items}, nil
}
