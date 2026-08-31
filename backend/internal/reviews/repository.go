package reviews

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/eirfefrggrvgrgrtbg/shop-zamk/backend/internal/platform/postgres"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
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
	if errors.As(err, &pgErr) && pgErr.Code == "23505" && pgErr.ConstraintName == "product_reviews_order_item_id_key" {
		return ErrReviewAlreadyExists
	}
	return err
}

func (r *Repository) GetReviewByID(ctx context.Context, tx postgres.DBTX, id uuid.UUID) (*ProductReview, error) {
	query := `
		SELECT r.id, r.product_id, r.product_variant_id, r.order_id, r.order_item_id, r.user_id, r.seller_id,
			   r.rating, r.title, r.comment, r.status, r.created_at, r.updated_at, r.published_at, r.rejected_at, r.moderation_comment,
			   oi.title as order_item_title
		FROM product_reviews r
		LEFT JOIN order_items oi ON oi.id = r.order_item_id
		WHERE r.id = $1
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
		&review.ProductTitle,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrReviewNotFound
		}
		return nil, err
	}
	return &review, nil
}

func (r *Repository) UpdateReviewStatus(ctx context.Context, tx postgres.DBTX, id uuid.UUID, status string, publishedAt, rejectedAt *time.Time, modText *string) error {
	query := `
		UPDATE product_reviews
		SET status = $2, published_at = COALESCE($3, published_at), rejected_at = COALESCE($4, rejected_at), moderation_comment = COALESCE($5, moderation_comment), updated_at = now()
		WHERE id = $1
	`
	dbExecutor := tx
	if dbExecutor == nil {
		dbExecutor = r.db.Pool
	}

	cmd, err := dbExecutor.Exec(ctx, query, id, status, publishedAt, rejectedAt, modText)
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
	query := `
		SELECT r.id, r.product_id, r.product_variant_id, r.order_id, r.order_item_id, r.user_id, r.seller_id,
			   r.rating, r.title, r.comment, r.status, r.created_at, r.updated_at, r.published_at, r.rejected_at, r.moderation_comment,
			   oi.title as order_item_title
		FROM product_reviews r
		LEFT JOIN order_items oi ON oi.id = r.order_item_id
		WHERE 1=1
	`
	var args []interface{}
	argIdx := 1

	if val, ok := filters["product_id"]; ok {
		query += fmt.Sprintf(" AND r.product_id = $%d", argIdx)
		args = append(args, val)
		argIdx++
	}
	if val, ok := filters["seller_id"]; ok {
		query += fmt.Sprintf(" AND r.seller_id = $%d", argIdx)
		args = append(args, val)
		argIdx++
	}
	if val, ok := filters["user_id"]; ok {
		query += fmt.Sprintf(" AND r.user_id = $%d", argIdx)
		args = append(args, val)
		argIdx++
	}
	if val, ok := filters["status"]; ok {
		query += fmt.Sprintf(" AND r.status = $%d", argIdx)
		args = append(args, val)
		argIdx++
	}

	query += fmt.Sprintf(" ORDER BY r.created_at DESC, r.id DESC LIMIT $%d OFFSET $%d", argIdx, argIdx+1)
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
			&rev.ProductTitle,
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

func (r *Repository) RecalculateProductRating(ctx context.Context, tx postgres.DBTX, productID uuid.UUID) error {
	query := `
		WITH stats AS (
			SELECT COALESCE(AVG(rating), 0) as avg_rating, COUNT(*) as cnt
			FROM product_reviews
			WHERE product_id = $1 AND status = 'published'
		)
		UPDATE products
		SET average_rating = ROUND(stats.avg_rating::numeric, 1),
		    reviews_count = stats.cnt
		FROM stats
		WHERE products.id = $1
	`
	dbExecutor := tx
	if dbExecutor == nil {
		dbExecutor = r.db.Pool
	}
	_, err := dbExecutor.Exec(ctx, query, productID)
	return err
}

func (r *Repository) RecalculateSellerRating(ctx context.Context, tx postgres.DBTX, sellerID uuid.UUID) error {
	query := `
		WITH stats AS (
			SELECT COALESCE(AVG(rating), 0) as avg_rating, COUNT(*) as cnt
			FROM product_reviews
			WHERE seller_id = $1 AND status = 'published'
		)
		UPDATE sellers
		SET average_rating = ROUND(stats.avg_rating::numeric, 1),
		    reviews_count = stats.cnt
		FROM stats
		WHERE sellers.id = $1
	`
	dbExecutor := tx
	if dbExecutor == nil {
		dbExecutor = r.db.Pool
	}
	_, err := dbExecutor.Exec(ctx, query, sellerID)
	return err
}

func (r *Repository) ResolvePublishedProductID(ctx context.Context, idOrSlug string) (uuid.UUID, error) {
	query := `
		SELECT p.id
		FROM products p
		INNER JOIN sellers s ON p.seller_id = s.id
		WHERE (p.slug = $1 OR p.id::text = $1) AND p.status = 'published' AND s.status = 'active'
		LIMIT 1
	`
	var id uuid.UUID
	err := r.db.Pool.QueryRow(ctx, query, idOrSlug).Scan(&id)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return uuid.Nil, ErrProductNotFound
		}
		return uuid.Nil, err
	}
	return id, nil
}

func (r *Repository) ListPublicReviews(ctx context.Context, productID uuid.UUID, limit, offset int) ([]PublicReviewRow, error) {
	query := `
		SELECT r.id, r.product_id, r.product_variant_id, r.order_id, r.order_item_id, r.user_id, r.seller_id,
			   r.rating, r.title, r.comment, r.status, r.created_at, r.updated_at, r.published_at, r.rejected_at, r.moderation_comment,
			   COALESCE(u.first_name, '') as reviewer_first_name, COALESCE(u.last_name, '') as reviewer_last_name,
			   oi.title as order_item_title, oi.variant_size as order_item_size, oi.variant_color as order_item_color
		FROM product_reviews r
		JOIN users u ON u.id = r.user_id
		JOIN order_items oi ON oi.id = r.order_item_id
		WHERE r.product_id = $1 AND r.status = 'published'
		ORDER BY r.created_at DESC, r.id DESC
		LIMIT $2 OFFSET $3
	`
	rows, err := r.db.Pool.Query(ctx, query, productID, limit, offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var results []PublicReviewRow
	for rows.Next() {
		var rev PublicReviewRow
		if err := rows.Scan(
			&rev.ID, &rev.ProductID, &rev.ProductVariantID, &rev.OrderID, &rev.OrderItemID,
			&rev.UserID, &rev.SellerID, &rev.Rating, &rev.Title, &rev.Comment,
			&rev.Status, &rev.CreatedAt, &rev.UpdatedAt, &rev.PublishedAt, &rev.RejectedAt, &rev.ModerationComment,
			&rev.ReviewerFirstName, &rev.ReviewerLastName,
			&rev.OrderItemTitle, &rev.OrderItemSize, &rev.OrderItemColor,
		); err != nil {
			return nil, err
		}
		results = append(results, rev)
	}
	return results, nil
}
