#!/bin/bash
cat << 'INNER_EOF' >> backend/internal/inventory/repository.go

func (r *Repository) ListAdminInventoryRich(ctx context.Context, q, sellerId, source string, lowStock bool, limit, offset int) ([]AdminInventoryItem, int, error) {
	baseQuery := `
		FROM inventory_items i
		JOIN products p ON i.product_id = p.id
		JOIN product_variants pv ON i.product_variant_id = pv.id
		LEFT JOIN seller_users su ON i.seller_id = su.id
		WHERE 1=1
	`
	args := []interface{}{}
	argID := 1

	if q != "" {
		baseQuery += ` AND (p.title ILIKE $` + fmt.Sprintf("%d", argID) + ` OR pv.sku ILIKE $` + fmt.Sprintf("%d", argID+1) + `)`
		args = append(args, "%"+q+"%", "%"+q+"%")
		argID += 2
	}

	if sellerId != "" {
		baseQuery += ` AND i.seller_id = $` + fmt.Sprintf("%d", argID)
		args = append(args, sellerId)
		argID++
	}

	if source == "auction_direct_sale" {
		baseQuery += ` AND i.seller_id = '` + common.PlatformSellerIDStr + `'`
	} else if source == "seller" {
		baseQuery += ` AND i.seller_id != '` + common.PlatformSellerIDStr + `'`
	}

	if lowStock {
		baseQuery += ` AND (i.total_stock - i.reserved_stock) <= 10`
	}

	countQuery := "SELECT COUNT(*) " + baseQuery
	var totalCount int
	err := r.db.QueryRow(ctx, countQuery, args...).Scan(&totalCount)
	if err != nil {
		return nil, 0, err
	}

	selectQuery := `
		SELECT 
			i.id, i.product_id, i.product_variant_id, 
			p.title, pv.size, pv.color, pv.sku,
			i.seller_id, su.brand_name,
			CASE WHEN i.seller_id::text = '` + common.PlatformSellerIDStr + `' THEN 'auction_direct_sale' ELSE 'seller' END,
			i.total_stock, i.reserved_stock, i.created_at, i.updated_at
	` + baseQuery + `
		ORDER BY i.created_at DESC
		LIMIT $` + fmt.Sprintf("%d", argID) + ` OFFSET $` + fmt.Sprintf("%d", argID+1)

	args = append(args, limit, offset)

	rows, err := r.db.Query(ctx, selectQuery, args...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	var items []AdminInventoryItem
	for rows.Next() {
		var item AdminInventoryItem
		var size, color, sku *string
		var brandName *string
		var cAt, uAt time.Time

		err := rows.Scan(
			&item.ID, &item.ProductID, &item.ProductVariantID,
			&item.ProductTitle, &size, &color, &sku,
			&item.SellerID, &brandName,
			&item.Source,
			&item.TotalStock, &item.ReservedStock, &cAt, &uAt,
		)
		if err != nil {
			return nil, 0, err
		}

		item.AvailableStock = item.TotalStock - item.ReservedStock
		item.CreatedAt = cAt.Format(time.RFC3339)
		item.UpdatedAt = uAt.Format(time.RFC3339)

		variantParts := []string{}
		if sku != nil && *sku != "" {
			variantParts = append(variantParts, *sku)
		}
		if size != nil && *size != "" {
			variantParts = append(variantParts, *size)
		}
		if color != nil && *color != "" {
			variantParts = append(variantParts, *color)
		}
		
		item.VariantLabel = ""
		for idx, part := range variantParts {
			if idx > 0 {
				item.VariantLabel += " / "
			}
			item.VariantLabel += part
		}

		if brandName != nil && *brandName != "" {
			item.SellerName = *brandName
		} else {
			item.SellerName = item.SellerID.String()
		}

		items = append(items, item)
	}

	return items, totalCount, nil
}
INNER_EOF

# Add imports
sed -i.bak -e '/"github.com\/google\/uuid"/a\
	"time"\
	"github.com/eirfefrggrvgrgrtbg/shop-zamk/backend/internal/common"
' backend/internal/inventory/repository.go
