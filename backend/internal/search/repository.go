package search

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/eirfefrggrvgrgrtbg/shop-zamk/backend/internal/platform/postgres"
)

const MaxSearchResults = 20

type Repository struct {
	db postgres.DBTX
}

func NewRepository(db postgres.DBTX) *Repository {
	return &Repository{db: db}
}

func (r *Repository) GlobalSearch(ctx context.Context, q string, perms AllowedPermissions) ([]GlobalSearchResult, error) {
	if !perms.HasAny() {
		return []GlobalSearchResult{}, nil
	}

	results := make([]GlobalSearchResult, 0, MaxSearchResults)
	seen := make(map[string]bool)

	addResult := func(item GlobalSearchResult) bool {
		key := string(item.Type) + ":" + item.ID
		if seen[key] {
			return false
		}
		seen[key] = true
		results = append(results, item)
		return len(results) >= MaxSearchResults
	}

	// 1. EXACT SEARCH
	exactResults, err := r.searchExact(ctx, q, perms)
	if err != nil {
		return nil, fmt.Errorf("search exact: %w", err)
	}

	for _, item := range exactResults {
		if addResult(item) {
			return results, nil
		}
	}

	// 2. PARTIAL SEARCH (if quota remaining)
	if len(results) < MaxSearchResults {
		partialResults, err := r.searchPartial(ctx, q, perms, MaxSearchResults-len(results), seen)
		if err != nil {
			return nil, fmt.Errorf("search partial: %w", err)
		}
		for _, item := range partialResults {
			if addResult(item) {
				return results, nil
			}
		}
	}

	return results, nil
}

func (r *Repository) searchExact(ctx context.Context, q string, perms AllowedPermissions) ([]GlobalSearchResult, error) {
	var results []GlobalSearchResult

	// 1. Exact Operational Identifier Matches
	// 1a. Orders
	if perms.CanReadOrders {
		orderQuery := `
			SELECT id, order_number, customer_name, customer_email, status
			FROM orders
			WHERE order_number = $1 OR order_number = UPPER($1)
			LIMIT 5
		`
		rows, err := r.db.Query(ctx, orderQuery, q)
		if err != nil {
			return nil, fmt.Errorf("query exact orders: %w", err)
		}
		for rows.Next() {
			var id uuid.UUID
			var orderNum, custName, custEmail, status string
			if err := rows.Scan(&id, &orderNum, &custName, &custEmail, &status); err != nil {
				rows.Close()
				return nil, fmt.Errorf("scan exact order: %w", err)
			}
			subtitle := fmt.Sprintf("%s · %s · %s", custName, custEmail, FormatOrderStatus(status))
			results = append(results, GlobalSearchResult{
				Type:                ResultTypeOrder,
				ID:                  id.String(),
				Title:               orderNum,
				Subtitle:            subtitle,
				CanonicalIdentifier: orderNum,
				NavigationTarget:    "/orders/" + id.String(),
			})
		}
		if err := rows.Err(); err != nil {
			rows.Close()
			return nil, fmt.Errorf("rows exact orders: %w", err)
		}
		rows.Close()
	}

	// 1b. Inventory Units (ZMU)
	if perms.CanReadInventory {
		unitQuery := `
			SELECT iu.id, iu.unit_code, iu.status, p.title
			FROM inventory_units iu
			JOIN product_variants pv ON pv.id = iu.product_variant_id
			JOIN products p ON p.id = pv.product_id
			WHERE iu.unit_code = $1 OR iu.unit_code = UPPER($1)
			LIMIT 5
		`
		rows, err := r.db.Query(ctx, unitQuery, q)
		if err != nil {
			return nil, fmt.Errorf("query exact inventory units: %w", err)
		}
		for rows.Next() {
			var id uuid.UUID
			var unitCode, status, prodTitle string
			if err := rows.Scan(&id, &unitCode, &status, &prodTitle); err != nil {
				rows.Close()
				return nil, fmt.Errorf("scan exact inventory unit: %w", err)
			}
			subtitle := fmt.Sprintf("%s · %s", prodTitle, FormatUnitStatus(status))
			results = append(results, GlobalSearchResult{
				Type:                ResultTypeInventoryUnit,
				ID:                  id.String(),
				Title:               unitCode,
				Subtitle:            subtitle,
				CanonicalIdentifier: unitCode,
				NavigationTarget:    "/inventory",
			})
		}
		if err := rows.Err(); err != nil {
			rows.Close()
			return nil, fmt.Errorf("rows exact inventory units: %w", err)
		}
		rows.Close()
	}

	// 1c. Product Variants (ZMK / Seller SKU)
	if perms.CanReadProducts {
		variantQuery := `
			SELECT pv.id, pv.product_id, pv.barcode, pv.seller_sku, p.title
			FROM product_variants pv
			JOIN products p ON p.id = pv.product_id
			WHERE pv.barcode = $1 OR pv.barcode = UPPER($1) OR pv.barcode = LOWER($1) OR LOWER(TRIM(pv.seller_sku)) = LOWER(TRIM($1))
			LIMIT 5
		`
		rows, err := r.db.Query(ctx, variantQuery, q)
		if err != nil {
			return nil, fmt.Errorf("query exact product variants: %w", err)
		}
		for rows.Next() {
			var id, prodID uuid.UUID
			var barcode, sellerSku *string
			var prodTitle string
			if err := rows.Scan(&id, &prodID, &barcode, &sellerSku, &prodTitle); err != nil {
				rows.Close()
				return nil, fmt.Errorf("scan exact variant: %w", err)
			}

			// Clarify which identifier was matched
			var matchedID string
			bCode := ""
			if barcode != nil {
				bCode = *barcode
			}
			sSku := ""
			if sellerSku != nil {
				sSku = *sellerSku
			}

			if bCode != "" && strings.EqualFold(bCode, q) {
				matchedID = bCode
			} else if sSku != "" && strings.EqualFold(sSku, q) {
				matchedID = sSku
			} else if bCode != "" {
				matchedID = bCode
			} else {
				matchedID = sSku
			}

			var subtitleParts []string
			subtitleParts = append(subtitleParts, prodTitle)
			if bCode != "" {
				subtitleParts = append(subtitleParts, bCode)
			}
			if sSku != "" {
				subtitleParts = append(subtitleParts, sSku)
			}

			results = append(results, GlobalSearchResult{
				Type:                ResultTypeProductVariant,
				ID:                  id.String(),
				Title:               matchedID,
				Subtitle:            strings.Join(subtitleParts, " · "),
				CanonicalIdentifier: matchedID,
				NavigationTarget:    "/products/" + prodID.String(),
			})
		}
		if err := rows.Err(); err != nil {
			rows.Close()
			return nil, fmt.Errorf("rows exact product variants: %w", err)
		}
		rows.Close()
	}

	// 2. Exact Customer Email
	if perms.CanReadUsers {
		custQuery := `
			SELECT id, name, email
			FROM users
			WHERE role = 'customer' AND LOWER(TRIM(email)) = LOWER(TRIM($1))
			LIMIT 5
		`
		rows, err := r.db.Query(ctx, custQuery, q)
		if err != nil {
			return nil, fmt.Errorf("query exact customers: %w", err)
		}
		for rows.Next() {
			var id uuid.UUID
			var name, email string
			if err := rows.Scan(&id, &name, &email); err != nil {
				rows.Close()
				return nil, fmt.Errorf("scan exact customer: %w", err)
			}
			title := name
			if strings.TrimSpace(title) == "" {
				title = email
			}
			results = append(results, GlobalSearchResult{
				Type:                ResultTypeCustomer,
				ID:                  id.String(),
				Title:               title,
				Subtitle:            email,
				CanonicalIdentifier: email,
				NavigationTarget:    "/users",
			})
		}
		if err := rows.Err(); err != nil {
			rows.Close()
			return nil, fmt.Errorf("rows exact customers: %w", err)
		}
		rows.Close()
	}

	// 3. Linked Returns from exact matched ORD
	if perms.CanReadReturns {
		returnQuery := `
			SELECT ret.id, o.order_number, ret.status, ret.created_at
			FROM returns ret
			JOIN orders o ON o.id = ret.order_id
			WHERE o.order_number = $1 OR o.order_number = UPPER($1)
			ORDER BY ret.created_at DESC, ret.id ASC
			LIMIT 10
		`
		rows, err := r.db.Query(ctx, returnQuery, q)
		if err != nil {
			return nil, fmt.Errorf("query returns for exact order: %w", err)
		}
		for rows.Next() {
			var id uuid.UUID
			var orderNum, status string
			var createdAt time.Time
			if err := rows.Scan(&id, &orderNum, &status, &createdAt); err != nil {
				rows.Close()
				return nil, fmt.Errorf("scan return: %w", err)
			}
			subtitle := fmt.Sprintf("Возврат · %s · %s", FormatReturnStatus(status), formatRussianDate(createdAt))
			results = append(results, GlobalSearchResult{
				Type:                ResultTypeReturn,
				ID:                  id.String(),
				Title:               orderNum,
				Subtitle:            subtitle,
				CanonicalIdentifier: orderNum,
				NavigationTarget:    "/returns",
			})
		}
		if err := rows.Err(); err != nil {
			rows.Close()
			return nil, fmt.Errorf("rows returns for exact order: %w", err)
		}
		rows.Close()
	}

	return results, nil
}

func (r *Repository) searchPartial(ctx context.Context, q string, perms AllowedPermissions, limit int, seenIDs map[string]bool) ([]GlobalSearchResult, error) {
	var results []GlobalSearchResult
	escapedPattern := "%" + escapeLikePattern(q) + "%"

	// 4. Partial Customer Email
	if perms.CanReadUsers && len(results) < limit {
		remaining := limit - len(results)
		custQuery := `
			SELECT id, name, email
			FROM users
			WHERE role = 'customer' AND email ILIKE $1 ESCAPE '\'
			ORDER BY email ASC, id ASC
			LIMIT $2
		`
		rows, err := r.db.Query(ctx, custQuery, escapedPattern, remaining)
		if err != nil {
			return nil, fmt.Errorf("query partial customers: %w", err)
		}
		for rows.Next() {
			var id uuid.UUID
			var name, email string
			if err := rows.Scan(&id, &name, &email); err != nil {
				rows.Close()
				return nil, fmt.Errorf("scan partial customer: %w", err)
			}
			key := string(ResultTypeCustomer) + ":" + id.String()
			if seenIDs[key] {
				continue
			}
			title := name
			if strings.TrimSpace(title) == "" {
				title = email
			}
			results = append(results, GlobalSearchResult{
				Type:                ResultTypeCustomer,
				ID:                  id.String(),
				Title:               title,
				Subtitle:            email,
				CanonicalIdentifier: email,
				NavigationTarget:    "/users",
			})
		}
		if err := rows.Err(); err != nil {
			rows.Close()
			return nil, fmt.Errorf("rows partial customers: %w", err)
		}
		rows.Close()
	}

	// 5. Partial Product Title
	if perms.CanReadProducts && len(results) < limit {
		remaining := limit - len(results)
		prodQuery := `
			SELECT id, title, slug, status
			FROM products
			WHERE title ILIKE $1 ESCAPE '\'
			ORDER BY title ASC, id ASC
			LIMIT $2
		`
		rows, err := r.db.Query(ctx, prodQuery, escapedPattern, remaining)
		if err != nil {
			return nil, fmt.Errorf("query partial products: %w", err)
		}
		for rows.Next() {
			var id uuid.UUID
			var title, slug, status string
			if err := rows.Scan(&id, &title, &slug, &status); err != nil {
				rows.Close()
				return nil, fmt.Errorf("scan partial product: %w", err)
			}
			key := string(ResultTypeProduct) + ":" + id.String()
			if seenIDs[key] {
				continue
			}
			subtitle := fmt.Sprintf("%s · %s", slug, FormatProductStatus(status))
			results = append(results, GlobalSearchResult{
				Type:                ResultTypeProduct,
				ID:                  id.String(),
				Title:               title,
				Subtitle:            subtitle,
				CanonicalIdentifier: slug,
				NavigationTarget:    "/products/" + id.String(),
			})
		}
		if err := rows.Err(); err != nil {
			rows.Close()
			return nil, fmt.Errorf("rows partial products: %w", err)
		}
		rows.Close()
	}

	return results, nil
}

func escapeLikePattern(s string) string {
	s = strings.ReplaceAll(s, `\`, `\\`)
	s = strings.ReplaceAll(s, `%`, `\%`)
	s = strings.ReplaceAll(s, `_`, `\_`)
	return s
}

func FormatOrderStatus(s string) string {
	switch strings.ToLower(s) {
	case "created":
		return "Создан"
	case "awaiting_payment", "pending_payment":
		return "Ожидает оплаты"
	case "paid":
		return "Оплачен"
	case "assembling", "processing":
		return "В сборке"
	case "packed":
		return "Упакован"
	case "shipped":
		return "Отправлен"
	case "delivered":
		return "Доставлен"
	case "cancelled":
		return "Отменен"
	case "returned":
		return "Возвращен"
	case "refunded":
		return "Возврат средств"
	default:
		return s
	}
}

func FormatReturnStatus(s string) string {
	switch strings.ToLower(s) {
	case "requested":
		return "Новая заявка"
	case "needs_info":
		return "Ожидает ответа"
	case "approved":
		return "Одобрен"
	case "receiving":
		return "Приемка"
	case "item_received":
		return "Товар принят"
	case "rejected":
		return "Отклонен"
	case "refunded":
		return "Возврат средств"
	case "completed":
		return "Завершен"
	case "cancelled":
		return "Отменен"
	default:
		return s
	}
}

func FormatUnitStatus(s string) string {
	switch strings.ToLower(s) {
	case "warehouse":
		return "На складе"
	case "expected":
		return "Ожидается"
	case "damaged":
		return "Поврежден"
	case "written_off":
		return "Списан"
	case "shipped":
		return "Отгружен"
	default:
		return s
	}
}

func FormatProductStatus(s string) string {
	switch strings.ToLower(s) {
	case "draft":
		return "Черновик"
	case "pending_moderation":
		return "На модерации"
	case "approved":
		return "Одобрен"
	case "rejected":
		return "Отклонен"
	case "published":
		return "Опубликован"
	case "hidden":
		return "Скрыт"
	case "blocked":
		return "Заблокирован"
	case "archived":
		return "В архиве"
	default:
		return s
	}
}

func formatRussianDate(t time.Time) string {
	months := [...]string{
		"января", "февраля", "марта", "апреля", "мая", "июня",
		"июля", "августа", "сентября", "октября", "ноября", "декабря",
	}
	mIndex := int(t.Month()) - 1
	if mIndex >= 0 && mIndex < 12 {
		return fmt.Sprintf("%d %s", t.Day(), months[mIndex])
	}
	return t.Format("02.01.2006")
}
