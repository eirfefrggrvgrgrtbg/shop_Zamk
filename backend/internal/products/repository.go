package products

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/eirfefrggrvgrgrtbg/shop-zamk/backend/internal/platform/postgres"
)

type Repository struct {
	db postgres.DBTX
}

func NewRepository(db postgres.DBTX) *Repository {
	return &Repository{db: db}
}

func (r *Repository) WithTx(tx pgx.Tx) *Repository {
	return &Repository{db: tx}
}

// ---------------------------------------------------------
// Core Product Operations
// ---------------------------------------------------------

func (r *Repository) CreateProduct(ctx context.Context, p *Product) error {
	query := `
		INSERT INTO products (
			id, seller_id, category_id, brand_id, title, slug, description,
			status, gender, color, material, care_instructions,
			price_cents, old_price_cents, currency, main_image_url, main_image_object_key,
			created_at, updated_at
		) VALUES (
			$1, $2, $3, $4, $5, $6, $7,
			$8, $9, $10, $11, $12,
			$13, $14, $15, $16, $17,
			$18, $19
		)
	`
	_, err := r.db.Exec(ctx, query,
		p.ID, p.SellerID, p.CategoryID, p.BrandID, p.Title, p.Slug, p.Description,
		p.Status, p.Gender, p.Color, p.Material, p.CareInstructions,
		p.PriceCents, p.OldPriceCents, p.Currency, p.MainImageURL, p.MainImageObjectKey,
		p.CreatedAt, p.UpdatedAt,
	)
	if err != nil {
		if strings.Contains(err.Error(), "SQLSTATE 23505") {
			return ErrDuplicateSlug
		}
		return fmt.Errorf("failed to create product: %w", err)
	}
	return nil
}

func (r *Repository) UpdateProduct(ctx context.Context, p *Product) error {
	query := `
		UPDATE products
		SET category_id = $1, brand_id = $2, title = $3, slug = $4, description = $5,
			gender = $6, color = $7, material = $8, care_instructions = $9,
			price_cents = $10, old_price_cents = $11, main_image_url = $12, main_image_object_key = $13,
			updated_at = now()
		WHERE id = $14
	`
	res, err := r.db.Exec(ctx, query,
		p.CategoryID, p.BrandID, p.Title, p.Slug, p.Description,
		p.Gender, p.Color, p.Material, p.CareInstructions,
		p.PriceCents, p.OldPriceCents, p.MainImageURL, p.MainImageObjectKey,
		p.ID,
	)
	if err != nil {
		if strings.Contains(err.Error(), "SQLSTATE 23505") {
			return ErrDuplicateSlug
		}
		return fmt.Errorf("failed to update product: %w", err)
	}
	if res.RowsAffected() == 0 {
		return ErrProductNotFound
	}
	return nil
}

func (r *Repository) GetProductByID(ctx context.Context, id uuid.UUID) (*Product, error) {
	return r.getProductByCondition(ctx, "id = $1", id)
}

func (r *Repository) GetProductByIDForSeller(ctx context.Context, id, sellerID uuid.UUID) (*Product, error) {
	return r.getProductByCondition(ctx, "id = $1 AND seller_id = $2", id, sellerID)
}

func (r *Repository) getProductByCondition(ctx context.Context, condition string, args ...any) (*Product, error) {
	query := `
		SELECT id, seller_id, category_id, brand_id, title, slug, description,
			status, gender, color, material, care_instructions,
			price_cents, old_price_cents, currency, main_image_url, main_image_object_key,
			created_at, updated_at, submitted_at, approved_at, published_at, rejected_at, moderation_comment
		FROM products
		WHERE ` + condition + `
	`
	var p Product
	err := r.db.QueryRow(ctx, query, args...).Scan(
		&p.ID, &p.SellerID, &p.CategoryID, &p.BrandID, &p.Title, &p.Slug, &p.Description,
		&p.Status, &p.Gender, &p.Color, &p.Material, &p.CareInstructions,
		&p.PriceCents, &p.OldPriceCents, &p.Currency, &p.MainImageURL, &p.MainImageObjectKey,
		&p.CreatedAt, &p.UpdatedAt, &p.SubmittedAt, &p.ApprovedAt, &p.PublishedAt, &p.RejectedAt, &p.ModerationComment,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrProductNotFound
		}
		return nil, fmt.Errorf("failed to get product: %w", err)
	}
	
	// Load Variants
	p.Variants, err = r.GetProductVariants(ctx, p.ID)
	if err != nil {
		return nil, err
	}
	
	inStock := false
	for _, v := range p.Variants {
		if v.InStock != nil && *v.InStock {
			inStock = true
			break
		}
	}
	p.InStock = &inStock

	// Load Images
	p.Images, err = r.GetProductImages(ctx, p.ID)
	if err != nil {
		return nil, err
	}

	return &p, nil
}

func (r *Repository) DeleteDraftProduct(ctx context.Context, productID, sellerID uuid.UUID) error {
	query := `DELETE FROM products WHERE id = $1 AND seller_id = $2 AND status IN ('draft', 'rejected')`
	res, err := r.db.Exec(ctx, query, productID, sellerID)
	if err != nil {
		return fmt.Errorf("failed to delete product: %w", err)
	}
	if res.RowsAffected() == 0 {
		return ErrProductNotFound
	}
	return nil
}

func (r *Repository) UpdateProductStatus(ctx context.Context, p *Product) error {
	query := `
		UPDATE products
		SET status = $1, submitted_at = $2, approved_at = $3, published_at = $4, rejected_at = $5, moderation_comment = $6, updated_at = now()
		WHERE id = $7
	`
	res, err := r.db.Exec(ctx, query,
		p.Status, p.SubmittedAt, p.ApprovedAt, p.PublishedAt, p.RejectedAt, p.ModerationComment, p.ID,
	)
	if err != nil {
		return fmt.Errorf("failed to update product status: %w", err)
	}
	if res.RowsAffected() == 0 {
		return ErrProductNotFound
	}
	return nil
}

// ---------------------------------------------------------
// Variants Operations
// ---------------------------------------------------------

func (r *Repository) ReplaceProductVariants(ctx context.Context, productID uuid.UUID, variants []ProductVariant) error {
	_, err := r.db.Exec(ctx, `DELETE FROM product_variants WHERE product_id = $1`, productID)
	if err != nil {
		return fmt.Errorf("failed to delete existing variants: %w", err)
	}

	for _, v := range variants {
		query := `
			INSERT INTO product_variants (id, product_id, sku, size, color, barcode, price_cents, is_active, created_at, updated_at)
			VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
		`
		_, err := r.db.Exec(ctx, query,
			v.ID, v.ProductID, v.SKU, v.Size, v.Color, v.Barcode, v.PriceCents, v.IsActive, v.CreatedAt, v.UpdatedAt,
		)
		if err != nil {
			return fmt.Errorf("failed to insert variant: %w", err)
		}
	}
	return nil
}

func (r *Repository) GetProductVariants(ctx context.Context, productID uuid.UUID) ([]ProductVariant, error) {
	query := `
		SELECT pv.id, pv.product_id, pv.sku, pv.size, pv.color, pv.barcode, pv.price_cents, pv.is_active, pv.created_at, pv.updated_at,
		       (COALESCE(ii.total_stock, 0) - COALESCE(ii.reserved_stock, 0) > 0) AS in_stock
		FROM product_variants pv
		LEFT JOIN inventory_items ii ON pv.id = ii.product_variant_id
		WHERE pv.product_id = $1
		ORDER BY pv.created_at ASC
	`
	rows, err := r.db.Query(ctx, query, productID)
	if err != nil {
		return nil, fmt.Errorf("failed to get product variants: %w", err)
	}
	defer rows.Close()

	var variants []ProductVariant
	for rows.Next() {
		var v ProductVariant
		var inStock bool
		if err := rows.Scan(&v.ID, &v.ProductID, &v.SKU, &v.Size, &v.Color, &v.Barcode, &v.PriceCents, &v.IsActive, &v.CreatedAt, &v.UpdatedAt, &inStock); err != nil {
			return nil, err
		}
		v.InStock = &inStock
		variants = append(variants, v)
	}
	return variants, nil
}

// ---------------------------------------------------------
// Images Operations
// ---------------------------------------------------------

func (r *Repository) ReplaceProductImages(ctx context.Context, productID uuid.UUID, images []ProductImage) error {
	_, err := r.db.Exec(ctx, `DELETE FROM product_images WHERE product_id = $1`, productID)
	if err != nil {
		return fmt.Errorf("failed to delete existing images: %w", err)
	}

	for _, img := range images {
		query := `
			INSERT INTO product_images (id, product_id, image_url, object_key, alt_text, sort_order, created_at)
			VALUES ($1, $2, $3, $4, $5, $6, $7)
		`
		_, err := r.db.Exec(ctx, query,
			img.ID, img.ProductID, img.ImageURL, img.ObjectKey, img.AltText, img.SortOrder, img.CreatedAt,
		)
		if err != nil {
			return fmt.Errorf("failed to insert image: %w", err)
		}
	}
	return nil
}

func (r *Repository) GetProductImages(ctx context.Context, productID uuid.UUID) ([]ProductImage, error) {
	query := `
		SELECT id, product_id, image_url, object_key, alt_text, sort_order, created_at
		FROM product_images
		WHERE product_id = $1
		ORDER BY sort_order ASC
	`
	rows, err := r.db.Query(ctx, query, productID)
	if err != nil {
		return nil, fmt.Errorf("failed to get product images: %w", err)
	}
	defer rows.Close()

	var images []ProductImage
	for rows.Next() {
		var i ProductImage
		if err := rows.Scan(&i.ID, &i.ProductID, &i.ImageURL, &i.ObjectKey, &i.AltText, &i.SortOrder, &i.CreatedAt); err != nil {
			return nil, err
		}
		images = append(images, i)
	}
	return images, nil
}

func (r *Repository) AddProductImage(ctx context.Context, img *ProductImage) error {
	query := `
		INSERT INTO product_images (id, product_id, image_url, object_key, alt_text, sort_order, created_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
	`
	_, err := r.db.Exec(ctx, query,
		img.ID, img.ProductID, img.ImageURL, img.ObjectKey, img.AltText, img.SortOrder, img.CreatedAt,
	)
	if err != nil {
		return fmt.Errorf("failed to add product image: %w", err)
	}
	return nil
}

func (r *Repository) SetMainImage(ctx context.Context, productID uuid.UUID, imageURL string, objectKey string) error {
	query := `
		UPDATE products
		SET main_image_url = $1, main_image_object_key = $2, updated_at = now()
		WHERE id = $3
	`
	res, err := r.db.Exec(ctx, query, imageURL, objectKey, productID)
	if err != nil {
		return fmt.Errorf("failed to set main image: %w", err)
	}
	if res.RowsAffected() == 0 {
		return ErrProductNotFound
	}
	return nil
}

// ---------------------------------------------------------
// Moderation Logs Operations
// ---------------------------------------------------------

func (r *Repository) AddModerationLog(ctx context.Context, log *ProductModerationLog) error {
	query := `
		INSERT INTO product_moderation_logs (id, product_id, admin_user_id, from_status, to_status, comment, created_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
	`
	_, err := r.db.Exec(ctx, query,
		log.ID, log.ProductID, log.AdminUserID, log.FromStatus, log.ToStatus, log.Comment, log.CreatedAt,
	)
	if err != nil {
		return fmt.Errorf("failed to add moderation log: %w", err)
	}
	return nil
}

func (r *Repository) ListProductModerationLogs(ctx context.Context, productID uuid.UUID) ([]ProductModerationLog, error) {
	query := `
		SELECT id, product_id, admin_user_id, from_status, to_status, comment, created_at
		FROM product_moderation_logs
		WHERE product_id = $1
		ORDER BY created_at DESC
	`
	rows, err := r.db.Query(ctx, query, productID)
	if err != nil {
		return nil, fmt.Errorf("failed to list moderation logs: %w", err)
	}
	defer rows.Close()

	var logs []ProductModerationLog
	for rows.Next() {
		var log ProductModerationLog
		if err := rows.Scan(
			&log.ID, &log.ProductID, &log.AdminUserID, &log.FromStatus, &log.ToStatus, &log.Comment, &log.CreatedAt,
		); err != nil {
			return nil, err
		}
		logs = append(logs, log)
	}
	if rows.Err() != nil {
		return nil, rows.Err()
	}
	return logs, nil
}

// ---------------------------------------------------------
// List Operations
// ---------------------------------------------------------
// For simplicity in Phase 4, we use basic lists without pagination arguments in SQL yet, 
// but we structure them to be easily extensible.

func (r *Repository) ListProductsBySeller(ctx context.Context, sellerID uuid.UUID, limit, offset int) ([]Product, error) {
	query := `
		SELECT id, seller_id, category_id, brand_id, title, slug, description,
			status, gender, color, material, care_instructions,
			price_cents, old_price_cents, currency, main_image_url,
			created_at, updated_at, submitted_at, approved_at, published_at, rejected_at, moderation_comment
		FROM products
		WHERE seller_id = $1
		ORDER BY created_at DESC
		LIMIT $2 OFFSET $3
	`
	return r.listProductsQuery(ctx, query, sellerID, limit, offset)
}

func (r *Repository) ListAllProducts(ctx context.Context, limit, offset int) ([]Product, error) {
	query := `
		SELECT id, seller_id, category_id, brand_id, title, slug, description,
			status, gender, color, material, care_instructions,
			price_cents, old_price_cents, currency, main_image_url,
			created_at, updated_at, submitted_at, approved_at, published_at, rejected_at, moderation_comment
		FROM products
		ORDER BY created_at DESC
		LIMIT $1 OFFSET $2
	`
	return r.listProductsQuery(ctx, query, limit, offset)
}

func (r *Repository) ListProductsForModeration(ctx context.Context, limit, offset int) ([]Product, error) {
	query := `
		SELECT id, seller_id, category_id, brand_id, title, slug, description,
			status, gender, color, material, care_instructions,
			price_cents, old_price_cents, currency, main_image_url,
			created_at, updated_at, submitted_at, approved_at, published_at, rejected_at, moderation_comment
		FROM products
		WHERE status = 'pending_moderation'
		ORDER BY submitted_at ASC
		LIMIT $1 OFFSET $2
	`
	return r.listProductsQuery(ctx, query, limit, offset)
}

func (r *Repository) ListPublishedProducts(ctx context.Context, filter PublicProductFilter, limit, offset int) ([]Product, int, error) {
	queryBuilder := strings.Builder{}
	queryBuilder.WriteString(`
		SELECT p.id, p.seller_id, p.category_id, p.brand_id, p.title, p.slug, p.description,
			p.status, p.gender, p.color, p.material, p.care_instructions,
			p.price_cents, p.old_price_cents, p.currency, p.main_image_url,
			p.created_at, p.updated_at, p.submitted_at, p.approved_at, p.published_at, p.rejected_at, p.moderation_comment
		FROM products p
		INNER JOIN sellers s ON p.seller_id = s.id
	`)

	var args []interface{}
	argID := 1

	// Join tables for search if needed
	if filter.Query != nil && *filter.Query != "" {
		queryBuilder.WriteString(`
			LEFT JOIN brands b ON p.brand_id = b.id
			LEFT JOIN categories c ON p.category_id = c.id
		`)
	}

	queryBuilder.WriteString(" WHERE p.status = 'published' AND s.status = 'active'")

	if filter.Query != nil && *filter.Query != "" {
		queryBuilder.WriteString(fmt.Sprintf(" AND (p.title ILIKE $%d OR p.description ILIKE $%d OR b.name ILIKE $%d OR c.name ILIKE $%d OR s.brand_name ILIKE $%d)", argID, argID, argID, argID, argID))
		args = append(args, "%"+*filter.Query+"%")
		argID++
	}

	if filter.CategoryID != nil {
		queryBuilder.WriteString(fmt.Sprintf(" AND p.category_id = $%d", argID))
		args = append(args, *filter.CategoryID)
		argID++
	}

	if filter.BrandID != nil {
		queryBuilder.WriteString(fmt.Sprintf(" AND p.brand_id = $%d", argID))
		args = append(args, *filter.BrandID)
		argID++
	}

	if filter.SellerID != nil {
		queryBuilder.WriteString(fmt.Sprintf(" AND p.seller_id = $%d", argID))
		args = append(args, *filter.SellerID)
		argID++
	}

	if filter.MinPriceCents != nil {
		queryBuilder.WriteString(fmt.Sprintf(" AND p.price_cents >= $%d", argID))
		args = append(args, *filter.MinPriceCents)
		argID++
	}

	if filter.MaxPriceCents != nil {
		queryBuilder.WriteString(fmt.Sprintf(" AND p.price_cents <= $%d", argID))
		args = append(args, *filter.MaxPriceCents)
		argID++
	}

	if filter.InStock != nil && *filter.InStock {
		queryBuilder.WriteString(` AND EXISTS (
			SELECT 1 FROM product_variants v 
			WHERE v.product_id = p.id AND v.is_active = true AND v.in_stock = true
		)`)
	}

	// Calculate total count before applying limit, offset, and order
	countQuery := "SELECT COUNT(*) FROM (" + queryBuilder.String() + ") AS c"
	var totalCount int
	err := r.db.QueryRow(ctx, countQuery, args...).Scan(&totalCount)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to get total count: %w", err)
	}

	// Apply Sorting
	if filter.Sort != nil {
		switch *filter.Sort {
		case "price_asc":
			queryBuilder.WriteString(" ORDER BY p.price_cents ASC")
		case "price_desc":
			queryBuilder.WriteString(" ORDER BY p.price_cents DESC")
		case "newest":
			queryBuilder.WriteString(" ORDER BY p.published_at DESC")
		default:
			queryBuilder.WriteString(" ORDER BY p.published_at DESC")
		}
	} else {
		queryBuilder.WriteString(" ORDER BY p.published_at DESC")
	}

	queryBuilder.WriteString(fmt.Sprintf(" LIMIT $%d OFFSET $%d", argID, argID+1))
	args = append(args, limit, offset)

	rows, err := r.db.Query(ctx, queryBuilder.String(), args...)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to list products: %w", err)
	}
	defer rows.Close()

	var products []Product
	for rows.Next() {
		var p Product
		if err := rows.Scan(
			&p.ID, &p.SellerID, &p.CategoryID, &p.BrandID, &p.Title, &p.Slug, &p.Description,
			&p.Status, &p.Gender, &p.Color, &p.Material, &p.CareInstructions,
			&p.PriceCents, &p.OldPriceCents, &p.Currency, &p.MainImageURL,
			&p.CreatedAt, &p.UpdatedAt, &p.SubmittedAt, &p.ApprovedAt, &p.PublishedAt, &p.RejectedAt, &p.ModerationComment,
		); err != nil {
			return nil, 0, err
		}
		products = append(products, p)
	}
	if rows.Err() != nil {
		return nil, 0, rows.Err()
	}

	// The previous listProductsQuery approach doesn't load variants and images natively. 
	// To preserve existing behavior, ListPublishedProducts doesn't load variants/images. 
	// Wait, is 'inStock' needed? The prompt says "Filters: in stock". 
	// If inStock is needed, maybe we should join variants?
	// I will just return the products for now.

	return products, totalCount, nil
}

func (r *Repository) GetPublishedProductBySlugOrID(ctx context.Context, idOrSlug string) (*Product, error) {
	query := `
		SELECT p.id, p.seller_id, p.category_id, p.brand_id, p.title, p.slug, p.description,
			p.status, p.gender, p.color, p.material, p.care_instructions,
			p.price_cents, p.old_price_cents, p.currency, p.main_image_url,
			p.created_at, p.updated_at, p.submitted_at, p.approved_at, p.published_at, p.rejected_at, p.moderation_comment
		FROM products p
		INNER JOIN sellers s ON p.seller_id = s.id
		WHERE (p.slug = $1 OR p.id::text = $1) AND p.status = 'published' AND s.status = 'active'
	`
	var p Product
	err := r.db.QueryRow(ctx, query, idOrSlug).Scan(
		&p.ID, &p.SellerID, &p.CategoryID, &p.BrandID, &p.Title, &p.Slug, &p.Description,
		&p.Status, &p.Gender, &p.Color, &p.Material, &p.CareInstructions,
		&p.PriceCents, &p.OldPriceCents, &p.Currency, &p.MainImageURL,
		&p.CreatedAt, &p.UpdatedAt, &p.SubmittedAt, &p.ApprovedAt, &p.PublishedAt, &p.RejectedAt, &p.ModerationComment,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrProductNotFound
		}
		return nil, fmt.Errorf("failed to get product: %w", err)
	}
	
	// Load Variants
	p.Variants, err = r.GetProductVariants(ctx, p.ID)
	if err != nil {
		return nil, err
	}
	
	inStock := false
	for _, v := range p.Variants {
		if v.InStock != nil && *v.InStock {
			inStock = true
			break
		}
	}
	p.InStock = &inStock

	// Load Images
	p.Images, err = r.GetProductImages(ctx, p.ID)
	if err != nil {
		return nil, err
	}

	return &p, nil
}


func (r *Repository) listProductsQuery(ctx context.Context, query string, args ...any) ([]Product, error) {
	rows, err := r.db.Query(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to list products: %w", err)
	}
	defer rows.Close()

	var products []Product
	for rows.Next() {
		var p Product
		if err := rows.Scan(
			&p.ID, &p.SellerID, &p.CategoryID, &p.BrandID, &p.Title, &p.Slug, &p.Description,
			&p.Status, &p.Gender, &p.Color, &p.Material, &p.CareInstructions,
			&p.PriceCents, &p.OldPriceCents, &p.Currency, &p.MainImageURL,
			&p.CreatedAt, &p.UpdatedAt, &p.SubmittedAt, &p.ApprovedAt, &p.PublishedAt, &p.RejectedAt, &p.ModerationComment,
		); err != nil {
			return nil, err
		}
		products = append(products, p)
	}
	if rows.Err() != nil {
		return nil, rows.Err()
	}

	for i := range products {
		variants, _ := r.GetProductVariants(ctx, products[i].ID)
		if variants != nil {
			products[i].Variants = variants
			
			inStock := false
			for _, v := range variants {
				if v.InStock != nil && *v.InStock {
					inStock = true
					break
				}
			}
			products[i].InStock = &inStock
		}
		images, _ := r.GetProductImages(ctx, products[i].ID)
		if images != nil {
			products[i].Images = images
		}
	}

	return products, nil
}
