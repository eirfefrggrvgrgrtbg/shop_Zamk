package favorites

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
)

var ErrBrandNotFound = errors.New("brand not found or not active")

// FavoriteBrand exposes only public storefront fields of a brand.
type FavoriteBrand struct {
	ID          uuid.UUID `json:"id"`
	Name        string    `json:"name"`
	Slug        string    `json:"slug"`
	Description *string   `json:"description,omitempty"`
	LogoURL     *string   `json:"logoUrl,omitempty"`
	AddedAt     time.Time `json:"addedAt"`
}

// AddFavoriteBrand is idempotent and race-safe: the unique (user_id, brand_id) pair absorbs duplicates.
func (r *Repository) AddFavoriteBrand(ctx context.Context, userID, brandID uuid.UUID) error {
	cmd, err := r.db.Exec(ctx, `
		INSERT INTO favorite_brands (user_id, brand_id)
		SELECT $1, b.id FROM brands b WHERE b.id = $2 AND b.is_active = true
		ON CONFLICT (user_id, brand_id) DO NOTHING
	`, userID, brandID)
	if err != nil {
		return err
	}
	if cmd.RowsAffected() > 0 {
		return nil
	}
	// Nothing inserted: either already saved (fine) or the brand is missing/inactive.
	var exists bool
	if err := r.db.QueryRow(ctx,
		`SELECT EXISTS(SELECT 1 FROM favorite_brands WHERE user_id = $1 AND brand_id = $2)`,
		userID, brandID).Scan(&exists); err != nil {
		return err
	}
	if !exists {
		return ErrBrandNotFound
	}
	return nil
}

func (r *Repository) RemoveFavoriteBrand(ctx context.Context, userID, brandID uuid.UUID) error {
	_, err := r.db.Exec(ctx, `DELETE FROM favorite_brands WHERE user_id = $1 AND brand_id = $2`, userID, brandID)
	return err
}

// ListFavoriteBrands returns only the caller's favorites whose brand is still active.
func (r *Repository) ListFavoriteBrands(ctx context.Context, userID uuid.UUID) ([]FavoriteBrand, error) {
	rows, err := r.db.Query(ctx, `
		SELECT b.id, b.name, b.slug, b.description, b.logo_url, fb.created_at
		FROM favorite_brands fb
		INNER JOIN brands b ON b.id = fb.brand_id
		WHERE fb.user_id = $1 AND b.is_active = true
		ORDER BY fb.created_at DESC, b.name ASC
	`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	results := []FavoriteBrand{}
	for rows.Next() {
		var fb FavoriteBrand
		if err := rows.Scan(&fb.ID, &fb.Name, &fb.Slug, &fb.Description, &fb.LogoURL, &fb.AddedAt); err != nil {
			return nil, err
		}
		results = append(results, fb)
	}
	return results, rows.Err()
}

func (s *Service) AddFavoriteBrand(ctx context.Context, userID, brandID uuid.UUID) error {
	return s.repo.AddFavoriteBrand(ctx, userID, brandID)
}

func (s *Service) RemoveFavoriteBrand(ctx context.Context, userID, brandID uuid.UUID) error {
	return s.repo.RemoveFavoriteBrand(ctx, userID, brandID)
}

func (s *Service) ListFavoriteBrands(ctx context.Context, userID uuid.UUID) ([]FavoriteBrand, error) {
	return s.repo.ListFavoriteBrands(ctx, userID)
}

func userIDFromRequest(r *http.Request) (uuid.UUID, bool) {
	id, ok := r.Context().Value("userID").(uuid.UUID)
	return id, ok
}

func (h *Handler) ListFavoriteBrands(w http.ResponseWriter, r *http.Request) {
	userID, ok := userIDFromRequest(r)
	if !ok {
		h.writeError(w, http.StatusUnauthorized, "unauthorized", "Missing user context")
		return
	}
	items, err := h.service.ListFavoriteBrands(r.Context(), userID)
	if err != nil {
		h.writeError(w, http.StatusInternalServerError, "internal_error", "Failed to list favorite brands")
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]any{"items": items})
}

func (h *Handler) AddFavoriteBrand(w http.ResponseWriter, r *http.Request) {
	userID, ok := userIDFromRequest(r)
	if !ok {
		h.writeError(w, http.StatusUnauthorized, "unauthorized", "Missing user context")
		return
	}
	brandID, err := uuid.Parse(chi.URLParam(r, "brandId"))
	if err != nil {
		h.writeError(w, http.StatusBadRequest, "invalid_id", "Invalid brand ID")
		return
	}
	if err := h.service.AddFavoriteBrand(r.Context(), userID, brandID); err != nil {
		if errors.Is(err, ErrBrandNotFound) {
			h.writeError(w, http.StatusNotFound, "not_found", "Brand not found or not available")
			return
		}
		h.writeError(w, http.StatusInternalServerError, "internal_error", "Failed to add favorite brand")
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
}

func (h *Handler) RemoveFavoriteBrand(w http.ResponseWriter, r *http.Request) {
	userID, ok := userIDFromRequest(r)
	if !ok {
		h.writeError(w, http.StatusUnauthorized, "unauthorized", "Missing user context")
		return
	}
	brandID, err := uuid.Parse(chi.URLParam(r, "brandId"))
	if err != nil {
		h.writeError(w, http.StatusBadRequest, "invalid_id", "Invalid brand ID")
		return
	}
	if err := h.service.RemoveFavoriteBrand(r.Context(), userID, brandID); err != nil {
		h.writeError(w, http.StatusInternalServerError, "internal_error", "Failed to remove favorite brand")
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
}
