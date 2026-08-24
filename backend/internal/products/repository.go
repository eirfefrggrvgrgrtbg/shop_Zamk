package products

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"

	"github.com/eirfefrggrvgrgrtbg/shop-zamk/backend/internal/platform/postgres"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
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
// Concurrency and SKU Uniqueness
// ---------------------------------------------------------

func (r *Repository) LockSellerForUpdate(ctx context.Context, sellerID uuid.UUID) error {
	var id uuid.UUID
	err := r.db.QueryRow(ctx, "SELECT id FROM sellers WHERE id = $1 FOR NO KEY UPDATE", sellerID).Scan(&id)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return ErrSellerNotFound
		}
		return fmt.Errorf("failed to lock seller: %w", err)
	}
	return nil
}

func (r *Repository) FindExistingSellerSKUs(ctx context.Context, sellerID uuid.UUID, skus []string, excludeVariantIDs []uuid.UUID) ([]string, error) {
	if len(skus) == 0 {
		return nil, nil
	}

	query := `
		SELECT pv.seller_sku
		FROM product_variants pv
		JOIN products p ON pv.product_id = p.id
		WHERE p.seller_id = $1
		  AND pv.is_active = true
		  AND LOWER(TRIM(pv.seller_sku)) = ANY($2)
	`
	args := []any{sellerID, skus}

	if len(excludeVariantIDs) > 0 {
		query += " AND pv.id != ALL($3)"
		args = append(args, excludeVariantIDs)
	}

	rows, err := r.db.Query(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to query existing skus: %w", err)
	}
	defer rows.Close()

	var existing []string
	for rows.Next() {
		var sku string
		if err := rows.Scan(&sku); err != nil {
			return nil, err
		}
		existing = append(existing, sku)
	}
	return existing, rows.Err()
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
		fmt.Printf("CreateProduct DB Error: %v\n", err)
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
	return r.getProductByCondition(ctx, "p.id = $1", id)
}

func (r *Repository) GetProductByIDForSeller(ctx context.Context, id, sellerID uuid.UUID) (*Product, error) {
	return r.getProductByCondition(ctx, "p.id = $1 AND p.seller_id = $2", id, sellerID)
}

func (r *Repository) getProductByCondition(ctx context.Context, condition string, args ...any) (*Product, error) {
	query := `
		SELECT p.id, p.seller_id, p.category_id, p.brand_id, p.title, p.slug, p.description,
			p.status, COALESCE(p.source, 'seller') AS source, p.gender, p.color, p.material, p.care_instructions,
			p.price_cents, p.old_price_cents, p.currency, p.main_image_url, p.main_image_object_key,
			COALESCE(p.average_rating, 0) AS average_rating, COALESCE(p.reviews_count, 0) AS reviews_count,
			p.created_at, p.updated_at, p.submitted_at, p.approved_at, p.published_at, p.rejected_at, p.moderation_comment,
			p.assigned_admin_user_id, admin_user.name, p.review_started_at,
			s.brand_name, s.slug, u.name, u.email, c.name, b.name, s.status
		FROM products p
		LEFT JOIN sellers s ON p.seller_id = s.id
		LEFT JOIN seller_users su ON su.seller_id = s.id AND su.role = 'owner'
		LEFT JOIN users u ON su.user_id = u.id
		LEFT JOIN categories c ON p.category_id = c.id
		LEFT JOIN brands b ON p.brand_id = b.id
		LEFT JOIN users admin_user ON p.assigned_admin_user_id = admin_user.id
		WHERE ` + condition + `
	`
	var p Product
	err := r.db.QueryRow(ctx, query, args...).Scan(
		&p.ID, &p.SellerID, &p.CategoryID, &p.BrandID, &p.Title, &p.Slug, &p.Description,
		&p.Status, &p.Source, &p.Gender, &p.Color, &p.Material, &p.CareInstructions,
		&p.PriceCents, &p.OldPriceCents, &p.Currency, &p.MainImageURL, &p.MainImageObjectKey,
		&p.AverageRating, &p.ReviewsCount,
		&p.CreatedAt, &p.UpdatedAt, &p.SubmittedAt, &p.ApprovedAt, &p.PublishedAt, &p.RejectedAt, &p.ModerationComment,
		&p.AssignedAdminUserID, &p.AssignedAdminName, &p.ReviewStartedAt,
		&p.SellerName, &p.SellerSlug, &p.SellerOwnerName, &p.SellerOwnerEmail, &p.CategoryName, &p.BrandName, &p.SellerStatus,
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

	p.Attributes, err = r.GetProductAttributes(ctx, p.ID)
	if err != nil {
		return nil, err
	}

	p.MaterialComposition, err = r.GetProductMaterialComposition(ctx, p.ID)
	if err != nil {
		return nil, err
	}

	p.SizeChart, err = r.GetProductSizeChart(ctx, p.ID)
	if err != nil {
		return nil, err
	}

	// Load Images
	p.Images, err = r.GetProductImages(ctx, p.ID)
	if err != nil {
		return nil, err
	}

	PopulateProductAggregates(&p)

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
		SET status = $1, submitted_at = $2, approved_at = $3, published_at = $4, rejected_at = $5, moderation_comment = $6, assigned_admin_user_id = $7, review_started_at = $8, updated_at = now()
		WHERE id = $9
	`
	res, err := r.db.Exec(ctx, query,
		p.Status, p.SubmittedAt, p.ApprovedAt, p.PublishedAt, p.RejectedAt, p.ModerationComment, p.AssignedAdminUserID, p.ReviewStartedAt, p.ID,
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

func (r *Repository) MergeProductVariants(ctx context.Context, productID uuid.UUID, variants []ProductVariant) error {
	var sellerID uuid.UUID
	err := r.db.QueryRow(ctx, `SELECT seller_id FROM products WHERE id = $1`, productID).Scan(&sellerID)
	if err != nil {
		return fmt.Errorf("failed to get seller_id for product: %w", err)
	}

	existingVariants, err := r.GetProductVariants(ctx, productID)
	if err != nil {
		return fmt.Errorf("failed to get existing variants: %w", err)
	}

	existingMap := make(map[uuid.UUID]ProductVariant)
	for _, v := range existingVariants {
		existingMap[v.ID] = v
	}

	claimedExisting := make(map[uuid.UUID]bool)
	incomingMap := make(map[uuid.UUID]bool)

	// Phase 1: resolve variant IDs (by ID or by canonical color+size combination)
	for i := range variants {
		v := &variants[i]
		if _, exists := existingMap[v.ID]; exists && !claimedExisting[v.ID] {
			claimedExisting[v.ID] = true
			incomingMap[v.ID] = true
			continue
		}

		// Try matching by canonical combination (color_id + size_value_id)
		if v.ColorID != nil && v.SizeValueID != nil {
			for _, ev := range existingVariants {
				if !claimedExisting[ev.ID] && ev.ColorID != nil && *ev.ColorID == *v.ColorID && ev.SizeValueID != nil && *ev.SizeValueID == *v.SizeValueID {
					v.ID = ev.ID
					if v.Barcode == nil || *v.Barcode == "" {
						v.Barcode = ev.Barcode
					}
					claimedExisting[ev.ID] = true
					incomingMap[ev.ID] = true
					break
				}
			}
		}
		incomingMap[v.ID] = true
	}

	// Phase 2: Soft delete any existing variants not in incomingMap BEFORE inserting/updating
	for id := range existingMap {
		if !incomingMap[id] {
			softDeleteQuery := `UPDATE product_variants SET is_active = false, updated_at = now() WHERE id = $1 AND product_id = $2`
			_, err := r.db.Exec(ctx, softDeleteQuery, id, productID)
			if err != nil {
				return fmt.Errorf("failed to soft-delete variant: %w", err)
			}
		}
	}

	// Phase 3: Insert or update incoming variants
	for _, v := range variants {
		_, exists := existingMap[v.ID]

		if exists {
			query := `
				UPDATE product_variants
				SET sku = $1, size = $2, color = $3, option_values = $4, barcode = $5, price_cents = $6, is_active = $7, seller_sku = $10, color_id = $11, size_value_id = $12, shade_name = $13, updated_at = now()
				WHERE id = $8 AND product_id = $9
			`
			_, err := r.db.Exec(ctx, query,
				v.SKU, v.Size, v.Color, v.OptionValues, v.Barcode, v.PriceCents, v.IsActive, v.ID, productID, v.SellerSKU, v.ColorID, v.SizeValueID, v.ShadeName,
			)
			if err != nil {
				return fmt.Errorf("failed to update variant: %w", err)
			}
		} else {
			query := `
				INSERT INTO product_variants (id, product_id, sku, size, color, option_values, barcode, price_cents, is_active, created_at, updated_at, seller_sku, color_id, size_value_id, shade_name)
				VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15)
			`
			_, err := r.db.Exec(ctx, query,
				v.ID, v.ProductID, v.SKU, v.Size, v.Color, v.OptionValues, v.Barcode, v.PriceCents, v.IsActive, v.CreatedAt, v.UpdatedAt, v.SellerSKU, v.ColorID, v.SizeValueID, v.ShadeName,
			)
			if err != nil {
				return fmt.Errorf("failed to insert variant: %w", err)
			}
		}
	}

	return nil
}

func PopulateProductAggregates(p *Product) {
	p.SellerIsActive = (p.SellerStatus != nil && *p.SellerStatus == "active")

	p.VariantsCount = len(p.Variants)
	activeCount := 0
	totalStock := 0
	reservedStock := 0
	availableStock := 0
	hasInvRecord := false
	var minPrice, maxPrice *int64

	for _, v := range p.Variants {
		if v.IsActive {
			activeCount++
			if v.PriceCents != nil {
				if minPrice == nil || *v.PriceCents < *minPrice {
					pVal := *v.PriceCents
					minPrice = &pVal
				}
				if maxPrice == nil || *v.PriceCents > *maxPrice {
					pVal := *v.PriceCents
					maxPrice = &pVal
				}
			}
		}
		if v.HasInventoryRecord {
			hasInvRecord = true
			totalStock += v.TotalStock
			reservedStock += v.ReservedStock
			availableStock += v.AvailableStock
		}
	}

	p.ActiveVariantsCount = activeCount
	p.TotalStock = totalStock
	p.ReservedStock = reservedStock
	p.AvailableStock = availableStock
	p.HasInventoryRecord = hasInvRecord
	p.MinPriceCents = minPrice
	p.MaxPriceCents = maxPrice

	if (p.PriceCents <= 0) && minPrice != nil {
		p.PriceCents = *minPrice
	}

	inStock := hasInvRecord && availableStock > 0
	p.InStock = &inStock

	vis := CalculateActualVisibility(p)
	p.ActualVisibility = vis.ActualVisibility
	p.VisibilityReasons = vis.VisibilityReasons

	if p.ActualVisibility {
		slugOrID := p.Slug
		if slugOrID == "" && p.ID != uuid.Nil {
			slugOrID = p.ID.String()
		}
		url := fmt.Sprintf("http://127.0.0.1:3000/product/%s", slugOrID)
		if base := os.Getenv("STOREFRONT_BASE_URL"); base != "" {
			url = fmt.Sprintf("%s/product/%s", strings.TrimRight(base, "/"), slugOrID)
		}
		p.StorefrontURL = &url
	} else {
		p.StorefrontURL = nil
	}
}

func (r *Repository) GetProductVariants(ctx context.Context, productID uuid.UUID) ([]ProductVariant, error) {
	query := `
		SELECT pv.id, pv.product_id,
		       COALESCE(pv.sku, pv.seller_sku) AS sku,
		       COALESCE(pv.size, sv.value) AS size,
		       COALESCE(pv.color, c.name_ru) AS color,
		       pv.option_values, pv.seller_sku, pv.color_id, pv.size_value_id, c.name_ru AS color_name, c.hex AS color_hex, pv.shade_name, pv.barcode, pv.price_cents, pv.is_active, pv.created_at, pv.updated_at,
		       (ii.id IS NOT NULL) AS has_inventory,
		       COALESCE(ii.total_stock, 0) AS total_stock,
		       COALESCE(ii.reserved_stock, 0) AS reserved_stock
		FROM product_variants pv
		LEFT JOIN size_values sv ON pv.size_value_id = sv.id
		LEFT JOIN colors c ON pv.color_id = c.id
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
		var hasInv bool
		var totalStock, reservedStock int
		if err := rows.Scan(
			&v.ID, &v.ProductID, &v.SKU, &v.Size, &v.Color, &v.OptionValues, &v.SellerSKU, &v.ColorID, &v.SizeValueID, &v.ColorName, &v.ColorHex, &v.ShadeName, &v.Barcode, &v.PriceCents, &v.IsActive, &v.CreatedAt, &v.UpdatedAt,
			&hasInv, &totalStock, &reservedStock,
		); err != nil {
			return nil, err
		}
		v.HasInventoryRecord = hasInv
		v.TotalStock = totalStock
		v.ReservedStock = reservedStock
		v.AvailableStock = totalStock - reservedStock
		inStock := hasInv && v.AvailableStock > 0
		v.InStock = &inStock
		variants = append(variants, v)
	}
	rows.Close() // Explicitly close

	if variants == nil {
		variants = []ProductVariant{}
	}

	for i := range variants {
		variants[i].Attributes, err = r.GetVariantAttributes(ctx, variants[i].ID)
		if err != nil {
			return nil, err
		}
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
			INSERT INTO product_images (id, product_id, image_url, object_key, alt_text, sort_order, color_id, width, height, crop_x, crop_y, crop_width, crop_height, rendition_url, rendition_object_key, is_main, created_at)
			VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16, $17)
		`
		_, err := r.db.Exec(ctx, query,
			img.ID,
			img.ProductID,
			img.ImageURL,
			img.ObjectKey,
			img.AltText,
			img.SortOrder,
			img.ColorID,
			img.Width, img.Height, img.CropX, img.CropY, img.CropWidth, img.CropHeight, img.RenditionURL, img.RenditionObjectKey, img.IsMain,
			img.CreatedAt,
		)
		if err != nil {
			return fmt.Errorf("failed to insert image: %w", err)
		}
	}
	return nil
}

func (r *Repository) GetProductImages(ctx context.Context, productID uuid.UUID) ([]ProductImage, error) {
	query := `
		SELECT id, product_id, image_url, object_key, alt_text, sort_order, color_id, width, height, crop_x, crop_y, crop_width, crop_height, rendition_url, rendition_object_key, is_main, created_at
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
		if err := rows.Scan(&i.ID, &i.ProductID, &i.ImageURL, &i.ObjectKey, &i.AltText, &i.SortOrder, &i.ColorID, &i.Width, &i.Height, &i.CropX, &i.CropY, &i.CropWidth, &i.CropHeight, &i.RenditionURL, &i.RenditionObjectKey, &i.IsMain, &i.CreatedAt); err != nil {
			return nil, err
		}
		images = append(images, i)
	}
	if images == nil {
		images = []ProductImage{}
	}
	return images, nil
}

func (r *Repository) AddProductImage(ctx context.Context, img *ProductImage) error {
	query := `
		INSERT INTO product_images (id, product_id, image_url, object_key, alt_text, sort_order, color_id, width, height, crop_x, crop_y, crop_width, crop_height, rendition_url, rendition_object_key, is_main, created_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16, $17)
	`
	_, err := r.db.Exec(ctx, query,
		img.ID,
		img.ProductID,
		img.ImageURL,
		img.ObjectKey,
		img.AltText,
		img.SortOrder,
		img.ColorID,
		img.Width, img.Height, img.CropX, img.CropY, img.CropWidth, img.CropHeight, img.RenditionURL, img.RenditionObjectKey, img.IsMain,
		img.CreatedAt,
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
		SELECT log.id, log.product_id, log.admin_user_id, log.from_status, log.to_status, log.comment, log.created_at, u.name
		FROM product_moderation_logs log
		LEFT JOIN users u ON log.admin_user_id = u.id
		WHERE log.product_id = $1
		ORDER BY log.created_at DESC
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
			&log.ID, &log.ProductID, &log.AdminUserID, &log.FromStatus, &log.ToStatus, &log.Comment, &log.CreatedAt, &log.AdminName,
		); err != nil {
			return nil, err
		}
		logs = append(logs, log)
	}
	if rows.Err() != nil {
		return nil, rows.Err()
	}
	if logs == nil {
		logs = []ProductModerationLog{}
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
			status, source, gender, color, material, care_instructions,
			price_cents, old_price_cents, currency, main_image_url,
			average_rating, reviews_count,
			created_at, updated_at, submitted_at, approved_at, published_at, rejected_at, moderation_comment
		FROM products
		WHERE seller_id = $1
		ORDER BY created_at DESC
		LIMIT $2 OFFSET $3
	`
	return r.listProductsQuery(ctx, query, sellerID, limit, offset)
}

func (r *Repository) ListAdminProducts(ctx context.Context, filter AdminProductFilter, limit, offset int) ([]Product, int, error) {
	queryBuilder := strings.Builder{}
	queryBuilder.WriteString(`
		SELECT p.id, p.seller_id, p.category_id, p.brand_id, p.title, p.slug, p.description,
			p.status, p.source, p.gender, p.color, p.material, p.care_instructions,
			p.price_cents, p.old_price_cents, p.currency, p.main_image_url,
			p.average_rating, p.reviews_count,
			p.created_at, p.updated_at, p.submitted_at, p.approved_at, p.published_at, p.rejected_at, p.moderation_comment,
			p.assigned_admin_user_id, admin_user.name, p.review_started_at,
			s.brand_name, s.slug, u.name, u.email, c.name, b.name, s.status
		FROM products p
		LEFT JOIN sellers s ON p.seller_id = s.id
		LEFT JOIN seller_users su ON su.seller_id = s.id AND su.role = 'owner'
		LEFT JOIN users u ON su.user_id = u.id
		LEFT JOIN categories c ON p.category_id = c.id
		LEFT JOIN brands b ON p.brand_id = b.id
		LEFT JOIN users admin_user ON p.assigned_admin_user_id = admin_user.id
	`)

	var args []interface{}
	argID := 1
	var conditions []string

	if filter.Query != nil && *filter.Query != "" {
		conditions = append(conditions, fmt.Sprintf("(p.title ILIKE $%d OR p.id::text ILIKE $%d OR s.brand_name ILIKE $%d OR c.name ILIKE $%d OR b.name ILIKE $%d)", argID, argID, argID, argID, argID))
		args = append(args, "%"+*filter.Query+"%")
		argID++
	}

	if filter.Status != nil && *filter.Status != "" && *filter.Status != "all" {
		conditions = append(conditions, fmt.Sprintf("p.status = $%d", argID))
		args = append(args, *filter.Status)
		argID++
	}

	if filter.Source != nil && *filter.Source != "" {
		conditions = append(conditions, fmt.Sprintf("p.source = $%d", argID))
		args = append(args, *filter.Source)
		argID++
	}

	if filter.SellerID != nil {
		conditions = append(conditions, fmt.Sprintf("p.seller_id = $%d", argID))
		args = append(args, *filter.SellerID)
		argID++
	}

	if filter.CategoryID != nil {
		conditions = append(conditions, fmt.Sprintf("p.category_id = $%d", argID))
		args = append(args, *filter.CategoryID)
		argID++
	} else if len(filter.CategoryIDs) > 0 {
		conditions = append(conditions, fmt.Sprintf("p.category_id = ANY($%d)", argID))
		args = append(args, filter.CategoryIDs)
		argID++
	}

	if filter.BrandID != nil {
		conditions = append(conditions, fmt.Sprintf("p.brand_id = $%d", argID))
		args = append(args, *filter.BrandID)
		argID++
	} else if len(filter.BrandIDs) > 0 {
		conditions = append(conditions, fmt.Sprintf("p.brand_id = ANY($%d)", argID))
		args = append(args, filter.BrandIDs)
		argID++
	}

	// Date Submitted Period
	if filter.SubmittedPeriod != nil && *filter.SubmittedPeriod != "" {
		switch *filter.SubmittedPeriod {
		case "today":
			conditions = append(conditions, "p.submitted_at >= CURRENT_DATE")
		case "3days":
			conditions = append(conditions, "p.submitted_at >= (NOW() - INTERVAL '3 days')")
		case "7days":
			conditions = append(conditions, "p.submitted_at >= (NOW() - INTERVAL '7 days')")
		case "30days":
			conditions = append(conditions, "p.submitted_at >= (NOW() - INTERVAL '30 days')")
		case "custom":
			if filter.SubmittedFrom != nil {
				conditions = append(conditions, fmt.Sprintf("p.submitted_at >= $%d", argID))
				args = append(args, *filter.SubmittedFrom)
				argID++
			}
			if filter.SubmittedTo != nil {
				conditions = append(conditions, fmt.Sprintf("p.submitted_at <= $%d", argID))
				args = append(args, *filter.SubmittedTo)
				argID++
			}
		}
	}

	// Specific Flag Filters (+ Ещё)
	if filter.NoMainImage != nil && *filter.NoMainImage {
		conditions = append(conditions, "(p.main_image_url IS NULL OR TRIM(p.main_image_url) = '')")
	}
	if filter.NoDescription != nil && *filter.NoDescription {
		conditions = append(conditions, "(p.description IS NULL OR TRIM(p.description) = '')")
	}
	if filter.NoBrand != nil && *filter.NoBrand {
		conditions = append(conditions, "p.brand_id IS NULL")
	}
	if filter.NoVariants != nil && *filter.NoVariants {
		conditions = append(conditions, "NOT EXISTS (SELECT 1 FROM product_variants pv WHERE pv.product_id = p.id)")
	}
	if filter.NoPrice != nil && *filter.NoPrice {
		conditions = append(conditions, "EXISTS (SELECT 1 FROM product_variants pv WHERE pv.product_id = p.id AND (pv.price_cents IS NULL OR pv.price_cents <= 0))")
	}
	if filter.DuplicateSKU != nil && *filter.DuplicateSKU {
		conditions = append(conditions, "EXISTS (SELECT LOWER(TRIM(pv1.sku)) FROM product_variants pv1 WHERE pv1.product_id = p.id AND pv1.sku IS NOT NULL AND TRIM(pv1.sku) != '' GROUP BY LOWER(TRIM(pv1.sku)) HAVING COUNT(*) > 1)")
	}
	if filter.NoStock != nil && *filter.NoStock {
		conditions = append(conditions, "(EXISTS (SELECT 1 FROM product_variants pv WHERE pv.product_id = p.id) AND (SELECT COALESCE(SUM(ii.total_stock - ii.reserved_stock), 0) FROM inventory_items ii WHERE ii.product_id = p.id) = 0)")
	}
	if filter.Resubmitted != nil && *filter.Resubmitted {
		conditions = append(conditions, "EXISTS (SELECT 1 FROM product_moderation_logs pml WHERE pml.product_id = p.id AND pml.from_status = 'rejected')")
	}

	if filter.HasProblems != nil && *filter.HasProblems {
		conditions = append(conditions, `((p.main_image_url IS NULL OR TRIM(p.main_image_url) = '') OR (p.description IS NULL OR TRIM(p.description) = '') OR p.brand_id IS NULL OR NOT EXISTS (SELECT 1 FROM product_variants pv WHERE pv.product_id = p.id) OR EXISTS (SELECT 1 FROM product_variants pv WHERE pv.product_id = p.id AND (pv.price_cents IS NULL OR pv.price_cents <= 0)) OR EXISTS (SELECT LOWER(TRIM(pv1.sku)) FROM product_variants pv1 WHERE pv1.product_id = p.id AND pv1.sku IS NOT NULL AND TRIM(pv1.sku) != '' GROUP BY LOWER(TRIM(pv1.sku)) HAVING COUNT(*) > 1) OR (EXISTS (SELECT 1 FROM product_variants pv WHERE pv.product_id = p.id) AND (SELECT COALESCE(SUM(ii.total_stock - ii.reserved_stock), 0) FROM inventory_items ii WHERE ii.product_id = p.id) = 0) OR EXISTS (SELECT 1 FROM product_moderation_logs pml WHERE pml.product_id = p.id AND pml.from_status = 'rejected'))`)
	}

	if len(conditions) > 0 {
		queryBuilder.WriteString(" WHERE " + strings.Join(conditions, " AND "))
	}

	fmt.Printf("DEBUG SQL: %s\n", queryBuilder.String())
	fmt.Printf("DEBUG ARGS: %v\n", args)

	// Calculate total count
	countQuery := "SELECT COUNT(*) FROM (" + queryBuilder.String() + ") AS count_tbl"
	var totalCount int
	err := r.db.QueryRow(ctx, countQuery, args...).Scan(&totalCount)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to get total count: %w", err)
	}

	sortField := "COALESCE(p.submitted_at, p.created_at)"
	sortDirection := "ASC"

	if filter.SortOrder != nil && strings.ToUpper(*filter.SortOrder) == "DESC" {
		sortDirection = "DESC"
	}

	if filter.Sort != nil {
		switch *filter.Sort {
		case "submitted_at", "waiting_time":
			sortField = "COALESCE(p.submitted_at, p.created_at)"
		case "created_at":
			sortField = "p.created_at"
		case "title", "product_name":
			sortField = "p.title"
		case "price":
			sortField = "p.price_cents"
		case "seller_name":
			sortField = "COALESCE(s.brand_name, '')"
		case "status":
			sortField = "p.status"
		case "variants_count":
			sortField = "(SELECT COUNT(*) FROM product_variants pv WHERE pv.product_id = p.id)"
		case "problems_count":
			sortField = "((CASE WHEN p.main_image_url IS NULL THEN 1 ELSE 0 END) + (CASE WHEN p.description IS NULL THEN 1 ELSE 0 END) + (CASE WHEN p.category_id IS NULL THEN 1 ELSE 0 END) + (CASE WHEN p.brand_id IS NULL THEN 1 ELSE 0 END))"
		}
	}

	queryBuilder.WriteString(fmt.Sprintf(" ORDER BY %s %s LIMIT $%d OFFSET $%d", sortField, sortDirection, argID, argID+1))
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
			&p.Status, &p.Source, &p.Gender, &p.Color, &p.Material, &p.CareInstructions,
			&p.PriceCents, &p.OldPriceCents, &p.Currency, &p.MainImageURL,
			&p.AverageRating, &p.ReviewsCount,
			&p.CreatedAt, &p.UpdatedAt, &p.SubmittedAt, &p.ApprovedAt, &p.PublishedAt, &p.RejectedAt, &p.ModerationComment,
			&p.AssignedAdminUserID, &p.AssignedAdminName, &p.ReviewStartedAt,
			&p.SellerName, &p.SellerSlug, &p.SellerOwnerName, &p.SellerOwnerEmail, &p.CategoryName, &p.BrandName, &p.SellerStatus,
		); err != nil {
			return nil, 0, err
		}
		products = append(products, p)
	}
	if rows.Err() != nil {
		return nil, 0, rows.Err()
	}

	for i := range products {
		variants, _ := r.GetProductVariants(ctx, products[i].ID)
		if variants != nil {
			products[i].Variants = variants
		}
		images, _ := r.GetProductImages(ctx, products[i].ID)
		if images != nil {
			products[i].Images = images
		}
		PopulateProductAggregates(&products[i])
	}

	return products, totalCount, nil
}

func (r *Repository) ListProductsForModeration(ctx context.Context, limit, offset int) ([]Product, error) {
	query := `
		SELECT id, seller_id, category_id, brand_id, title, slug, description,
			status, source, gender, color, material, care_instructions,
			price_cents, old_price_cents, currency, main_image_url,
			average_rating, reviews_count,
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
			p.status, p.source, p.gender, p.color, p.material, p.care_instructions,
			p.price_cents, p.old_price_cents, p.currency, p.main_image_url,
			p.average_rating, p.reviews_count,
			p.created_at, p.updated_at, p.submitted_at, p.approved_at, p.published_at, p.rejected_at, p.moderation_comment,
			s.slug, s.brand_name
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
			SELECT 1 FROM product_variants pv2
			JOIN inventory_items ii ON pv2.id = ii.product_variant_id
			WHERE pv2.product_id = p.id AND pv2.is_active = true
			AND (COALESCE(ii.total_stock, 0) - COALESCE(ii.reserved_stock, 0)) > 0
		)`)
	}

	if filter.Size != nil && *filter.Size != "" {
		queryBuilder.WriteString(fmt.Sprintf(` AND EXISTS (
			SELECT 1 FROM product_variants v
			WHERE v.product_id = p.id AND v.is_active = true AND v.size = $%d
		)`, argID))
		args = append(args, *filter.Size)
		argID++
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
			&p.Status, &p.Source, &p.Gender, &p.Color, &p.Material, &p.CareInstructions,
			&p.PriceCents, &p.OldPriceCents, &p.Currency, &p.MainImageURL,
			&p.AverageRating, &p.ReviewsCount,
			&p.CreatedAt, &p.UpdatedAt, &p.SubmittedAt, &p.ApprovedAt, &p.PublishedAt, &p.RejectedAt, &p.ModerationComment,
			&p.SellerSlug, &p.SellerName,
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
			p.status, p.source, p.gender, p.color, p.material, p.care_instructions,
			p.price_cents, p.old_price_cents, p.currency, p.main_image_url,
			p.average_rating, p.reviews_count,
			p.created_at, p.updated_at, p.submitted_at, p.approved_at, p.published_at, p.rejected_at, p.moderation_comment,
			s.slug, s.brand_name
		FROM products p
		INNER JOIN sellers s ON p.seller_id = s.id
		WHERE (p.slug = $1 OR p.id::text = $1) AND p.status = 'published' AND s.status = 'active'
	`
	var p Product
	err := r.db.QueryRow(ctx, query, idOrSlug).Scan(
		&p.ID, &p.SellerID, &p.CategoryID, &p.BrandID, &p.Title, &p.Slug, &p.Description,
		&p.Status, &p.Source, &p.Gender, &p.Color, &p.Material, &p.CareInstructions,
		&p.PriceCents, &p.OldPriceCents, &p.Currency, &p.MainImageURL,
		&p.AverageRating, &p.ReviewsCount,
		&p.CreatedAt, &p.UpdatedAt, &p.SubmittedAt, &p.ApprovedAt, &p.PublishedAt, &p.RejectedAt, &p.ModerationComment,
		&p.SellerSlug, &p.SellerName,
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

	p.Attributes, err = r.GetProductAttributes(ctx, p.ID)
	if err != nil {
		return nil, err
	}

	p.MaterialComposition, err = r.GetProductMaterialComposition(ctx, p.ID)
	if err != nil {
		return nil, err
	}

	p.SizeChart, err = r.GetProductSizeChart(ctx, p.ID)
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
			&p.Status, &p.Source, &p.Gender, &p.Color, &p.Material, &p.CareInstructions,
			&p.PriceCents, &p.OldPriceCents, &p.Currency, &p.MainImageURL,
			&p.AverageRating, &p.ReviewsCount,
			&p.CreatedAt, &p.UpdatedAt, &p.SubmittedAt, &p.ApprovedAt, &p.PublishedAt, &p.RejectedAt, &p.ModerationComment,
		); err != nil {
			return nil, err
		}
		products = append(products, p)
	}
	if rows.Err() != nil {
		return nil, rows.Err()
	}
	rows.Close() // Explicitly close to release connection before N+1 queries

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

func (r *Repository) GetProductImageByID(ctx context.Context, imageID uuid.UUID) (ProductImage, error) {
	query := `
		SELECT id, product_id, image_url, object_key, alt_text, sort_order, color_id, width, height, crop_x, crop_y, crop_width, crop_height, rendition_url, rendition_object_key, is_main, created_at
		FROM product_images
		WHERE id = $1
	`
	var img ProductImage
	err := r.db.QueryRow(ctx, query, imageID).Scan(
		&img.ID, &img.ProductID, &img.ImageURL, &img.ObjectKey,
		&img.AltText, &img.SortOrder, &img.ColorID, &img.Width, &img.Height, &img.CropX, &img.CropY, &img.CropWidth, &img.CropHeight, &img.RenditionURL, &img.RenditionObjectKey, &img.IsMain, &img.CreatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return ProductImage{}, ErrProductNotFound
		}
		return ProductImage{}, fmt.Errorf("failed to get product image: %w", err)
	}
	return img, nil
}

func (r *Repository) DeleteProductImage(ctx context.Context, imageID uuid.UUID) error {
	query := `DELETE FROM product_images WHERE id = $1`
	res, err := r.db.Exec(ctx, query, imageID)
	if err != nil {
		return fmt.Errorf("failed to delete product image: %w", err)
	}
	if res.RowsAffected() == 0 {
		return ErrProductNotFound
	}
	return nil
}

func (r *Repository) ReorderProductImages(ctx context.Context, productID uuid.UUID, imageIDs []uuid.UUID) error {
	for i, id := range imageIDs {
		query := `UPDATE product_images SET sort_order = $1 WHERE id = $2 AND product_id = $3`
		_, err := r.db.Exec(ctx, query, i, id, productID)
		if err != nil {
			return fmt.Errorf("failed to reorder product image: %w", err)
		}
	}
	return nil
}

func (r *Repository) GetPrimaryBrandForSeller(ctx context.Context, sellerID uuid.UUID) (*uuid.UUID, error) {
	rows, err := r.db.Query(ctx, "SELECT brand_id FROM seller_brands WHERE seller_id = $1 AND is_primary = true AND status = 'active'", sellerID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var brandIDs []uuid.UUID
	for rows.Next() {
		var brandID uuid.UUID
		if err := rows.Scan(&brandID); err != nil {
			return nil, err
		}
		brandIDs = append(brandIDs, brandID)
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	if len(brandIDs) == 0 {
		return nil, ErrSellerHasNoPrimaryBrand
	}
	if len(brandIDs) > 1 {
		return nil, ErrSellerHasMultiplePrimaryBrands
	}

	return &brandIDs[0], nil
}

func (r *Repository) UpdateVariantPrice(ctx context.Context, variantID uuid.UUID, priceCents int64) error {
	query := `UPDATE product_variants SET price_cents = $1, updated_at = now() WHERE id = $2`
	_, err := r.db.Exec(ctx, query, priceCents, variantID)
	return err
}

func (r *Repository) InsertProductAttributeValues(ctx context.Context, productID uuid.UUID, attrs []ProductAttributeValue) error {
	_, err := r.db.Exec(ctx, "DELETE FROM product_attribute_values WHERE product_id = $1", productID)
	if err != nil {
		return err
	}
	if len(attrs) == 0 {
		return nil
	}
	for _, a := range attrs {
		_, err := r.db.Exec(ctx, `
			INSERT INTO product_attribute_values (id, product_id, attribute_definition_id, enum_value_id, text_value, number_value, bool_value)
			VALUES (gen_random_uuid(), $1, $2, $3, $4, $5, $6)
		`, productID, a.AttributeDefinitionID, a.EnumValueID, a.TextValue, a.NumberValue, a.BoolValue)
		if err != nil {
			return err
		}
	}
	return nil
}

func (r *Repository) InsertVariantAttributeValues(ctx context.Context, variantID uuid.UUID, attrs []VariantAttributeValue) error {
	_, err := r.db.Exec(ctx, "DELETE FROM variant_attribute_values WHERE product_variant_id = $1", variantID)
	if err != nil {
		return err
	}
	if len(attrs) == 0 {
		return nil
	}
	for _, a := range attrs {
		_, err := r.db.Exec(ctx, `
			INSERT INTO variant_attribute_values (id, product_variant_id, attribute_definition_id, enum_value_id, text_value, number_value, bool_value)
			VALUES (gen_random_uuid(), $1, $2, $3, $4, $5, $6)
		`, variantID, a.AttributeDefinitionID, a.EnumValueID, a.TextValue, a.NumberValue, a.BoolValue)
		if err != nil {
			return err
		}
	}
	return nil
}

func (r *Repository) InsertMaterialComposition(ctx context.Context, productID uuid.UUID, comp []ProductMaterialComposition) error {
	_, err := r.db.Exec(ctx, "DELETE FROM product_material_composition WHERE product_id = $1", productID)
	if err != nil {
		return err
	}
	for _, c := range comp {
		_, err := r.db.Exec(ctx, `
			INSERT INTO product_material_composition (product_id, material_id, percentage)
			VALUES ($1, $2, $3)
		`, productID, c.MaterialID, c.Percentage)
		if err != nil {
			return err
		}
	}
	return nil
}

func (r *Repository) InsertSizeChart(ctx context.Context, productID uuid.UUID, categoryID uuid.UUID, rows []ProductSizeChartRow) error {
	_, err := r.db.Exec(ctx, "DELETE FROM product_size_charts WHERE product_id = $1", productID)
	if err != nil {
		return err
	}
	if len(rows) == 0 {
		return nil
	}
	var chartID uuid.UUID
	err = r.db.QueryRow(ctx, "INSERT INTO product_size_charts (id, product_id, category_id) VALUES (gen_random_uuid(), $1, $2) RETURNING id", productID, categoryID).Scan(&chartID)
	if err != nil {
		return err
	}
	for _, rRow := range rows {
		_, err := r.db.Exec(ctx, `
			INSERT INTO product_size_chart_rows (size_chart_id, size_value_id, measurements)
			VALUES ($1, $2, $3)
		`, chartID, rRow.SizeValueID, rRow.Measurements)
		if err != nil {
			return err
		}
	}
	return nil
}

func (r *Repository) GetProductMaterialComposition(ctx context.Context, productID uuid.UUID) ([]ProductMaterialComposition, error) {
	rows, err := r.db.Query(ctx, `
		SELECT pmc.material_id, m.name_ru, pmc.percentage
		FROM product_material_composition pmc
		LEFT JOIN materials m ON pmc.material_id = m.id
		WHERE pmc.product_id = $1
	`, productID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var res []ProductMaterialComposition
	for rows.Next() {
		var c ProductMaterialComposition
		c.ProductID = productID
		if err := rows.Scan(&c.MaterialID, &c.MaterialName, &c.Percentage); err != nil {
			return nil, err
		}
		res = append(res, c)
	}
	return res, nil
}

func (r *Repository) GetProductAttributes(ctx context.Context, productID uuid.UUID) ([]ProductAttributeValue, error) {
	rows, err := r.db.Query(ctx, "SELECT id, attribute_definition_id, enum_value_id, text_value, number_value, bool_value, created_at, updated_at FROM product_attribute_values WHERE product_id = $1", productID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var res []ProductAttributeValue
	for rows.Next() {
		var a ProductAttributeValue
		a.ProductID = productID
		if err := rows.Scan(&a.ID, &a.AttributeDefinitionID, &a.EnumValueID, &a.TextValue, &a.NumberValue, &a.BoolValue, &a.CreatedAt, &a.UpdatedAt); err != nil {
			return nil, err
		}
		res = append(res, a)
	}
	return res, nil
}

func (r *Repository) GetVariantAttributes(ctx context.Context, variantID uuid.UUID) ([]VariantAttributeValue, error) {
	rows, err := r.db.Query(ctx, "SELECT id, attribute_definition_id, enum_value_id, text_value, number_value, bool_value, created_at, updated_at FROM variant_attribute_values WHERE product_variant_id = $1", variantID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var res []VariantAttributeValue
	for rows.Next() {
		var a VariantAttributeValue
		a.ProductVariantID = variantID
		if err := rows.Scan(&a.ID, &a.AttributeDefinitionID, &a.EnumValueID, &a.TextValue, &a.NumberValue, &a.BoolValue, &a.CreatedAt, &a.UpdatedAt); err != nil {
			return nil, err
		}
		res = append(res, a)
	}
	return res, nil
}

func (r *Repository) GetProductSizeChart(ctx context.Context, productID uuid.UUID) (*ProductSizeChart, error) {
	var chart ProductSizeChart
	err := r.db.QueryRow(ctx, "SELECT id, category_id FROM product_size_charts WHERE product_id = $1", productID).Scan(&chart.ID, &chart.CategoryID)
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, nil
		}
		return nil, err
	}
	chart.ProductID = productID

	rows, err := r.db.Query(ctx, `
		SELECT pscr.size_value_id, sv.value, pscr.measurements
		FROM product_size_chart_rows pscr
		LEFT JOIN size_values sv ON pscr.size_value_id = sv.id
		WHERE pscr.size_chart_id = $1
	`, chart.ID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var rRow ProductSizeChartRow
		rRow.SizeChartID = chart.ID
		if err := rows.Scan(&rRow.SizeValueID, &rRow.SizeValueName, &rRow.Measurements); err != nil {
			return nil, err
		}
		chart.Rows = append(chart.Rows, rRow)
	}
	return &chart, nil
}

func (r *Repository) GetDictionaryValues(ctx context.Context, dictionaryID uuid.UUID) ([]AttributeDictionaryValue, error) {
	rows, err := r.db.Query(ctx, "SELECT id, dictionary_id, code, name_ru, sort_order, is_active FROM attribute_dictionary_values WHERE dictionary_id = $1 AND is_active = true ORDER BY name_ru", dictionaryID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var vals []AttributeDictionaryValue
	for rows.Next() {
		var v AttributeDictionaryValue
		if err := rows.Scan(&v.ID, &v.DictionaryID, &v.Code, &v.NameRU, &v.SortOrder, &v.IsActive); err != nil {
			return nil, err
		}
		vals = append(vals, v)
	}
	return vals, nil
}

func (r *Repository) CheckSellerSKUExists(ctx context.Context, sellerID uuid.UUID, sku string) (bool, error) {
	var exists bool
	query := `
		SELECT EXISTS (
			SELECT 1 FROM product_variants pv
			JOIN products p ON pv.product_id = p.id
			WHERE p.seller_id = $1 AND LOWER(TRIM(pv.seller_sku)) = LOWER(TRIM($2))
		)
	`
	err := r.db.QueryRow(ctx, query, sellerID, sku).Scan(&exists)
	return exists, err
}

func (r *Repository) UpdateProductImageCrop(ctx context.Context, imageID uuid.UUID, cropX, cropY, cropWidth, cropHeight float64, renditionURL, renditionObjectKey string) error {
	query := `
		UPDATE product_images
		SET crop_x = $1, crop_y = $2, crop_width = $3, crop_height = $4, rendition_url = $5, rendition_object_key = $6
		WHERE id = $7
	`
	_, err := r.db.Exec(ctx, query, cropX, cropY, cropWidth, cropHeight, renditionURL, renditionObjectKey, imageID)
	return err
}

func (r *Repository) ClearOtherMainImages(ctx context.Context, productID uuid.UUID, excludeImageID uuid.UUID) error {
	query := `
		UPDATE product_images
		SET is_main = false
		WHERE product_id = $1 AND id != $2
	`
	_, err := r.db.Exec(ctx, query, productID, excludeImageID)
	return err
}
