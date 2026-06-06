package reviews

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/eirfefrggrvgrgrtbg/shop-zamk/backend/internal/platform/postgres"
	"github.com/google/uuid"
)

type Repository struct {
	db *postgres.Client
}

func NewRepository(db *postgres.Client) *Repository {
	return &Repository{db: db}
}

func (r *Repository) CreateReview(ctx context.Context, tx postgres.DBTX, review *ProductReview) error {
	query := `
		INSERT INTO product_reviews (
			id, product_id, product_variant_id, order_id, order_item_id, user_id, seller_id,
			rating, title, comment, status, created_at, updated_at
		) VALUES (
			$1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13
		)
	`
	dbExecutor := tx
	if dbExecutor == nil {
		dbExecutor = r.db.Pool
	}

	_, err := dbExecutor.Exec(ctx, query,
		review.ID, review.ProductID, review.ProductVariantID, review.OrderID, review.OrderItemID,
		review.UserID, review.SellerID, review.Rating, review.Title, review.Comment,
		review.Status, review.CreatedAt, review.UpdatedAt,
	)

	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) && pgErr.Code == "23505" { // unique violation
		return ErrDuplicateReview
	}
	return err
}

func (r *Repository) GetReviewByID(ctx context.Context, tx postgres.DBTX, id uuid.UUID) (*ProductReview, error) {
	query := `
		SELECT id, product_id, product_variant_id, order_id, order_item_id, user_id, seller_id,
			   rating, title, comment, status, created_at, updated_at, published_at, rejected_at, moderation_comment
		FROM product_reviews
		WHERE id = $1
	`
	dbExecutor := tx
	if dbExecutor == nil {
		dbExecutor = r.db.Pool
	}

	var review ProductReview
	err := dbExecutor.QueryRow(ctx, query, id).Scan(
		&review.ID, &review.ProductID, &review.ProductVariantID, &review.OrderID, &review.OrderItemID,
		&review.UserID, &review.SellerID, &review.Rating, &review.Title, &review.Comment,
		&review.Status, &review.CreatedAt, &review.UpdatedAt, &review.PublishedAt, &review.RejectedAt, &review.ModerationComment,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrReviewNotFound
		}
		return nil, err
	}
	return &review, nil
}

func (r *Repository) UpdateReviewStatus(ctx context.Context, tx postgres.DBTX, id uuid.UUID, status string, publishedAt, rejectedAt *time.Time, modComment *string) error {
	query := `
		UPDATE product_reviews
		SET status = $2, published_at = COALESCE($3, published_at), rejected_at = COALESCE($4, rejected_at), moderation_comment = COALESCE($5, moderation_comment), updated_at = now()
		WHERE id = $1
	`
	dbExecutor := tx
	if dbExecutor == nil {
		dbExecutor = r.db.Pool
	}

	cmd, err := dbExecutor.Exec(ctx, query, id, status, publishedAt, rejectedAt, modComment)
	if err != nil {
		return err
	}
	if cmd.RowsAffected() == 0 {
		return ErrReviewNotFound
	}
	return nil
}

func (r *Repository) LogModeration(ctx context.Context, tx postgres.DBTX, log *ProductReviewModerationLog) error {
	query := `
		INSERT INTO product_review_moderation_logs (
			id, review_id, admin_user_id, from_status, to_status, comment, created_at
		) VALUES (
			$1, $2, $3, $4, $5, $6, $7
		)
	`
	dbExecutor := tx
	if dbExecutor == nil {
		dbExecutor = r.db.Pool
	}

	_, err := dbExecutor.Exec(ctx, query,
		log.ID, log.ReviewID, log.AdminUserID, log.FromStatus, log.ToStatus, log.Comment, log.CreatedAt,
	)
	return err
}

func (r *Repository) ListReviews(ctx context.Context, filters map[string]interface{}, limit, offset int) ([]ProductReview, error) {
	// A simple query builder for demonstration
	query := `
		SELECT id, product_id, product_variant_id, order_id, order_item_id, user_id, seller_id,
			   rating, title, comment, status, created_at, updated_at, published_at, rejected_at, moderation_comment
		FROM product_reviews
		WHERE 1=1
	`
	var args []interface{}
	argIdx := 1

	if val, ok := filters["product_id"]; ok {
		query += fmt.Sprintf(" AND product_id = $%d", argIdx)
		args = append(args, val)
		argIdx++
	}
	if val, ok := filters["seller_id"]; ok {
		query += fmt.Sprintf(" AND seller_id = $%d", argIdx)
		args = append(args, val)
		argIdx++
	}
	if val, ok := filters["user_id"]; ok {
		query += fmt.Sprintf(" AND user_id = $%d", argIdx)
		args = append(args, val)
		argIdx++
	}
	if val, ok := filters["status"]; ok {
		query += fmt.Sprintf(" AND status = $%d", argIdx)
		args = append(args, val)
		argIdx++
	}
	
	query += fmt.Sprintf(" ORDER BY created_at DESC LIMIT $%d OFFSET $%d", argIdx, argIdx+1)
	args = append(args, limit, offset)

	rows, err := r.db.Pool.Query(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var reviews []ProductReview
	for rows.Next() {
		var rev ProductReview
		if err := rows.Scan(
			&rev.ID, &rev.ProductID, &rev.ProductVariantID, &rev.OrderID, &rev.OrderItemID,
			&rev.UserID, &rev.SellerID, &rev.Rating, &rev.Title, &rev.Comment,
			&rev.Status, &rev.CreatedAt, &rev.UpdatedAt, &rev.PublishedAt, &rev.RejectedAt, &rev.ModerationComment,
		); err != nil {
			return nil, err
		}
		reviews = append(reviews, rev)
	}
	return reviews, rows.Err()
}

func (r *Repository) GetRatingSummary(ctx context.Context, productID uuid.UUID) (*RatingSummaryResponse, error) {
	query := `
		SELECT COALESCE(AVG(rating), 0), COUNT(*)
		FROM product_reviews
		WHERE product_id = $1 AND status = 'published'
	`
	var summary RatingSummaryResponse
	err := r.db.Pool.QueryRow(ctx, query, productID).Scan(&summary.Average, &summary.Count)
	if err != nil {
		return nil, err
	}
	return &summary, nil
}
