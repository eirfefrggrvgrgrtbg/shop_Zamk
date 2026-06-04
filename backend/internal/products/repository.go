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

// ---------------------------------------------------------
// List Operations
// ---------------------------------------------------------
// For simplicity in Phase 4, we use basic lists without pagination arguments in SQL yet, 
// but we structure them to be easily extensible.

func (r *Repository) ListProductsBySeller(ctx context.Context, sellerID uuid.UUID) ([]Product, error) {
	query := `
		SELECT id, seller_id, category_id, brand_id, title, slug, description,
			status, gender, color, material, care_instructions,
			price_cents, old_price_cents, currency, main_image_url,
			created_at, updated_at, submitted_at, approved_at, published_at, rejected_at, moderation_comment
		FROM products
		WHERE seller_id = $1
		ORDER BY created_at DESC
	`
	return r.listProductsQuery(ctx, query, sellerID)
}

func (r *Repository) ListAllProducts(ctx context.Context) ([]Product, error) {
	query := `
		SELECT id, seller_id, category_id, brand_id, title, slug, description,
			status, gender, color, material, care_instructions,
			price_cents, old_price_cents, currency, main_image_url,
			created_at, updated_at, submitted_at, approved_at, published_at, rejected_at, moderation_comment
		FROM products
		ORDER BY created_at DESC
	`
	return r.listProductsQuery(ctx, query)
}

func (r *Repository) ListProductsForModeration(ctx context.Context) ([]Product, error) {
	query := `
		SELECT id, seller_id, category_id, brand_id, title, slug, description,
			status, gender, color, material, care_instructions,
			price_cents, old_price_cents, currency, main_image_url,
			created_at, updated_at, submitted_at, approved_at, published_at, rejected_at, moderation_comment
		FROM products
		WHERE status = 'pending_moderation'
		ORDER BY submitted_at ASC
	`
	return r.listProductsQuery(ctx, query)
}

func (r *Repository) ListPublishedProducts(ctx context.Context) ([]Product, error) {
	query := `
		SELECT id, seller_id, category_id, brand_id, title, slug, description,
			status, gender, color, material, care_instructions,
			price_cents, old_price_cents, currency, main_image_url,
			created_at, updated_at, submitted_at, approved_at, published_at, rejected_at, moderation_comment
		FROM products
		WHERE status = 'published'
		ORDER BY published_at DESC
	`
	return r.listProductsQuery(ctx, query)
}

func (r *Repository) GetPublishedProductBySlugOrID(ctx context.Context, idOrSlug string) (*Product, error) {
	query := `
		SELECT id, seller_id, category_id, brand_id, title, slug, description,
			status, gender, color, material, care_instructions,
			price_cents, old_price_cents, currency, main_image_url,
			created_at, updated_at, submitted_at, approved_at, published_at, rejected_at, moderation_comment
		FROM products
		WHERE (slug = $1 OR id::text = $1) AND status = 'published'
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
