package search_test

import (
	"context"
	"fmt"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/eirfefrggrvgrgrtbg/shop-zamk/backend/internal/search"
	"github.com/eirfefrggrvgrgrtbg/shop-zamk/backend/internal/testutil"
)

var uuidRegex = regexp.MustCompile(`^[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}$`)

func TestM61A_AdminGlobalSearch(t *testing.T) {
	ctx := context.Background()
	dsn := testutil.GetTestDatabaseURL()
	pool, err := pgxpool.New(ctx, dsn)
	require.NoError(t, err)
	defer pool.Close()

	// Strict canonical DB safety guard
	testutil.AssertTestDatabase(t, pool)

	// Clean up only our test data by unique prefix/keys
	cleanup := func() {
		_, _ = pool.Exec(ctx, `DELETE FROM inventory_units WHERE unit_code LIKE 'ZMU-7K9%'`)
		_, _ = pool.Exec(ctx, `DELETE FROM seller_supply_items WHERE id IN (SELECT id FROM seller_supply_items WHERE variant_id IN (SELECT id FROM product_variants WHERE barcode LIKE 'ZMK-99%'))`)
		_, _ = pool.Exec(ctx, `DELETE FROM seller_supplies WHERE supply_number LIKE 'SUP-99%'`)
		_, _ = pool.Exec(ctx, `DELETE FROM product_variants WHERE barcode LIKE 'ZMK-99%' OR seller_sku LIKE 'Sku-98%'`)
		_, _ = pool.Exec(ctx, `DELETE FROM products WHERE slug LIKE 'm61a-%'`)
		_, _ = pool.Exec(ctx, `DELETE FROM returns WHERE reason = 'm61a_test'`)
		_, _ = pool.Exec(ctx, `DELETE FROM order_fulfillments WHERE id IN (SELECT id FROM order_fulfillments WHERE order_id IN (SELECT id FROM orders WHERE order_number LIKE 'ORD-99%'))`)
		_, _ = pool.Exec(ctx, `DELETE FROM orders WHERE order_number LIKE 'ORD-99%'`)
		_, _ = pool.Exec(ctx, `DELETE FROM sellers WHERE slug LIKE 'm61a-%'; DELETE FROM users WHERE email LIKE 'm61a_%'`)
	}
	cleanup()
	defer cleanup()

	// 1. Seed Fixtures
	sellerID := uuid.New()
	customerID := uuid.New()
	productID := uuid.New()
	variantID := uuid.New()
	orderID := uuid.New()
	fulfillmentID := uuid.New()
	return1ID := uuid.New()
	return2ID := uuid.New()
	supplyID := uuid.New()
	supplyItemID := uuid.New()
	unitID := uuid.New()

	// Seller
	_, err = pool.Exec(ctx, `
		INSERT INTO users (id, name, email, password_hash, role, created_at, updated_at)
		VALUES ($1, 'M61A Seller', 'm61a_seller@zamk.local', 'hash', 'seller', NOW(), NOW())
	`, sellerID)
	require.NoError(t, err)

	_, err = pool.Exec(ctx, `
		INSERT INTO sellers (id, brand_name, slug, status, created_at, updated_at)
		VALUES ($1, 'M61A Brand', 'm61a-brand', 'active', NOW(), NOW())
	`, sellerID)
	require.NoError(t, err)

	// Customer
	_, err = pool.Exec(ctx, `
		INSERT INTO users (id, name, email, password_hash, role, created_at, updated_at)
		VALUES ($1, 'M61A Customer', 'm61a_customer@zamk.local', 'hash', 'customer', NOW(), NOW())
	`, customerID)
	require.NoError(t, err)

	// Product
	_, err = pool.Exec(ctx, `
		INSERT INTO products (id, title, slug, status, price_cents, average_rating, seller_id, created_at, updated_at)
		VALUES ($1, 'M61A Global Search Wool Coat', 'm61a-wool-coat', 'published', 15000, 0, $2, NOW(), NOW())
	`, productID, sellerID)
	require.NoError(t, err)

	// Variant with both ZMK barcode and Seller SKU
	_, err = pool.Exec(ctx, `
		INSERT INTO product_variants (id, product_id, is_active, barcode, seller_sku, created_at, updated_at)
		VALUES ($1, $2, true, 'ZMK-9901', 'Sku-98AbC-X', NOW(), NOW())
	`, variantID, productID)
	require.NoError(t, err)

	// Supply & Unit
	_, err = pool.Exec(ctx, `
		INSERT INTO seller_supplies (id, seller_id, status, supply_number, handoff_method, created_at, updated_at)
		VALUES ($1, $2, 'completed', 'SUP-9901', 'courier', NOW(), NOW())
	`, supplyID, sellerID)
	require.NoError(t, err)

	_, err = pool.Exec(ctx, `
		INSERT INTO seller_supply_items (id, supply_id, variant_id, expected_quantity, accepted_quantity, damaged_quantity, missing_quantity, extra_quantity, created_at, updated_at)
		VALUES ($1, $2, $3, 1, 1, 0, 0, 0, NOW(), NOW())
	`, supplyItemID, supplyID, variantID)
	require.NoError(t, err)

	_, err = pool.Exec(ctx, `
		INSERT INTO inventory_units (id, product_variant_id, unit_code, status, origin_supply_id, origin_supply_item_id, unit_index, created_at, updated_at)
		VALUES ($1, $2, 'ZMU-7K9M2X4P8R3V5W6Y', 'warehouse', $3, $4, 1, NOW(), NOW())
	`, unitID, variantID, supplyID, supplyItemID)
	require.NoError(t, err)

	// Order ORD-990001
	_, err = pool.Exec(ctx, `
		INSERT INTO orders (id, order_number, user_id, status, customer_name, customer_email, customer_phone, delivery_address, total_price_cents, currency, created_at, updated_at)
		VALUES ($1, 'ORD-990001', $2, 'delivered', 'M61A Buyer', 'm61a_customer@zamk.local', '+79990001122', 'Address 1', 15000, 'RUB', NOW(), NOW())
	`, orderID, customerID)
	require.NoError(t, err)

	// Order Fulfillment
	_, err = pool.Exec(ctx, `
		INSERT INTO order_fulfillments (id, order_id, seller_id, status, created_at, updated_at)
		VALUES ($1, $2, $3, 'delivered', NOW(), NOW())
	`, fulfillmentID, orderID, sellerID)
	require.NoError(t, err)

	// Multi-Return: 2 Returns for the same ORD-990001
	time1 := time.Now().Add(-2 * time.Hour)
	time2 := time.Now().Add(-1 * time.Hour)
	_, err = pool.Exec(ctx, `
		INSERT INTO returns (id, order_id, fulfillment_id, user_id, status, reason, created_at, updated_at)
		VALUES ($1, $2, $3, $4, 'rejected', 'm61a_test', $5, $5)
	`, return1ID, orderID, fulfillmentID, customerID, time1)
	require.NoError(t, err)

	_, err = pool.Exec(ctx, `
		INSERT INTO returns (id, order_id, fulfillment_id, user_id, status, reason, created_at, updated_at)
		VALUES ($1, $2, $3, $4, 'needs_info', 'm61a_test', $5, $5)
	`, return2ID, orderID, fulfillmentID, customerID, time2)
	require.NoError(t, err)

	// Additional products for testing partial search escaping
	prodPercentID := uuid.New()
	_, err = pool.Exec(ctx, `
		INSERT INTO products (id, title, slug, status, price_cents, average_rating, seller_id, created_at, updated_at)
		VALUES ($1, 'Special 100% Organic Silk Scarf', 'm61a-percent-scarf', 'published', 5000, 0, $2, NOW(), NOW())
	`, prodPercentID, sellerID)
	require.NoError(t, err)

	prodUnderscoreID := uuid.New()
	_, err = pool.Exec(ctx, `
		INSERT INTO products (id, title, slug, status, price_cents, average_rating, seller_id, created_at, updated_at)
		VALUES ($1, 'Model_A Premium Jacket', 'm61a-underscore-jacket', 'published', 12000, 0, $2, NOW(), NOW())
	`, prodUnderscoreID, sellerID)
	require.NoError(t, err)

	repo := search.NewRepository(pool)
	svc := search.NewService(repo)

	allPerms := search.AllowedPermissions{
		CanReadOrders:    true,
		CanReadReturns:   true,
		CanReadInventory: true,
		CanReadProducts:  true,
		CanReadUsers:     true,
	}

	t.Run("Query Validation: Short & Empty", func(t *testing.T) {
		shortQueries := []string{"", " ", "a", "  a  "}
		for _, q := range shortQueries {
			_, err := svc.GlobalSearch(ctx, q, allPerms)
			assert.Equal(t, search.ErrQueryTooShort, err, "Query %q should return ErrQueryTooShort", q)
		}
	})

	t.Run("Exact ORD & Case Normalization & Multi-Return Collisions", func(t *testing.T) {
		for _, q := range []string{"ORD-990001", "ord-990001"} {
			res, err := svc.GlobalSearch(ctx, q, allPerms)
			require.NoError(t, err)

			var orderResults []search.GlobalSearchResult
			var returnResults []search.GlobalSearchResult
			for _, r := range res {
				if r.Type == search.ResultTypeOrder {
					orderResults = append(orderResults, r)
				} else if r.Type == search.ResultTypeReturn {
					returnResults = append(returnResults, r)
				}
			}

			// Must return exactly 1 Order
			require.Len(t, orderResults, 1)
			assert.Equal(t, "ORD-990001", orderResults[0].Title)
			assert.Equal(t, "ORD-990001", orderResults[0].CanonicalIdentifier)
			assert.Equal(t, "/orders/"+orderID.String(), orderResults[0].NavigationTarget)
			assert.Contains(t, orderResults[0].Subtitle, "M61A Buyer")
			assert.Contains(t, orderResults[0].Subtitle, "m61a_customer@zamk.local")
			assert.Contains(t, orderResults[0].Subtitle, "Доставлен")
			assert.False(t, uuidRegex.MatchString(orderResults[0].Title), "Title must not be a raw UUID")
			assert.False(t, uuidRegex.MatchString(orderResults[0].CanonicalIdentifier), "CanonicalIdentifier must not be a raw UUID")

			// Must return exactly 2 distinct Returns
			require.Len(t, returnResults, 2, "Both returns for the order must be returned distinctly")
			assert.NotEqual(t, returnResults[0].ID, returnResults[1].ID, "Returns must have distinct result IDs")
			assert.Equal(t, "ORD-990001", returnResults[0].Title)
			assert.Equal(t, "ORD-990001", returnResults[1].Title)
			assert.Equal(t, "ORD-990001", returnResults[0].CanonicalIdentifier)
			assert.Equal(t, "ORD-990001", returnResults[1].CanonicalIdentifier)
			assert.Equal(t, "/returns", returnResults[0].NavigationTarget)
			assert.Equal(t, "/returns", returnResults[1].NavigationTarget)

			// Check human status differentiation in subtitles
			// return2 (needs_info) was created later than return1 (rejected) . appears first
			assert.Contains(t, returnResults[0].Subtitle, "Ожидает ответа")
			assert.Contains(t, returnResults[1].Subtitle, "Отклонен")
			assert.False(t, uuidRegex.MatchString(returnResults[0].Title), "Title must not be a raw UUID")
			assert.False(t, uuidRegex.MatchString(returnResults[0].CanonicalIdentifier), "CanonicalIdentifier must not be a raw UUID")
		}
	})

	t.Run("Exact ZMU & Canonical Alphabet Validation", func(t *testing.T) {
		zmuFixture := "ZMU-7K9M2X4P8R3V5W6Y"
		// Programmatically verify canonical ZMU format
		require.True(t, strings.HasPrefix(zmuFixture, "ZMU-"), "ZMU must start with ZMU-")
		suffix := strings.TrimPrefix(zmuFixture, "ZMU-")
		assert.Len(t, suffix, 16, "ZMU suffix must be exactly 16 characters")
		const canonicalAlphabet = "23456789ABCDEFGHJKMNPQRSTUVWXYZ"
		for _, c := range suffix {
			assert.True(t, strings.ContainsRune(canonicalAlphabet, c), "Character %c must belong to canonical Crockford Base32 alphabet (no 0, 1, I, L, O, U)", c)
		}

		for _, q := range []string{zmuFixture, strings.ToLower(zmuFixture)} {
			res, err := svc.GlobalSearch(ctx, q, allPerms)
			require.NoError(t, err)
			require.NotEmpty(t, res)

			var unitRes *search.GlobalSearchResult
			for _, r := range res {
				if r.Type == search.ResultTypeInventoryUnit {
					unitRes = &r
					break
				}
			}
			require.NotNil(t, unitRes, "Should find inventory unit")
			assert.Equal(t, zmuFixture, unitRes.Title, "Stored canonical uppercase ZMU must be returned regardless of query case")
			assert.Equal(t, zmuFixture, unitRes.CanonicalIdentifier)
			assert.Equal(t, "/inventory", unitRes.NavigationTarget)
			assert.Contains(t, unitRes.Subtitle, "Wool Coat")
			assert.Contains(t, unitRes.Subtitle, "На складе")
			assert.False(t, uuidRegex.MatchString(unitRes.CanonicalIdentifier), "No raw UUID leakage")
		}
	})

	t.Run("Exact ZMK Barcode vs Mixed-Case Seller SKU Resolution", func(t *testing.T) {
		// 1. Searching by Barcode (ZMK) -> CanonicalIdentifier must be ZMK barcode
		resZMK, err := svc.GlobalSearch(ctx, "ZMK-9901", allPerms)
		require.NoError(t, err)
		require.NotEmpty(t, resZMK)
		assert.Equal(t, search.ResultTypeProductVariant, resZMK[0].Type)
		assert.Equal(t, "ZMK-9901", resZMK[0].Title)
		assert.Equal(t, "ZMK-9901", resZMK[0].CanonicalIdentifier)
		assert.Equal(t, "/products/"+productID.String(), resZMK[0].NavigationTarget)
		assert.Contains(t, resZMK[0].Subtitle, "Wool Coat")
		assert.Contains(t, resZMK[0].Subtitle, "Sku-98AbC-X")

		// 2. Searching by lowercase Seller SKU -> must match mixed-case persisted Sku-98AbC-X and preserve stored casing
		resSKULower, err := svc.GlobalSearch(ctx, "sku-98abc-x", allPerms)
		require.NoError(t, err)
		require.NotEmpty(t, resSKULower)
		assert.Equal(t, search.ResultTypeProductVariant, resSKULower[0].Type)
		assert.Equal(t, "Sku-98AbC-X", resSKULower[0].Title, "When searching by lowercase Seller SKU, title must preserve stored mixed-case SKU")
		assert.Equal(t, "Sku-98AbC-X", resSKULower[0].CanonicalIdentifier, "When searching by lowercase Seller SKU, canonical ID must preserve stored mixed-case SKU")
		assert.Equal(t, "/products/"+productID.String(), resSKULower[0].NavigationTarget)
		assert.Contains(t, resSKULower[0].Subtitle, "Wool Coat")
		assert.Contains(t, resSKULower[0].Subtitle, "ZMK-9901")

		// 3. Searching by exact mixed-case Seller SKU -> same result
		resSKUExact, err := svc.GlobalSearch(ctx, "Sku-98AbC-X", allPerms)
		require.NoError(t, err)
		require.NotEmpty(t, resSKUExact)
		assert.Equal(t, "Sku-98AbC-X", resSKUExact[0].Title)
		assert.Equal(t, "Sku-98AbC-X", resSKUExact[0].CanonicalIdentifier)

		// 4. Lowercase ZMK search
		resZMKLower, err := svc.GlobalSearch(ctx, "zmk-9901", allPerms)
		require.NoError(t, err)
		require.NotEmpty(t, resZMKLower)
		assert.Equal(t, "ZMK-9901", resZMKLower[0].Title)
	})

	t.Run("Customer Email: Exact, Case-Insensitive & Partial", func(t *testing.T) {
		// Exact & Case-Insensitive
		for _, emailQ := range []string{"m61a_customer@zamk.local", "M61A_CUSTOMER@ZAMK.LOCAL"} {
			res, err := svc.GlobalSearch(ctx, emailQ, allPerms)
			require.NoError(t, err)
			require.NotEmpty(t, res)

			var custRes *search.GlobalSearchResult
			for _, r := range res {
				if r.Type == search.ResultTypeCustomer {
					custRes = &r
					break
				}
			}
			require.NotNil(t, custRes)
			assert.Equal(t, "M61A Customer", custRes.Title)
			assert.Equal(t, "m61a_customer@zamk.local", custRes.Subtitle)
			assert.Equal(t, "m61a_customer@zamk.local", custRes.CanonicalIdentifier)
			assert.Equal(t, "/users", custRes.NavigationTarget)
			assert.False(t, uuidRegex.MatchString(custRes.CanonicalIdentifier))
		}

		// Partial email
		resPartial, err := svc.GlobalSearch(ctx, "m61a_cust", allPerms)
		require.NoError(t, err)
		require.NotEmpty(t, resPartial)
		assert.Equal(t, search.ResultTypeCustomer, resPartial[0].Type)
		assert.Equal(t, "m61a_customer@zamk.local", resPartial[0].Subtitle)
	})

	t.Run("Partial Product Title & Navigation", func(t *testing.T) {
		res, err := svc.GlobalSearch(ctx, "Wool Coat", allPerms)
		require.NoError(t, err)
		require.NotEmpty(t, res)

		var prodRes *search.GlobalSearchResult
		for _, r := range res {
			if r.Type == search.ResultTypeProduct {
				prodRes = &r
				break
			}
		}
		require.NotNil(t, prodRes)
		assert.Equal(t, "M61A Global Search Wool Coat", prodRes.Title)
		assert.Equal(t, "m61a-wool-coat", prodRes.CanonicalIdentifier)
		assert.Equal(t, "/products/"+productID.String(), prodRes.NavigationTarget)
		assert.Contains(t, prodRes.Subtitle, "m61a-wool-coat")
		assert.Contains(t, prodRes.Subtitle, "Опубликован")
		assert.False(t, uuidRegex.MatchString(prodRes.CanonicalIdentifier), "No raw UUID leakage in canonicalIdentifier")
	})

	t.Run("Partial Search Escaping for Wildcards % and _", func(t *testing.T) {
		// Literal '%' search: query "100%" must match "Special 100% Organic Silk Scarf"
		resPercent, err := svc.GlobalSearch(ctx, "100%", allPerms)
		require.NoError(t, err)
		require.NotEmpty(t, resPercent)
		assert.Equal(t, "Special 100% Organic Silk Scarf", resPercent[0].Title)

		// Query "%%" must not return everything, only items with "%%" (which is none)
		resDoublePercent, err := svc.GlobalSearch(ctx, "%%", allPerms)
		require.NoError(t, err)
		assert.Empty(t, resDoublePercent, "%% should match nothing when no entity has %%")

		// Literal '_' search: query "Model_" must match "Model_A Premium Jacket"
		resUnderscore, err := svc.GlobalSearch(ctx, "Model_", allPerms)
		require.NoError(t, err)
		require.NotEmpty(t, resUnderscore)
		assert.Equal(t, "Model_A Premium Jacket", resUnderscore[0].Title)
	})

	t.Run("Deterministic Ranking & Deduplication & Result Cap", func(t *testing.T) {
		// Seed 25 products with same prefix to verify cap
		var seedProdIDs []uuid.UUID
		for i := 1; i <= 25; i++ {
			pID := uuid.New()
			seedProdIDs = append(seedProdIDs, pID)
			_, err = pool.Exec(ctx, `
				INSERT INTO products (id, title, slug, status, price_cents, average_rating, seller_id, created_at, updated_at)
				VALUES ($1, $2, $3, 'published', 1000, 0, $4, NOW(), NOW())
			`, pID, fmt.Sprintf("M61A Cap Test Product %02d", i), fmt.Sprintf("m61a-cap-%02d", i), sellerID)
			require.NoError(t, err)
		}
		defer func() {
			for _, id := range seedProdIDs {
				_, _ = pool.Exec(ctx, `DELETE FROM products WHERE id = $1`, id)
			}
		}()

		// Cap test
		resCap, err := svc.GlobalSearch(ctx, "Cap Test Product", allPerms)
		require.NoError(t, err)
		assert.Len(t, resCap, 20, "Results must be capped at exactly 20")

		// Deduplication test: when query matches both exact email and partial email
		resDup, err := svc.GlobalSearch(ctx, "m61a_customer@zamk.local", allPerms)
		require.NoError(t, err)
		seenMap := make(map[string]bool)
		for _, r := range resDup {
			key := string(r.Type) + ":" + r.ID
			assert.False(t, seenMap[key], "Duplicate item detected: %s", key)
			seenMap[key] = true
		}
	})

	t.Run("RBAC: Permission-aware result filtering", func(t *testing.T) {
		query := "ORD-990001"

		// 1. User with ONLY orders.read: sees Order, but NOT Returns
		onlyOrders := search.AllowedPermissions{CanReadOrders: true}
		resOrders, err := svc.GlobalSearch(ctx, query, onlyOrders)
		require.NoError(t, err)
		require.Len(t, resOrders, 1)
		assert.Equal(t, search.ResultTypeOrder, resOrders[0].Type)

		// 2. User with ONLY returns.read: sees Returns, but NOT Order
		onlyReturns := search.AllowedPermissions{CanReadReturns: true}
		resReturns, err := svc.GlobalSearch(ctx, query, onlyReturns)
		require.NoError(t, err)
		require.Len(t, resReturns, 2)
		assert.Equal(t, search.ResultTypeReturn, resReturns[0].Type)
		assert.Equal(t, search.ResultTypeReturn, resReturns[1].Type)

		// 3. User with ONLY inventory.read: searching ZMU returns unit, searching ORD returns empty
		onlyInventory := search.AllowedPermissions{CanReadInventory: true}
		resInv, err := svc.GlobalSearch(ctx, "ZMU-7K9M2X4P8R3V5W6Y", onlyInventory)
		require.NoError(t, err)
		require.Len(t, resInv, 1)
		assert.Equal(t, search.ResultTypeInventoryUnit, resInv[0].Type)

		resInvEmpty, err := svc.GlobalSearch(ctx, query, onlyInventory)
		require.NoError(t, err)
		assert.Empty(t, resInvEmpty)

		// 4. User with ONLY products.read: searching Wool Coat returns Product & Variant, searching ORD returns empty
		onlyProducts := search.AllowedPermissions{CanReadProducts: true}
		resProd, err := svc.GlobalSearch(ctx, "ZMK-9901", onlyProducts)
		require.NoError(t, err)
		require.Len(t, resProd, 1)
		assert.Equal(t, search.ResultTypeProductVariant, resProd[0].Type)

		// 5. User with ONLY users.read: searching customer email returns Customer, searching ORD returns empty
		onlyUsers := search.AllowedPermissions{CanReadUsers: true}
		resUser, err := svc.GlobalSearch(ctx, "m61a_customer@zamk.local", onlyUsers)
		require.NoError(t, err)
		require.Len(t, resUser, 1)
		assert.Equal(t, search.ResultTypeCustomer, resUser[0].Type)

		// 6. User with NO permissions: returns empty results immediately
		noPerms := search.AllowedPermissions{}
		resNone, err := svc.GlobalSearch(ctx, "ORD-990001", noPerms)
		require.NoError(t, err)
		assert.Empty(t, resNone)
	})

	t.Run("Navigation Targets Audit", func(t *testing.T) {
		validPrefixes := []string{"/orders/", "/returns", "/inventory", "/products/", "/users"}
		res, err := svc.GlobalSearch(ctx, "m61a", allPerms)
		require.NoError(t, err)
		for _, r := range res {
			var matched bool
			for _, prefix := range validPrefixes {
				if strings.HasPrefix(r.NavigationTarget, prefix) || r.NavigationTarget == prefix {
					matched = true
					break
				}
			}
			assert.True(t, matched, "NavigationTarget %q must match a real Admin route shape", r.NavigationTarget)
		}
	})

	t.Run("Canonical Status Formatting Matrix", func(t *testing.T) {
		orderTests := map[string]string{
			"created":          "Создан",
			"awaiting_payment": "Ожидает оплаты",
			"pending_payment":  "Ожидает оплаты",
			"paid":             "Оплачен",
			"assembling":       "В сборке",
			"processing":       "В сборке",
			"packed":           "Упакован",
			"shipped":          "Отправлен",
			"delivered":        "Доставлен",
			"cancelled":        "Отменен",
			"returned":         "Возвращен",
			"refunded":         "Возврат средств",
		}
		for st, expected := range orderTests {
			assert.Equal(t, expected, search.FormatOrderStatus(st), "Order status %s must format properly", st)
		}

		returnTests := map[string]string{
			"requested":     "Новая заявка",
			"needs_info":    "Ожидает ответа",
			"approved":      "Одобрен",
			"receiving":     "Приемка",
			"item_received": "Товар принят",
			"rejected":      "Отклонен",
			"refunded":      "Возврат средств",
			"completed":     "Завершен",
			"cancelled":     "Отменен",
		}
		for st, expected := range returnTests {
			assert.Equal(t, expected, search.FormatReturnStatus(st), "Return status %s must format properly", st)
		}

		unitTests := map[string]string{
			"warehouse":   "На складе",
			"expected":    "Ожидается",
			"damaged":     "Поврежден",
			"written_off": "Списан",
			"shipped":     "Отгружен",
		}
		for st, expected := range unitTests {
			assert.Equal(t, expected, search.FormatUnitStatus(st), "Unit status %s must format properly", st)
		}

		productTests := map[string]string{
			"draft":              "Черновик",
			"pending_moderation": "На модерации",
			"approved":           "Одобрен",
			"rejected":           "Отклонен",
			"published":          "Опубликован",
			"hidden":             "Скрыт",
			"blocked":            "Заблокирован",
			"archived":           "В архиве",
		}
		for st, expected := range productTests {
			assert.Equal(t, expected, search.FormatProductStatus(st), "Product status %s must format properly", st)
		}
	})

}
