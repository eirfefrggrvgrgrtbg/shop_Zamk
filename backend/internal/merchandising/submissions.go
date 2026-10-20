package merchandising

// MARKETING.M1.7 — sellers propose their own products for editorial collections, ZAMK staff decide.
//
// A submission never publishes anything by itself: accepting it adds the product to the collection through
// the existing collection_items mechanism (same ordering rules as ReplaceCollectionItems), in the same
// transaction that marks the submission accepted. The editorial decision stays with ZAMK, and accepted
// products can still be removed from the collection later.

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"

	"github.com/eirfefrggrvgrgrtbg/shop-zamk/backend/internal/auth"
	"github.com/eirfefrggrvgrgrtbg/shop-zamk/backend/internal/products"
)

const (
	SubmissionPending  = "pending"
	SubmissionAccepted = "accepted"
	SubmissionRejected = "rejected"

	maxSubmissionTextRunes = 300
)

var (
	ErrSubmissionNotFound      = errors.New("submission not found")
	ErrProductNotEligible      = errors.New("product is not published or not available in the catalog")
	ErrCollectionUnavailable   = errors.New("collection is not published")
	ErrAlreadyInCollection     = errors.New("product is already in the collection")
	ErrDuplicateSubmission     = errors.New("an active submission for this product and collection already exists")
	ErrSubmissionReviewed      = errors.New("submission was already reviewed")
	ErrTextTooLong             = errors.New("text is too long")
	ErrInvalidSubmissionStatus = errors.New("invalid submission status filter")
)

// SellerCollectionOption is a published collection a seller may propose products for.
type SellerCollectionOption struct {
	ID          uuid.UUID `json:"id"`
	Slug        string    `json:"slug"`
	Title       string    `json:"title"`
	Description *string   `json:"description"`
}

type SubmissionProduct struct {
	ID       uuid.UUID `json:"id"`
	Title    string    `json:"title"`
	ImageURL *string   `json:"imageUrl"`
	Status   string    `json:"status"`
}

// SellerSubmission is what a seller sees about their own submission. It never exposes who reviewed it.
type SellerSubmission struct {
	ID              uuid.UUID         `json:"id"`
	CollectionID    uuid.UUID         `json:"collectionId"`
	CollectionTitle string            `json:"collectionTitle"`
	Product         SubmissionProduct `json:"product"`
	Status          string            `json:"status"`
	SellerComment   *string           `json:"sellerComment"`
	RejectionReason *string           `json:"rejectionReason"`
	CreatedAt       time.Time         `json:"createdAt"`
	ReviewedAt      *time.Time        `json:"reviewedAt"`
}

// AdminSubmission is the staff view: brand, product, collection and the review trail.
type AdminSubmission struct {
	SellerSubmission
	SellerID       uuid.UUID `json:"sellerId"`
	BrandName      string    `json:"brandName"`
	CollectionSlug string    `json:"collectionSlug"`
	ReviewerName   *string   `json:"reviewerName"`
}

type CreateSubmissionRequest struct {
	CollectionID uuid.UUID `json:"collectionId"`
	ProductID    uuid.UUID `json:"productId"`
	Comment      string    `json:"comment"`
}

type RejectSubmissionRequest struct {
	Reason string `json:"reason"`
}

func normalizeText(s string) (*string, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return nil, nil
	}
	if utf8.RuneCountInString(s) > maxSubmissionTextRunes {
		return nil, ErrTextTooLong
	}
	return &s, nil
}

func collectionAvailable(c *Collection, now time.Time) bool {
	if !c.IsActive {
		return false
	}
	if c.StartsAt != nil && c.StartsAt.After(now) {
		return false
	}
	if c.EndsAt != nil && !c.EndsAt.After(now) {
		return false
	}
	return true
}

// ---------------------------------------------------------------- repository

const submissionSelect = `
	SELECT cs.id, cs.collection_id, c.title, c.slug, cs.product_id, p.title, p.main_image_url, p.status,
	       cs.status, cs.seller_comment, cs.rejection_reason, cs.created_at, cs.reviewed_at,
	       cs.seller_id, s.brand_name, ru.name
	FROM collection_submissions cs
	JOIN collections c ON c.id = cs.collection_id
	JOIN products p ON p.id = cs.product_id
	JOIN sellers s ON s.id = cs.seller_id
	LEFT JOIN users ru ON ru.id = cs.reviewed_by`

func scanAdminSubmission(row pgx.Row) (*AdminSubmission, error) {
	var a AdminSubmission
	err := row.Scan(&a.ID, &a.CollectionID, &a.CollectionTitle, &a.CollectionSlug, &a.Product.ID, &a.Product.Title,
		&a.Product.ImageURL, &a.Product.Status, &a.Status, &a.SellerComment, &a.RejectionReason, &a.CreatedAt,
		&a.ReviewedAt, &a.SellerID, &a.BrandName, &a.ReviewerName)
	if err != nil {
		return nil, err
	}
	return &a, nil
}

func (r *Repository) productEligibleForSeller(ctx context.Context, productID uuid.UUID, sellerID *uuid.UUID) (owned, eligible bool, err error) {
	query := `
		SELECT (` + products.CanonicalStorefrontEligibilitySQL("p", "s") + `)
		FROM products p JOIN sellers s ON s.id = p.seller_id
		WHERE p.id = $1 AND ($2::uuid IS NULL OR p.seller_id = $2)`
	err = r.db.QueryRow(ctx, query, productID, sellerID).Scan(&eligible)
	if errors.Is(err, pgx.ErrNoRows) {
		return false, false, nil
	}
	if err != nil {
		return false, false, fmt.Errorf("failed to check product eligibility: %w", err)
	}
	return true, eligible, nil
}

func (r *Repository) productInCollection(ctx context.Context, collectionID, productID uuid.UUID) (bool, error) {
	var exists bool
	err := r.db.QueryRow(ctx, `SELECT EXISTS (SELECT 1 FROM collection_items WHERE collection_id = $1 AND product_id = $2)`,
		collectionID, productID).Scan(&exists)
	return exists, err
}

func (r *Repository) insertSubmission(ctx context.Context, id, collectionID, productID, sellerID uuid.UUID, comment *string) error {
	_, err := r.db.Exec(ctx, `
		INSERT INTO collection_submissions (id, collection_id, product_id, seller_id, seller_comment)
		VALUES ($1, $2, $3, $4, $5)`, id, collectionID, productID, sellerID, comment)
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) && pgErr.Code == "23505" {
		return ErrDuplicateSubmission
	}
	return err
}

func (r *Repository) getSubmission(ctx context.Context, id uuid.UUID) (*AdminSubmission, error) {
	a, err := scanAdminSubmission(r.db.QueryRow(ctx, submissionSelect+` WHERE cs.id = $1`, id))
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrSubmissionNotFound
	}
	return a, err
}

func (r *Repository) listSubmissions(ctx context.Context, sellerID *uuid.UUID, status string, limit int) ([]AdminSubmission, error) {
	rows, err := r.db.Query(ctx, submissionSelect+`
		WHERE ($1::uuid IS NULL OR cs.seller_id = $1) AND ($2 = '' OR cs.status = $2)
		ORDER BY (cs.status = 'pending') DESC, COALESCE(cs.reviewed_at, cs.created_at) DESC
		LIMIT $3`, sellerID, status, limit)
	if err != nil {
		return nil, fmt.Errorf("failed to list submissions: %w", err)
	}
	defer rows.Close()
	out := []AdminSubmission{}
	for rows.Next() {
		a, err := scanAdminSubmission(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, *a)
	}
	return out, rows.Err()
}

// ---------------------------------------------------------------- service

// ListSellerCollectionOptions returns collections that are published right now.
func (s *Service) ListSellerCollectionOptions(ctx context.Context) ([]SellerCollectionOption, error) {
	cols, err := s.repo.ListActiveCollectionsPublic(ctx, time.Now())
	if err != nil {
		return nil, err
	}
	out := make([]SellerCollectionOption, 0, len(cols))
	for _, c := range cols {
		out = append(out, SellerCollectionOption{ID: c.ID, Slug: c.Slug, Title: c.Title, Description: c.Description})
	}
	return out, nil
}

func (s *Service) ListSellerSubmissions(ctx context.Context, sellerID uuid.UUID) ([]SellerSubmission, error) {
	rows, err := s.repo.listSubmissions(ctx, &sellerID, "", 100)
	if err != nil {
		return nil, err
	}
	out := make([]SellerSubmission, len(rows))
	for i, r := range rows {
		out[i] = r.SellerSubmission
	}
	return out, nil
}

// CreateSubmission validates that the product is the seller's own, published and visible in the catalog,
// that the collection is published, that the product is not in it yet and that no active submission exists.
func (s *Service) CreateSubmission(ctx context.Context, sellerID uuid.UUID, req CreateSubmissionRequest) (*SellerSubmission, error) {
	comment, err := normalizeText(req.Comment)
	if err != nil {
		return nil, err
	}
	col, err := s.repo.GetCollectionByID(ctx, req.CollectionID)
	if err != nil {
		return nil, err
	}
	if !collectionAvailable(col, time.Now()) {
		return nil, ErrCollectionUnavailable
	}
	owned, eligible, err := s.repo.productEligibleForSeller(ctx, req.ProductID, &sellerID)
	if err != nil {
		return nil, err
	}
	if !owned {
		return nil, ErrProductNotFound // someone else's product looks like a missing one
	}
	if !eligible {
		return nil, ErrProductNotEligible
	}
	if in, err := s.repo.productInCollection(ctx, col.ID, req.ProductID); err != nil {
		return nil, err
	} else if in {
		return nil, ErrAlreadyInCollection
	}
	id := uuid.New()
	if err := s.repo.insertSubmission(ctx, id, col.ID, req.ProductID, sellerID, comment); err != nil {
		return nil, err
	}
	created, err := s.repo.getSubmission(ctx, id)
	if err != nil {
		return nil, err
	}
	return &created.SellerSubmission, nil
}

func (s *Service) ListAdminSubmissions(ctx context.Context, status string) ([]AdminSubmission, error) {
	switch status {
	case "", "all":
		status = ""
	case SubmissionPending, SubmissionAccepted, SubmissionRejected:
	default:
		return nil, ErrInvalidSubmissionStatus
	}
	return s.repo.listSubmissions(ctx, nil, status, 200)
}

// lockPendingSubmission locks the submission row; a concurrent reviewer waits and then sees it reviewed.
func lockPendingSubmission(ctx context.Context, tx pgx.Tx, id uuid.UUID) (collectionID, productID uuid.UUID, err error) {
	var status string
	err = tx.QueryRow(ctx, `SELECT status, collection_id, product_id FROM collection_submissions WHERE id = $1 FOR UPDATE`, id).
		Scan(&status, &collectionID, &productID)
	if errors.Is(err, pgx.ErrNoRows) {
		return uuid.Nil, uuid.Nil, ErrSubmissionNotFound
	}
	if err != nil {
		return uuid.Nil, uuid.Nil, err
	}
	if status != SubmissionPending {
		return uuid.Nil, uuid.Nil, ErrSubmissionReviewed
	}
	return collectionID, productID, nil
}

// AcceptSubmission re-checks the collection and the product, appends the product at the end of the collection
// using the existing item replacement logic and marks the submission accepted, all in one transaction.
func (s *Service) AcceptSubmission(ctx context.Context, id, reviewerID uuid.UUID) (*AdminSubmission, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx)
	repo := s.repo.WithTx(tx)

	collectionID, productID, err := lockPendingSubmission(ctx, tx, id)
	if err != nil {
		return nil, err
	}
	// Serializes with every other writer of this collection's items (incl. the admin item editor).
	if _, err := tx.Exec(ctx, `SELECT 1 FROM collections WHERE id = $1 FOR UPDATE`, collectionID); err != nil {
		return nil, err
	}
	col, err := repo.GetCollectionByID(ctx, collectionID)
	if err != nil {
		return nil, err
	}
	if !collectionAvailable(col, time.Now()) {
		return nil, ErrCollectionUnavailable
	}
	owned, eligible, err := repo.productEligibleForSeller(ctx, productID, nil)
	if err != nil {
		return nil, err
	}
	if !owned || !eligible {
		return nil, ErrProductNotEligible
	}
	items, err := repo.GetAdminCollectionItems(ctx, collectionID)
	if err != nil {
		return nil, err
	}
	ids := make([]uuid.UUID, 0, len(items)+1)
	for _, it := range items {
		if it.ProductID == productID {
			return nil, ErrAlreadyInCollection
		}
		ids = append(ids, it.ProductID)
	}
	ids = append(ids, productID)
	if err := repo.ReplaceCollectionItems(ctx, collectionID, ids); err != nil {
		return nil, err
	}
	if _, err := tx.Exec(ctx, `
		UPDATE collection_submissions
		SET status = 'accepted', reviewed_by = $2, reviewed_at = now(), updated_at = now()
		WHERE id = $1`, id, reviewerID); err != nil {
		return nil, err
	}
	res, err := repo.getSubmission(ctx, id)
	if err != nil {
		return nil, err
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}
	return res, nil
}

func (s *Service) RejectSubmission(ctx context.Context, id, reviewerID uuid.UUID, reason string) (*AdminSubmission, error) {
	text, err := normalizeText(reason)
	if err != nil {
		return nil, err
	}
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx)
	if _, _, err := lockPendingSubmission(ctx, tx, id); err != nil {
		return nil, err
	}
	if _, err := tx.Exec(ctx, `
		UPDATE collection_submissions
		SET status = 'rejected', rejection_reason = $3, reviewed_by = $2, reviewed_at = now(), updated_at = now()
		WHERE id = $1`, id, reviewerID, text); err != nil {
		return nil, err
	}
	res, err := s.repo.WithTx(tx).getSubmission(ctx, id)
	if err != nil {
		return nil, err
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}
	return res, nil
}

// ---------------------------------------------------------------- handlers

func (h *Handler) submissionError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, ErrCollectionNotFound):
		h.writeError(w, http.StatusNotFound, "collection_not_found", "Подборка не найдена")
	case errors.Is(err, ErrProductNotFound):
		h.writeError(w, http.StatusNotFound, "product_not_found", "Товар не найден среди ваших товаров")
	case errors.Is(err, ErrSubmissionNotFound):
		h.writeError(w, http.StatusNotFound, "not_found", "Заявка не найдена")
	case errors.Is(err, ErrCollectionUnavailable):
		h.writeError(w, http.StatusConflict, "collection_unavailable", "Подборка сейчас не опубликована")
	case errors.Is(err, ErrProductNotEligible):
		h.writeError(w, http.StatusConflict, "product_not_eligible", "Товар должен быть опубликован и доступен в каталоге")
	case errors.Is(err, ErrAlreadyInCollection):
		h.writeError(w, http.StatusConflict, "already_in_collection", "Товар уже входит в эту подборку")
	case errors.Is(err, ErrDuplicateSubmission):
		h.writeError(w, http.StatusConflict, "duplicate_submission", "Заявка на этот товар в эту подборку уже ожидает рассмотрения")
	case errors.Is(err, ErrSubmissionReviewed):
		h.writeError(w, http.StatusConflict, "already_reviewed", "Заявка уже рассмотрена")
	case errors.Is(err, ErrTextTooLong):
		h.writeError(w, http.StatusBadRequest, "text_too_long", "Текст не должен превышать 300 символов")
	case errors.Is(err, ErrInvalidSubmissionStatus):
		h.writeError(w, http.StatusBadRequest, "invalid_status", "Неизвестный статус заявки")
	default:
		h.writeError(w, http.StatusInternalServerError, "internal_error", "Не удалось выполнить операцию")
	}
}

func sellerFromRequest(h *Handler, w http.ResponseWriter, r *http.Request) (uuid.UUID, bool) {
	sellerID, err := auth.GetSellerID(r.Context())
	if err != nil || sellerID == uuid.Nil {
		h.writeError(w, http.StatusUnauthorized, "unauthorized", "Seller authorization required")
		return uuid.Nil, false
	}
	return sellerID, true
}

func (h *Handler) ListSellerCollectionOptions(w http.ResponseWriter, r *http.Request) {
	if _, ok := sellerFromRequest(h, w, r); !ok {
		return
	}
	items, err := h.service.ListSellerCollectionOptions(r.Context())
	if err != nil {
		h.submissionError(w, err)
		return
	}
	h.writeJSON(w, http.StatusOK, map[string]any{"items": items})
}

func (h *Handler) ListSellerSubmissions(w http.ResponseWriter, r *http.Request) {
	sellerID, ok := sellerFromRequest(h, w, r)
	if !ok {
		return
	}
	items, err := h.service.ListSellerSubmissions(r.Context(), sellerID)
	if err != nil {
		h.submissionError(w, err)
		return
	}
	h.writeJSON(w, http.StatusOK, map[string]any{"items": items})
}

func (h *Handler) CreateSellerSubmission(w http.ResponseWriter, r *http.Request) {
	sellerID, ok := sellerFromRequest(h, w, r)
	if !ok {
		return
	}
	var req CreateSubmissionRequest
	if err := decodeJSON(r, &req); err != nil || req.CollectionID == uuid.Nil || req.ProductID == uuid.Nil {
		h.writeError(w, http.StatusBadRequest, "invalid_request", "Укажите подборку и товар")
		return
	}
	created, err := h.service.CreateSubmission(r.Context(), sellerID, req)
	if err != nil {
		h.submissionError(w, err)
		return
	}
	h.writeJSON(w, http.StatusCreated, created)
}

func (h *Handler) ListAdminSubmissions(w http.ResponseWriter, r *http.Request) {
	items, err := h.service.ListAdminSubmissions(r.Context(), r.URL.Query().Get("status"))
	if err != nil {
		h.submissionError(w, err)
		return
	}
	h.writeJSON(w, http.StatusOK, map[string]any{"items": items})
}

func (h *Handler) AcceptSubmission(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		h.writeError(w, http.StatusNotFound, "not_found", "Заявка не найдена")
		return
	}
	res, err := h.service.AcceptSubmission(r.Context(), id, auth.GetUserID(r.Context()))
	if err != nil {
		h.submissionError(w, err)
		return
	}
	h.writeJSON(w, http.StatusOK, res)
}

func (h *Handler) RejectSubmission(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		h.writeError(w, http.StatusNotFound, "not_found", "Заявка не найдена")
		return
	}
	var req RejectSubmissionRequest
	if r.ContentLength != 0 {
		if err := decodeJSON(r, &req); err != nil {
			h.writeError(w, http.StatusBadRequest, "invalid_request", "Invalid JSON body")
			return
		}
	}
	res, err := h.service.RejectSubmission(r.Context(), id, auth.GetUserID(r.Context()), req.Reason)
	if err != nil {
		h.submissionError(w, err)
		return
	}
	h.writeJSON(w, http.StatusOK, res)
}

func decodeJSON(r *http.Request, v any) error {
	return json.NewDecoder(http.MaxBytesReader(nil, r.Body, 1<<16)).Decode(v)
}

func (h *Handler) writeJSON(w http.ResponseWriter, status int, data any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(data)
}
