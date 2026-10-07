package behavior_test

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/eirfefrggrvgrgrtbg/shop-zamk/backend/internal/auth"
	"github.com/eirfefrggrvgrgrtbg/shop-zamk/backend/internal/behavior"
	"github.com/eirfefrggrvgrgrtbg/shop-zamk/backend/internal/http/middleware"
	"github.com/eirfefrggrvgrgrtbg/shop-zamk/backend/internal/platform/postgres"
	"github.com/eirfefrggrvgrgrtbg/shop-zamk/backend/internal/platform/ratelimit"
	"github.com/eirfefrggrvgrgrtbg/shop-zamk/backend/internal/testutil"
	"github.com/eirfefrggrvgrgrtbg/shop-zamk/backend/internal/users"
	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	goredis "github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type mockSessionValidator struct {
	sessions map[string]uuid.UUID
}

func (m *mockSessionValidator) ValidateSessionToken(ctx context.Context, rawRefreshToken string) (uuid.UUID, string, string, error) {
	if uid, ok := m.sessions[rawRefreshToken]; ok {
		return uid, "customer@example.com", "customer", nil
	}
	return uuid.Nil, "", "", auth.ErrInvalidToken
}

func setupTestRouter(t *testing.T, db *postgres.Client, validator middleware.SessionValidator) (*chi.Mux, *behavior.Repository, *auth.TokenService) {
	testutil.AssertTestDatabase(t, db.Pool)

	repo := behavior.NewRepository(db)
	svc := behavior.NewService(repo)
	handler := behavior.NewHandler(svc)

	tokenSvc := auth.NewTokenService("test_access_secret_behavior_tests_123", "test_refresh_secret", 15)

	r := chi.NewRouter()

	// Body Size Limit middleware (256 KiB)
	r.Use(func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
			req.Body = http.MaxBytesReader(w, req.Body, 256*1024)
			next.ServeHTTP(w, req)
		})
	})

	var valArg []middleware.SessionValidator
	if validator != nil {
		valArg = append(valArg, validator)
	}

	r.With(middleware.OptionalAuthMiddleware(tokenSvc, valArg...)).Post("/api/behavior/events", handler.HandleIngest)

	return r, repo, tokenSvc
}

type testFixture struct {
	CategoryID uuid.UUID
	ProductID  uuid.UUID
	VariantID  uuid.UUID
	SellerID   uuid.UUID
}

func createTestFixture(t *testing.T, db *postgres.Client) testFixture {
	testutil.AssertTestDatabase(t, db.Pool)
	ctx := context.Background()

	fix := testFixture{
		CategoryID: uuid.New(),
		ProductID:  uuid.New(),
		VariantID:  uuid.New(),
		SellerID:   uuid.New(),
	}

	// Register scoped cleanup BEFORE first INSERT
	t.Cleanup(func() {
		cleanupCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		if _, err := db.Pool.Exec(cleanupCtx, "DELETE FROM product_variants WHERE id = $1", fix.VariantID); err != nil {
			t.Logf("cleanup error product_variants: %v", err)
		}
		if _, err := db.Pool.Exec(cleanupCtx, "DELETE FROM products WHERE id = $1", fix.ProductID); err != nil {
			t.Logf("cleanup error products: %v", err)
		}
		if _, err := db.Pool.Exec(cleanupCtx, "DELETE FROM categories WHERE id = $1", fix.CategoryID); err != nil {
			t.Logf("cleanup error categories: %v", err)
		}
		if _, err := db.Pool.Exec(cleanupCtx, "DELETE FROM sellers WHERE id = $1", fix.SellerID); err != nil {
			t.Logf("cleanup error sellers: %v", err)
		}
		if _, err := db.Pool.Exec(cleanupCtx, "DELETE FROM users WHERE id = $1", fix.SellerID); err != nil {
			t.Logf("cleanup error users: %v", err)
		}
	})

	// 1. Create seller user
	sellerEmail := fmt.Sprintf("seller-%s@example.com", fix.SellerID.String()[:8])
	_, err := db.Pool.Exec(ctx, `
		INSERT INTO users (id, email, password_hash, role, status, name)
		VALUES ($1, $2, 'hash', 'seller', 'active', 'Test Seller')
	`, fix.SellerID, sellerEmail)
	require.NoError(t, err)

	// 1b. Create seller record
	sellerSlug := fmt.Sprintf("seller-slug-%s", fix.SellerID.String()[:8])
	_, err = db.Pool.Exec(ctx, `
		INSERT INTO sellers (id, brand_name, slug, status)
		VALUES ($1, 'Test Seller Brand', $2, 'active')
	`, fix.SellerID, sellerSlug)
	require.NoError(t, err)

	// 2. Create category
	catSlug := fmt.Sprintf("cat-%s", fix.CategoryID.String()[:8])
	_, err = db.Pool.Exec(ctx, `
		INSERT INTO categories (id, slug, name, parent_id)
		VALUES ($1, $2, 'Test Category', NULL)
	`, fix.CategoryID, catSlug)
	require.NoError(t, err)

	// 3. Create product
	prodSlug := fmt.Sprintf("prod-%s", fix.ProductID.String()[:8])
	_, err = db.Pool.Exec(ctx, `
		INSERT INTO products (id, title, slug, status, description, price_cents, category_id, seller_id)
		VALUES ($1, 'Test Product', $2, 'published', 'Description', 19900, $3, $4)
	`, fix.ProductID, prodSlug, fix.CategoryID, fix.SellerID)
	require.NoError(t, err)

	// 4. Create variant
	sku := fmt.Sprintf("SKU-%s", fix.VariantID.String()[:8])
	_, err = db.Pool.Exec(ctx, `
		INSERT INTO product_variants (id, product_id, size, sku)
		VALUES ($1, $2, 'L', $3)
	`, fix.VariantID, fix.ProductID, sku)
	require.NoError(t, err)

	return fix
}

func cleanupEvent(t *testing.T, db *postgres.Client, eventID uuid.UUID) {
	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if _, err := db.Pool.Exec(ctx, "DELETE FROM behavioral_events WHERE id = $1", eventID); err != nil {
			t.Logf("cleanup error behavioral_events: %v", err)
		}
	})
}

func cleanupEvents(t *testing.T, db *postgres.Client, eventIDs []uuid.UUID) {
	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		for _, id := range eventIDs {
			if _, err := db.Pool.Exec(ctx, "DELETE FROM behavioral_events WHERE id = $1", id); err != nil {
				t.Logf("cleanup error behavioral_events: %v", err)
			}
		}
	})
}

func connectTestDB(t *testing.T) *postgres.Client {
	dbURL := testutil.GetTestDatabaseURL()
	db, err := postgres.NewClient(context.Background(), dbURL)
	require.NoError(t, err)
	testutil.AssertTestDatabase(t, db.Pool)
	t.Cleanup(func() {
		db.Close()
	})
	return db
}

// 1. Anonymous valid event -> user_id NULL
func TestBehaviorIngestion_AnonymousValid(t *testing.T) {
	db := connectTestDB(t)
	fix := createTestFixture(t, db)

	r, _, _ := setupTestRouter(t, db, nil)

	eventID := uuid.New()
	cleanupEvent(t, db, eventID)

	visitorID := uuid.New()
	payload := map[string]interface{}{
		"events": []map[string]interface{}{
			{
				"eventId":    eventID.String(),
				"eventType":  "product_view",
				"visitorId":  visitorID.String(),
				"occurredAt": time.Now().UTC().Format(time.RFC3339),
				"productId":  fix.ProductID.String(),
			},
		},
	}

	body, _ := json.Marshal(payload)
	req := httptest.NewRequest("POST", "/api/behavior/events", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	require.Equal(t, http.StatusAccepted, w.Code)

	var storedUserID *uuid.UUID
	var source string
	err := db.Pool.QueryRow(context.Background(), "SELECT user_id, source FROM behavioral_events WHERE id = $1", eventID).Scan(&storedUserID, &source)
	require.NoError(t, err)
	require.Nil(t, storedUserID, "anonymous event must store user_id as NULL")
	require.Equal(t, "client", source)
}

// 2. Authenticated Shop-session valid event (Bearer & Cookie) -> exact canonical user_id stored
func TestBehaviorIngestion_AuthenticatedShopSession(t *testing.T) {
	db := connectTestDB(t)
	fix := createTestFixture(t, db)

	authenticatedUser := uuid.New()
	mockVal := &mockSessionValidator{
		sessions: map[string]uuid.UUID{
			"valid-shop-refresh-token": authenticatedUser,
		},
	}

	r, _, tokenSvc := setupTestRouter(t, db, mockVal)

	// Subtest 2A: Bearer token auth
	t.Run("Bearer Token Auth", func(t *testing.T) {
		eventID := uuid.New()
		cleanupEvent(t, db, eventID)

		accessToken, err := tokenSvc.GenerateAccessToken(authenticatedUser, "customer@example.com", "customer")
		require.NoError(t, err)

		payload := map[string]interface{}{
			"events": []map[string]interface{}{
				{
					"eventId":    eventID.String(),
					"eventType":  "product_view",
					"visitorId":  uuid.New().String(),
					"occurredAt": time.Now().UTC().Format(time.RFC3339),
					"productId":  fix.ProductID.String(),
				},
			},
		}

		body, _ := json.Marshal(payload)
		req := httptest.NewRequest("POST", "/api/behavior/events", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer "+accessToken)
		w := httptest.NewRecorder()

		r.ServeHTTP(w, req)

		require.Equal(t, http.StatusAccepted, w.Code)

		var storedUserID *uuid.UUID
		err = db.Pool.QueryRow(context.Background(), "SELECT user_id FROM behavioral_events WHERE id = $1", eventID).Scan(&storedUserID)
		require.NoError(t, err)
		require.NotNil(t, storedUserID)
		require.Equal(t, authenticatedUser, *storedUserID)
	})

	// Subtest 2B: zamk_shop_session cookie auth
	t.Run("zamk_shop_session Cookie Auth", func(t *testing.T) {
		eventID := uuid.New()
		cleanupEvent(t, db, eventID)

		payload := map[string]interface{}{
			"events": []map[string]interface{}{
				{
					"eventId":    eventID.String(),
					"eventType":  "product_view",
					"visitorId":  uuid.New().String(),
					"occurredAt": time.Now().UTC().Format(time.RFC3339),
					"productId":  fix.ProductID.String(),
				},
			},
		}

		body, _ := json.Marshal(payload)
		req := httptest.NewRequest("POST", "/api/behavior/events", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		req.AddCookie(&http.Cookie{
			Name:  auth.CookieShopSession,
			Value: "valid-shop-refresh-token",
		})
		w := httptest.NewRecorder()

		r.ServeHTTP(w, req)

		require.Equal(t, http.StatusAccepted, w.Code)

		var storedUserID *uuid.UUID
		err := db.Pool.QueryRow(context.Background(), "SELECT user_id FROM behavioral_events WHERE id = $1", eventID).Scan(&storedUserID)
		require.NoError(t, err)
		require.NotNil(t, storedUserID)
		require.Equal(t, authenticatedUser, *storedUserID)
	})
}

// 3. Expired / invalid auth -> user_id NULL
func TestBehaviorIngestion_InvalidOrExpiredAuth(t *testing.T) {
	db := connectTestDB(t)
	fix := createTestFixture(t, db)

	mockVal := &mockSessionValidator{
		sessions: map[string]uuid.UUID{},
	}
	r, _, _ := setupTestRouter(t, db, mockVal)

	eventID := uuid.New()
	cleanupEvent(t, db, eventID)

	payload := map[string]interface{}{
		"events": []map[string]interface{}{
			{
				"eventId":    eventID.String(),
				"eventType":  "product_view",
				"visitorId":  uuid.New().String(),
				"occurredAt": time.Now().UTC().Format(time.RFC3339),
				"productId":  fix.ProductID.String(),
			},
		},
	}

	body, _ := json.Marshal(payload)
	req := httptest.NewRequest("POST", "/api/behavior/events", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer invalid-or-expired-token")
	req.AddCookie(&http.Cookie{
		Name:  auth.CookieShopSession,
		Value: "expired-or-revoked-cookie",
	})
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	require.Equal(t, http.StatusAccepted, w.Code)

	var storedUserID *uuid.UUID
	err := db.Pool.QueryRow(context.Background(), "SELECT user_id FROM behavioral_events WHERE id = $1", eventID).Scan(&storedUserID)
	require.NoError(t, err)
	require.Nil(t, storedUserID, "expired or invalid auth must proceed anonymously with user_id NULL")
}

// 4-10. Forbidden fields rejected with HTTP 400
func TestBehaviorIngestion_ForbiddenFields(t *testing.T) {
	db := connectTestDB(t)
	fix := createTestFixture(t, db)

	r, _, _ := setupTestRouter(t, db, nil)

	forbiddenKeys := []string{
		"userId",
		"source",
		"receivedAt",
		"categoryId",
		"orderId",
		"returnId",
		"orderItemId",
	}

	for _, key := range forbiddenKeys {
		t.Run("Forbidden field: "+key, func(t *testing.T) {
			payload := map[string]interface{}{
				"events": []map[string]interface{}{
					{
						"eventId":    uuid.New().String(),
						"eventType":  "product_view",
						"visitorId":  uuid.New().String(),
						"occurredAt": time.Now().UTC().Format(time.RFC3339),
						"productId":  fix.ProductID.String(),
						key:          "forbidden-value",
					},
				},
			}

			body, _ := json.Marshal(payload)
			req := httptest.NewRequest("POST", "/api/behavior/events", bytes.NewReader(body))
			req.Header.Set("Content-Type", "application/json")
			w := httptest.NewRecorder()

			r.ServeHTTP(w, req)

			require.Equal(t, http.StatusBadRequest, w.Code, "field %s must be rejected with 400", key)
			require.Contains(t, w.Body.String(), key)
		})
	}
}

// 11-12. Server-only event -> HTTP 400, unknown event -> HTTP 400
func TestBehaviorIngestion_ServerOnlyAndUnknownEvents(t *testing.T) {
	db := connectTestDB(t)

	r, _, _ := setupTestRouter(t, db, nil)

	serverOnly := []string{"order_paid", "order_delivered", "return_requested"}
	for _, evtType := range serverOnly {
		t.Run("Server only event: "+evtType, func(t *testing.T) {
			payload := map[string]interface{}{
				"events": []map[string]interface{}{
					{
						"eventId":    uuid.New().String(),
						"eventType":  evtType,
						"visitorId":  uuid.New().String(),
						"occurredAt": time.Now().UTC().Format(time.RFC3339),
					},
				},
			}

			body, _ := json.Marshal(payload)
			req := httptest.NewRequest("POST", "/api/behavior/events", bytes.NewReader(body))
			req.Header.Set("Content-Type", "application/json")
			w := httptest.NewRecorder()

			r.ServeHTTP(w, req)

			require.Equal(t, http.StatusBadRequest, w.Code)
			require.Contains(t, w.Body.String(), "forbidden_event_type")
		})
	}

	t.Run("Unknown event type", func(t *testing.T) {
		payload := map[string]interface{}{
			"events": []map[string]interface{}{
				{
					"eventId":    uuid.New().String(),
					"eventType":  "hack_system",
					"visitorId":  uuid.New().String(),
					"occurredAt": time.Now().UTC().Format(time.RFC3339),
				},
			},
		}

		body, _ := json.Marshal(payload)
		req := httptest.NewRequest("POST", "/api/behavior/events", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()

		r.ServeHTTP(w, req)

		require.Equal(t, http.StatusBadRequest, w.Code)
		require.Contains(t, w.Body.String(), "unknown_event_type")
	})
}

// 13-15. Valid batch & duplicate ID retry
func TestBehaviorIngestion_ValidBatchAndIdempotency(t *testing.T) {
	db := connectTestDB(t)
	fix := createTestFixture(t, db)

	r, _, _ := setupTestRouter(t, db, nil)

	id1 := uuid.New()
	id2 := uuid.New()
	cleanupEvents(t, db, []uuid.UUID{id1, id2})

	visitorID := uuid.New()
	nowStr := time.Now().UTC().Format(time.RFC3339)

	payload := map[string]interface{}{
		"events": []map[string]interface{}{
			{
				"eventId":    id1.String(),
				"eventType":  "catalog_impression",
				"visitorId":  visitorID.String(),
				"occurredAt": nowStr,
				"productId":  fix.ProductID.String(),
			},
			{
				"eventId":    id2.String(),
				"eventType":  "product_view",
				"visitorId":  visitorID.String(),
				"occurredAt": nowStr,
				"productId":  fix.ProductID.String(),
			},
		},
	}

	body, _ := json.Marshal(payload)
	req := httptest.NewRequest("POST", "/api/behavior/events", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	require.Equal(t, http.StatusAccepted, w.Code)
	var resp behavior.EventIngestionResponse
	err := json.Unmarshal(w.Body.Bytes(), &resp)
	require.NoError(t, err)
	require.Equal(t, 2, resp.Accepted)
	require.Equal(t, 0, resp.Duplicates)

	// Resend exact batch -> Idempotent retry
	reqRetry := httptest.NewRequest("POST", "/api/behavior/events", bytes.NewReader(body))
	reqRetry.Header.Set("Content-Type", "application/json")
	wRetry := httptest.NewRecorder()

	r.ServeHTTP(wRetry, reqRetry)

	require.Equal(t, http.StatusAccepted, wRetry.Code)
	var respRetry behavior.EventIngestionResponse
	err = json.Unmarshal(wRetry.Body.Bytes(), &respRetry)
	require.NoError(t, err)
	require.Equal(t, 0, respRetry.Accepted)
	require.Equal(t, 2, respRetry.Duplicates)

	var count int
	err = db.Pool.QueryRow(context.Background(), "SELECT count(*) FROM behavioral_events WHERE id IN ($1, $2)", id1, id2).Scan(&count)
	require.NoError(t, err)
	require.Equal(t, 2, count, "exactly 2 rows must exist despite duplicate submission")
}

// 16. Batch > 50 -> HTTP 400
func TestBehaviorIngestion_BatchLimit(t *testing.T) {
	db := connectTestDB(t)
	fix := createTestFixture(t, db)

	r, _, _ := setupTestRouter(t, db, nil)

	events := make([]map[string]interface{}, 51)
	for i := 0; i < 51; i++ {
		events[i] = map[string]interface{}{
			"eventId":    uuid.New().String(),
			"eventType":  "product_view",
			"visitorId":  uuid.New().String(),
			"occurredAt": time.Now().UTC().Format(time.RFC3339),
			"productId":  fix.ProductID.String(),
		}
	}

	payload := map[string]interface{}{"events": events}
	body, _ := json.Marshal(payload)
	req := httptest.NewRequest("POST", "/api/behavior/events", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	require.Equal(t, http.StatusBadRequest, w.Code)
	require.Contains(t, w.Body.String(), "batch_too_large")
}

// 17. Oversized body -> HTTP 413
func TestBehaviorIngestion_OversizedBody(t *testing.T) {
	db := connectTestDB(t)

	r, _, _ := setupTestRouter(t, db, nil)

	// Create valid JSON payload > 256 KiB
	largeData := fmt.Sprintf(`{"events":[{"eventId":"%s","eventType":"product_view","visitorId":"%s","occurredAt":"%s","productId":"%s","placement":"%s"}]}`,
		uuid.New().String(), uuid.New().String(), time.Now().UTC().Format(time.RFC3339), uuid.New().String(), strings.Repeat("x", 260*1024))
	req := httptest.NewRequest("POST", "/api/behavior/events", strings.NewReader(largeData))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	require.Equal(t, http.StatusRequestEntityTooLarge, w.Code)
	require.Contains(t, w.Body.String(), "payload_too_large")
}

// 18-19. Old occurredAt & future occurredAt -> rejected per-event with 202
func TestBehaviorIngestion_TimestampBounds(t *testing.T) {
	db := connectTestDB(t)
	fix := createTestFixture(t, db)

	r, _, _ := setupTestRouter(t, db, nil)

	oldID := uuid.New()
	futureID := uuid.New()
	validID := uuid.New()
	cleanupEvents(t, db, []uuid.UUID{oldID, futureID, validID})

	payload := map[string]interface{}{
		"events": []map[string]interface{}{
			{
				"eventId":    oldID.String(),
				"eventType":  "product_view",
				"visitorId":  uuid.New().String(),
				"occurredAt": time.Now().UTC().Add(-25 * time.Hour).Format(time.RFC3339),
				"productId":  fix.ProductID.String(),
			},
			{
				"eventId":    futureID.String(),
				"eventType":  "product_view",
				"visitorId":  uuid.New().String(),
				"occurredAt": time.Now().UTC().Add(10 * time.Minute).Format(time.RFC3339),
				"productId":  fix.ProductID.String(),
			},
			{
				"eventId":    validID.String(),
				"eventType":  "product_view",
				"visitorId":  uuid.New().String(),
				"occurredAt": time.Now().UTC().Format(time.RFC3339),
				"productId":  fix.ProductID.String(),
			},
		},
	}

	body, _ := json.Marshal(payload)
	req := httptest.NewRequest("POST", "/api/behavior/events", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	require.Equal(t, http.StatusAccepted, w.Code)
	var resp behavior.EventIngestionResponse
	err := json.Unmarshal(w.Body.Bytes(), &resp)
	require.NoError(t, err)
	require.Equal(t, 1, resp.Accepted)
	require.Equal(t, 2, len(resp.Rejected))

	for _, rej := range resp.Rejected {
		require.Equal(t, "invalid_occurred_at", rej.Code)
	}
}

// 20-22. Unknown product, unknown variant, variant/product mismatch
func TestBehaviorIngestion_EntityValidationAndMismatch(t *testing.T) {
	db := connectTestDB(t)
	fix := createTestFixture(t, db)

	// Create a second product fixture
	fix2 := createTestFixture(t, db)

	r, _, _ := setupTestRouter(t, db, nil)

	unknownProdEvt := uuid.New()
	unknownVarEvt := uuid.New()
	mismatchEvt := uuid.New()
	validEvt := uuid.New()
	cleanupEvents(t, db, []uuid.UUID{unknownProdEvt, unknownVarEvt, mismatchEvt, validEvt})

	qty := 1
	payload := map[string]interface{}{
		"events": []map[string]interface{}{
			{
				"eventId":    unknownProdEvt.String(),
				"eventType":  "product_view",
				"visitorId":  uuid.New().String(),
				"occurredAt": time.Now().UTC().Format(time.RFC3339),
				"productId":  uuid.New().String(),
			},
			{
				"eventId":    unknownVarEvt.String(),
				"eventType":  "product_variant_selected",
				"visitorId":  uuid.New().String(),
				"occurredAt": time.Now().UTC().Format(time.RFC3339),
				"productId":  fix.ProductID.String(),
				"variantId":  uuid.New().String(),
			},
			{
				"eventId":    mismatchEvt.String(),
				"eventType":  "add_to_cart",
				"visitorId":  uuid.New().String(),
				"occurredAt": time.Now().UTC().Format(time.RFC3339),
				"productId":  fix.ProductID.String(),
				"variantId":  fix2.VariantID.String(), // variant belongs to product 2, but client claims product 1
				"quantity":   qty,
			},
			{
				"eventId":    validEvt.String(),
				"eventType":  "product_view",
				"visitorId":  uuid.New().String(),
				"occurredAt": time.Now().UTC().Format(time.RFC3339),
				"productId":  fix.ProductID.String(),
			},
		},
	}

	body, _ := json.Marshal(payload)
	req := httptest.NewRequest("POST", "/api/behavior/events", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	require.Equal(t, http.StatusAccepted, w.Code)
	var resp behavior.EventIngestionResponse
	err := json.Unmarshal(w.Body.Bytes(), &resp)
	require.NoError(t, err)
	require.Equal(t, 1, resp.Accepted)
	require.Equal(t, 3, len(resp.Rejected))

	rejMap := make(map[uuid.UUID]string)
	for _, rj := range resp.Rejected {
		rejMap[rj.EventID] = rj.Code
	}

	require.Equal(t, "unknown_product", rejMap[unknownProdEvt])
	require.Equal(t, "unknown_variant", rejMap[unknownVarEvt])
	require.Equal(t, "variant_product_mismatch", rejMap[mismatchEvt])

	// Prove only validEvt exists in DB
	var validExists, invalidExists bool
	db.Pool.QueryRow(context.Background(), "SELECT EXISTS(SELECT 1 FROM behavioral_events WHERE id = $1)", validEvt).Scan(&validExists)
	db.Pool.QueryRow(context.Background(), "SELECT EXISTS(SELECT 1 FROM behavioral_events WHERE id = $1)", mismatchEvt).Scan(&invalidExists)
	require.True(t, validExists)
	require.False(t, invalidExists)
}

// 23. Category snapshot: product.category_id derived onto event
func TestBehaviorIngestion_CategorySnapshot(t *testing.T) {
	db := connectTestDB(t)
	fix := createTestFixture(t, db)

	r, _, _ := setupTestRouter(t, db, nil)

	eventID := uuid.New()
	cleanupEvent(t, db, eventID)

	payload := map[string]interface{}{
		"events": []map[string]interface{}{
			{
				"eventId":    eventID.String(),
				"eventType":  "product_view",
				"visitorId":  uuid.New().String(),
				"occurredAt": time.Now().UTC().Format(time.RFC3339),
				"productId":  fix.ProductID.String(),
			},
		},
	}

	body, _ := json.Marshal(payload)
	req := httptest.NewRequest("POST", "/api/behavior/events", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	require.Equal(t, http.StatusAccepted, w.Code)

	var storedCatID *uuid.UUID
	err := db.Pool.QueryRow(context.Background(), "SELECT category_id FROM behavioral_events WHERE id = $1", eventID).Scan(&storedCatID)
	require.NoError(t, err)
	require.NotNil(t, storedCatID)
	require.Equal(t, fix.CategoryID, *storedCatID, "server must derive and snapshot category_id from canonical product")
}

// 24-25. Quantity validation: 0 or > 100 -> HTTP 400
func TestBehaviorIngestion_InvalidQuantity(t *testing.T) {
	db := connectTestDB(t)
	fix := createTestFixture(t, db)

	r, _, _ := setupTestRouter(t, db, nil)

	testQuantities := []int{0, -1, 101, 1000}
	for _, q := range testQuantities {
		t.Run(fmt.Sprintf("Invalid quantity %d", q), func(t *testing.T) {
			payload := map[string]interface{}{
				"events": []map[string]interface{}{
					{
						"eventId":    uuid.New().String(),
						"eventType":  "add_to_cart",
						"visitorId":  uuid.New().String(),
						"occurredAt": time.Now().UTC().Format(time.RFC3339),
						"productId":  fix.ProductID.String(),
						"variantId":  fix.VariantID.String(),
						"quantity":   q,
					},
				},
			}

			body, _ := json.Marshal(payload)
			req := httptest.NewRequest("POST", "/api/behavior/events", bytes.NewReader(body))
			req.Header.Set("Content-Type", "application/json")
			w := httptest.NewRecorder()

			r.ServeHTTP(w, req)

			require.Equal(t, http.StatusBadRequest, w.Code)
			require.Contains(t, w.Body.String(), "invalid_quantity")
		})
	}
}

// 26. Metadata validation: custom keys -> HTTP 400
func TestBehaviorIngestion_MetadataValidation(t *testing.T) {
	db := connectTestDB(t)
	fix := createTestFixture(t, db)

	r, _, _ := setupTestRouter(t, db, nil)

	payload := map[string]interface{}{
		"events": []map[string]interface{}{
			{
				"eventId":    uuid.New().String(),
				"eventType":  "product_view",
				"visitorId":  uuid.New().String(),
				"occurredAt": time.Now().UTC().Format(time.RFC3339),
				"productId":  fix.ProductID.String(),
				"metadata": map[string]interface{}{
					"arbitraryKey": "arbitraryValue",
				},
			},
		},
	}

	body, _ := json.Marshal(payload)
	req := httptest.NewRequest("POST", "/api/behavior/events", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	require.Equal(t, http.StatusBadRequest, w.Code)
	require.Contains(t, w.Body.String(), "invalid_metadata")
}

// 27. Absolute route rejection -> HTTP 400
func TestBehaviorIngestion_RouteValidation(t *testing.T) {
	db := connectTestDB(t)
	fix := createTestFixture(t, db)

	r, _, _ := setupTestRouter(t, db, nil)

	invalidRoutes := []string{
		"https://evil.com/phishing",
		"http://localhost:3000/orders",
		"ftp://files.example.com",
		strings.Repeat("a", 256),
	}

	for _, badRoute := range invalidRoutes {
		t.Run("Bad route: "+badRoute[:15], func(t *testing.T) {
			payload := map[string]interface{}{
				"events": []map[string]interface{}{
					{
						"eventId":    uuid.New().String(),
						"eventType":  "product_view",
						"visitorId":  uuid.New().String(),
						"occurredAt": time.Now().UTC().Format(time.RFC3339),
						"productId":  fix.ProductID.String(),
						"route":      badRoute,
					},
				},
			}

			body, _ := json.Marshal(payload)
			req := httptest.NewRequest("POST", "/api/behavior/events", bytes.NewReader(body))
			req.Header.Set("Content-Type", "application/json")
			w := httptest.NewRecorder()

			r.ServeHTTP(w, req)

			require.Equal(t, http.StatusBadRequest, w.Code)
			require.Contains(t, w.Body.String(), "invalid_route")
		})
	}

	// Valid route with query & fragment stripped
	eventID := uuid.New()
	cleanupEvent(t, db, eventID)

	cleanPayload := map[string]interface{}{
		"events": []map[string]interface{}{
			{
				"eventId":    eventID.String(),
				"eventType":  "product_view",
				"visitorId":  uuid.New().String(),
				"occurredAt": time.Now().UTC().Format(time.RFC3339),
				"productId":  fix.ProductID.String(),
				"route":      "/catalog/shoes?sort=price_asc#reviews",
			},
		},
	}
	body, _ := json.Marshal(cleanPayload)
	req := httptest.NewRequest("POST", "/api/behavior/events", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)
	require.Equal(t, http.StatusAccepted, w.Code)

	var storedRoute *string
	err := db.Pool.QueryRow(context.Background(), "SELECT route FROM behavioral_events WHERE id = $1", eventID).Scan(&storedRoute)
	require.NoError(t, err)
	require.NotNil(t, storedRoute)
	require.Equal(t, "/catalog/shoes", *storedRoute, "query string and fragment must be stripped")
}

// 28. Event-specific required entity semantics -> HTTP 400
func TestBehaviorIngestion_EventSpecificRequiredEntities(t *testing.T) {
	db := connectTestDB(t)
	fix := createTestFixture(t, db)

	r, _, _ := setupTestRouter(t, db, nil)

	testCases := []struct {
		name      string
		eventType string
		prodID    *string
		varID     *string
		qty       *int
		errCode   string
	}{
		{
			name:      "product_view without productId",
			eventType: "product_view",
			prodID:    nil,
			errCode:   "missing_product_id",
		},
		{
			name:      "catalog_impression without productId",
			eventType: "catalog_impression",
			prodID:    nil,
			errCode:   "missing_product_id",
		},
		{
			name:      "product_variant_selected without variantId",
			eventType: "product_variant_selected",
			prodID:    &[]string{fix.ProductID.String()}[0],
			varID:     nil,
			errCode:   "missing_variant_id",
		},
		{
			name:      "add_to_cart without quantity",
			eventType: "add_to_cart",
			prodID:    &[]string{fix.ProductID.String()}[0],
			varID:     &[]string{fix.VariantID.String()}[0],
			qty:       nil,
			errCode:   "missing_quantity",
		},
		{
			name:      "remove_from_cart without variantId",
			eventType: "remove_from_cart",
			prodID:    &[]string{fix.ProductID.String()}[0],
			varID:     nil,
			qty:       &[]int{1}[0],
			errCode:   "missing_variant_id",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			evt := map[string]interface{}{
				"eventId":    uuid.New().String(),
				"eventType":  tc.eventType,
				"visitorId":  uuid.New().String(),
				"occurredAt": time.Now().UTC().Format(time.RFC3339),
			}
			if tc.prodID != nil {
				evt["productId"] = *tc.prodID
			}
			if tc.varID != nil {
				evt["variantId"] = *tc.varID
			}
			if tc.qty != nil {
				evt["quantity"] = *tc.qty
			}

			payload := map[string]interface{}{
				"events": []map[string]interface{}{evt},
			}

			body, _ := json.Marshal(payload)
			req := httptest.NewRequest("POST", "/api/behavior/events", bytes.NewReader(body))
			req.Header.Set("Content-Type", "application/json")
			w := httptest.NewRecorder()

			r.ServeHTTP(w, req)

			require.Equal(t, http.StatusBadRequest, w.Code)
			require.Contains(t, w.Body.String(), tc.errCode)
		})
	}
}

// Real DB session token validation test
func TestValidateSessionToken_RealDB(t *testing.T) {
	db := connectTestDB(t)

	ctx := context.Background()
	userRepo := users.NewRepository(db.Pool)
	authRepo := auth.NewRepository(db)
	tokenSvc := auth.NewTokenService("test_access_secret", "test_refresh_secret", 15)
	authSvc := auth.NewService(authRepo, userRepo, tokenSvc, 7)

	testUserID := uuid.New()
	testUserEmail := fmt.Sprintf("cust-%s@example.com", testUserID.String()[:8])

	t.Cleanup(func() {
		cleanupCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		db.Pool.Exec(cleanupCtx, "DELETE FROM user_sessions WHERE user_id = $1", testUserID)
		db.Pool.Exec(cleanupCtx, "DELETE FROM users WHERE id = $1", testUserID)
	})

	_, err := db.Pool.Exec(ctx, `
		INSERT INTO users (id, email, password_hash, role, status, name)
		VALUES ($1, $2, 'hash', 'customer', 'active', 'Session Customer')
	`, testUserID, testUserEmail)
	require.NoError(t, err)

	rawRefresh := "test-raw-refresh-token-" + uuid.New().String()
	h := sha256.New()
	h.Write([]byte(rawRefresh))
	hash := hex.EncodeToString(h.Sum(nil))

	sessionID := uuid.New()
	_, err = db.Pool.Exec(ctx, `
		INSERT INTO user_sessions (id, user_id, refresh_token_hash, user_agent, ip_address, expires_at, created_at)
		VALUES ($1, $2, $3, 'UA', '127.0.0.1', now() + interval '7 days', now())
	`, sessionID, testUserID, hash)
	require.NoError(t, err)

	// Validate valid session
	resolvedID, email, role, err := authSvc.ValidateSessionToken(ctx, rawRefresh)
	require.NoError(t, err)
	require.Equal(t, testUserID, resolvedID)
	require.Equal(t, testUserEmail, email)
	require.Equal(t, "customer", role)

	// Validate invalid token
	_, _, _, err = authSvc.ValidateSessionToken(ctx, "non-existent-token")
	require.Error(t, err)

	// Validate revoked session
	_, err = db.Pool.Exec(ctx, "UPDATE user_sessions SET revoked_at = now() WHERE id = $1", sessionID)
	require.NoError(t, err)
	_, _, _, err = authSvc.ValidateSessionToken(ctx, rawRefresh)
	require.ErrorIs(t, err, auth.ErrSessionRevoked)
}

type fakeRateLimitRedis struct {
	counts map[string]int64
}

func (f *fakeRateLimitRedis) Incr(ctx context.Context, key string) *goredis.IntCmd {
	cmd := goredis.NewIntCmd(ctx)
	f.counts[key]++
	cmd.SetVal(f.counts[key])
	return cmd
}

func (f *fakeRateLimitRedis) Expire(ctx context.Context, key string, expiration time.Duration) *goredis.BoolCmd {
	cmd := goredis.NewBoolCmd(ctx)
	cmd.SetVal(true)
	return cmd
}

func (f *fakeRateLimitRedis) TTL(ctx context.Context, key string) *goredis.DurationCmd {
	cmd := goredis.NewDurationCmd(ctx, time.Second)
	cmd.SetVal(time.Minute)
	return cmd
}

// 14. Rate limit test: proves route wiring and rate limit behavior without external Redis
func TestBehaviorIngestion_RateLimit(t *testing.T) {
	db := connectTestDB(t)

	repo := behavior.NewRepository(db)
	svc := behavior.NewService(repo)
	handler := behavior.NewHandler(svc)

	fakeRdb := &fakeRateLimitRedis{counts: map[string]int64{}}
	limiter := ratelimit.New(fakeRdb)
	mw := ratelimit.NewMiddleware(limiter, true, false, nil)

	r := chi.NewRouter()
	r.Use(mw.Limit(ratelimit.Rule{
		Group:  "behavior_ingestion",
		Limit:  2,
		Window: time.Minute,
		Key:    ratelimit.IPKey("behavior_events"),
	}))
	r.Post("/api/behavior/events", handler.HandleIngest)

	req1 := httptest.NewRequest("POST", "/api/behavior/events", strings.NewReader(`{}`))
	req1.RemoteAddr = "192.168.1.100:1234"
	w1 := httptest.NewRecorder()
	r.ServeHTTP(w1, req1)
	// Body was {}, so handler responds with 400 (empty_batch or syntax), but passed rate limiter!
	require.NotEqual(t, http.StatusTooManyRequests, w1.Code)

	req2 := httptest.NewRequest("POST", "/api/behavior/events", strings.NewReader(`{}`))
	req2.RemoteAddr = "192.168.1.100:1234"
	w2 := httptest.NewRecorder()
	r.ServeHTTP(w2, req2)
	require.NotEqual(t, http.StatusTooManyRequests, w2.Code)

	// Third request from same IP should be blocked by rate limiter with HTTP 429
	req3 := httptest.NewRequest("POST", "/api/behavior/events", strings.NewReader(`{}`))
	req3.RemoteAddr = "192.168.1.100:1234"
	w3 := httptest.NewRecorder()
	r.ServeHTTP(w3, req3)
	require.Equal(t, http.StatusTooManyRequests, w3.Code)
	require.Contains(t, w3.Body.String(), "rate_limited")
	require.NotEmpty(t, w3.Header().Get("Retry-After"))
}
// Comprehensive ADS.1A Attribution & Session Hardening Test Suite
func TestBehaviorSessionAttribution_TrueLNDC(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}
	ctx := context.Background()
	db := connectTestDB(t)
	repo := behavior.NewRepository(db)
	service := behavior.NewService(repo)

	visitorID := uuid.New()
	sessionID := uuid.New()
	t0 := time.Now().UTC().Add(-1 * time.Hour).Truncate(time.Microsecond)

	// Step A: T0 - Google non-direct touch -> stored
	t.Run("A. Google non-direct -> stored with canonical 30-day expiry", func(t *testing.T) {
		req := behavior.EventIngestionRequest{
			Events: []behavior.IngestionEvent{
				{
					EventID:    uuid.New(),
					EventType:  "session_started",
					VisitorID:  visitorID,
					SessionID:  &sessionID,
					OccurredAt: t0,
					Metadata: map[string]interface{}{
						"utm_source":   "google",
						"utm_medium":   "cpc",
						"utm_campaign": "campaign-A",
						"referrer":     "https://google.com",
					},
				},
			},
		}

		resp, err := service.IngestEvents(ctx, nil, req)
		require.NoError(t, err)
		assert.Equal(t, 1, resp.Accepted)
		assert.Empty(t, resp.Rejected)

		var utmSource, utmCampaign string
		var capturedAt, expiresAt time.Time
		err = db.Pool.QueryRow(ctx, "SELECT utm_source, utm_campaign, attribution_captured_at, attribution_expires_at FROM analytics_sessions WHERE id = $1", sessionID).
			Scan(&utmSource, &utmCampaign, &capturedAt, &expiresAt)
		require.NoError(t, err)
		assert.Equal(t, "google", utmSource)
		assert.Equal(t, "campaign-A", utmCampaign)
		assert.Equal(t, t0.Unix(), capturedAt.Unix(), "captured_at must be touch time")
		expectedExpiry := behavior.CalculateAttributionExpiry(t0)
		assert.Equal(t, expectedExpiry.Unix(), expiresAt.Unix(), "expiry must be touch time + 30 days")
	})

	// Step B: T0 + 5m - Direct navigation -> Google preserved
	t.Run("B. direct -> Google preserved", func(t *testing.T) {
		t5m := t0.Add(5 * time.Minute)
		req := behavior.EventIngestionRequest{
			Events: []behavior.IngestionEvent{
				{
					EventID:    uuid.New(),
					EventType:  "page_view",
					VisitorID:  visitorID,
					SessionID:  &sessionID,
					OccurredAt: t5m,
				},
			},
		}

		resp, err := service.IngestEvents(ctx, nil, req)
		require.NoError(t, err)
		assert.Equal(t, 1, resp.Accepted)

		var utmSource, utmCampaign string
		var lastSeen, capturedAt time.Time
		err = db.Pool.QueryRow(ctx, "SELECT utm_source, utm_campaign, last_seen_at, attribution_captured_at FROM analytics_sessions WHERE id = $1", sessionID).
			Scan(&utmSource, &utmCampaign, &lastSeen, &capturedAt)
		require.NoError(t, err)
		assert.Equal(t, "google", utmSource, "direct touch must not overwrite existing non-direct utm_source")
		assert.Equal(t, "campaign-A", utmCampaign)
		assert.Equal(t, t0.Unix(), capturedAt.Unix(), "captured_at must remain original touch time")
		assert.Equal(t, t5m.Unix(), lastSeen.Unix(), "last_seen_at must be updated")
	})

	// Step C: T0 + 10m - VK non-direct touch -> VK replaces Google
	t.Run("C. VK non-direct -> VK replaces Google", func(t *testing.T) {
		t10m := t0.Add(10 * time.Minute)
		req := behavior.EventIngestionRequest{
			Events: []behavior.IngestionEvent{
				{
					EventID:    uuid.New(),
					EventType:  "page_view",
					VisitorID:  visitorID,
					SessionID:  &sessionID,
					OccurredAt: t10m,
					Metadata: map[string]interface{}{
						"utm_source":   "vk",
						"utm_medium":   "paid_social",
						"utm_campaign": "campaign-B",
					},
				},
			},
		}

		resp, err := service.IngestEvents(ctx, nil, req)
		require.NoError(t, err)
		assert.Equal(t, 1, resp.Accepted)

		var utmSource, utmMedium, utmCampaign string
		var capturedAt, expiresAt time.Time
		err = db.Pool.QueryRow(ctx, "SELECT utm_source, utm_medium, utm_campaign, attribution_captured_at, attribution_expires_at FROM analytics_sessions WHERE id = $1", sessionID).
			Scan(&utmSource, &utmMedium, &utmCampaign, &capturedAt, &expiresAt)
		require.NoError(t, err)
		assert.Equal(t, "vk", utmSource, "new non-direct touch must replace attribution source")
		assert.Equal(t, "paid_social", utmMedium)
		assert.Equal(t, "campaign-B", utmCampaign)
		assert.Equal(t, t10m.Unix(), capturedAt.Unix(), "captured_at must reset to new non-direct touch time")
		expectedExpiry := behavior.CalculateAttributionExpiry(t10m)
		assert.Equal(t, expectedExpiry.Unix(), expiresAt.Unix(), "expiry must reset to new touch time + 30 days")
	})

	// Step D: T0 + 20m - Direct navigation -> VK preserved
	t.Run("D. direct -> VK preserved", func(t *testing.T) {
		t20m := t0.Add(20 * time.Minute)
		req := behavior.EventIngestionRequest{
			Events: []behavior.IngestionEvent{
				{
					EventID:    uuid.New(),
					EventType:  "page_view",
					VisitorID:  visitorID,
					SessionID:  &sessionID,
					OccurredAt: t20m,
				},
			},
		}

		resp, err := service.IngestEvents(ctx, nil, req)
		require.NoError(t, err)
		assert.Equal(t, 1, resp.Accepted)

		var utmSource, utmCampaign string
		var lastSeen, capturedAt time.Time
		err = db.Pool.QueryRow(ctx, "SELECT utm_source, utm_campaign, last_seen_at, attribution_captured_at FROM analytics_sessions WHERE id = $1", sessionID).
			Scan(&utmSource, &utmCampaign, &lastSeen, &capturedAt)
		require.NoError(t, err)
		assert.Equal(t, "vk", utmSource, "subsequent direct touch must preserve VK attribution")
		assert.Equal(t, "campaign-B", utmCampaign)
		assert.Equal(t, t0.Add(10*time.Minute).Unix(), capturedAt.Unix())
		assert.Equal(t, t20m.Unix(), lastSeen.Unix())
	})
}

func TestBehaviorSessionAttribution_30DayExpiryAndOrderResolution(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}
	ctx := context.Background()
	db := connectTestDB(t)
	repo := behavior.NewRepository(db)
	service := behavior.NewService(repo)

	visitorID := uuid.New()
	sessionID := uuid.New()
	t0 := time.Now().UTC().Add(-35 * 24 * time.Hour) // 35 days ago

	// Seed session 35 days ago with Google attribution directly in DB
	src := "google"
	camp := "old-campaign"
	cAt := t0
	eAt := behavior.CalculateAttributionExpiry(t0)
	sess := behavior.AnalyticsSession{
		ID:                    sessionID,
		VisitorID:             visitorID,
		StartedAt:             t0,
		LastSeenAt:            t0,
		Source:                &src,
		UTMSource:             &src,
		UTMCampaign:           &camp,
		AttributionCapturedAt: &cAt,
		AttributionExpiresAt:  &eAt,
	}
	err := repo.UpsertSessions(ctx, []behavior.AnalyticsSession{sess})
	require.NoError(t, err)

	now := time.Now().UTC()

	// 1. Check active attribution within 30 days (e.g. at t0 + 20 days)
	t.Run("Active within 30 days", func(t *testing.T) {
		active, err := service.GetActiveAttribution(ctx, visitorID, t0.Add(20*24*time.Hour))
		require.NoError(t, err)
		require.NotNil(t, active)
		assert.Equal(t, "google", *active.UTMSource)
	})

	// 2. Check active attribution after 30 days (now is 35 days later) -> expired!
	t.Run("Expired after 30 days", func(t *testing.T) {
		active, err := service.GetActiveAttribution(ctx, visitorID, now)
		require.NoError(t, err)
		assert.Nil(t, active, "attribution older than 30 days must be expired")
	})

	// 3. Create order snapshot after expiry -> must NOT receive expired attribution
	t.Run("Order after expiry does not use expired attribution", func(t *testing.T) {
		orderID := uuid.New()
		_, err := db.Pool.Exec(ctx, "INSERT INTO users (id, email, password_hash, name, first_name, last_name) VALUES ($1, $2, 'hash', 'Test', 'T', 'T')", visitorID, uuid.New().String()+"@zamk.app")
		require.NoError(t, err)
		_, err = db.Pool.Exec(ctx, "INSERT INTO orders (id, user_id, status, total_price_cents, currency, customer_name, customer_phone, customer_email, delivery_address) VALUES ($1, $2, 'created', 100, 'RUB', 'T', '1', 't@e.com', 'A')", orderID, visitorID)
		require.NoError(t, err)

		snapshot, err := service.CreateOrderAttributionFromActive(ctx, orderID, visitorID, nil, now)
		require.NoError(t, err)
		require.NotNil(t, snapshot)
		assert.Nil(t, snapshot.UTMSource, "expired attribution must not be applied to new order")
		assert.Nil(t, snapshot.SessionID)
	})
}

func TestBehaviorSessionAttribution_SessionOwnershipFailClosed(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}
	ctx := context.Background()
	db := connectTestDB(t)
	repo := behavior.NewRepository(db)
	service := behavior.NewService(repo)

	visitor1 := uuid.New()
	visitor2 := uuid.New()
	sessionID := uuid.New()
	t0 := time.Now().UTC()

	// 1. Session created by visitor1
	req1 := behavior.EventIngestionRequest{
		Events: []behavior.IngestionEvent{
			{
				EventID:    uuid.New(),
				EventType:  "session_started",
				VisitorID:  visitor1,
				SessionID:  &sessionID,
				OccurredAt: t0,
			},
		},
	}
	resp1, err := service.IngestEvents(ctx, nil, req1)
	require.NoError(t, err)
	assert.Equal(t, 1, resp1.Accepted)

	// 2. Later event arrives with SAME session_id but DIFFERENT visitor2 -> MUST FAIL CLOSED
	req2 := behavior.EventIngestionRequest{
		Events: []behavior.IngestionEvent{
			{
				EventID:    uuid.New(),
				EventType:  "page_view",
				VisitorID:  visitor2,
				SessionID:  &sessionID,
				OccurredAt: t0.Add(2 * time.Minute),
			},
		},
	}
	resp2, err := service.IngestEvents(ctx, nil, req2)
	require.NoError(t, err)
	assert.Equal(t, 0, resp2.Accepted, "must not accept event for hijacked session")
	require.Len(t, resp2.Rejected, 1)
	assert.Equal(t, "session_visitor_mismatch", resp2.Rejected[0].Code)

	// Verify visitor_id in DB was NOT overwritten
	var dbVisitor uuid.UUID
	err = db.Pool.QueryRow(ctx, "SELECT visitor_id FROM analytics_sessions WHERE id = $1", sessionID).Scan(&dbVisitor)
	require.NoError(t, err)
	assert.Equal(t, visitor1, dbVisitor, "session visitor_id must remain original visitor1")
}

func TestBehaviorSessionAttribution_UserBindingCannotBeStolen(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}
	ctx := context.Background()
	db := connectTestDB(t)
	repo := behavior.NewRepository(db)
	service := behavior.NewService(repo)

	visitorID := uuid.New()
	sessionID := uuid.New()
	user1 := uuid.New()
	user2 := uuid.New()
	t0 := time.Now().UTC()

	// Insert test users
	_, err := db.Pool.Exec(ctx, "INSERT INTO users (id, email, password_hash, name, first_name, last_name) VALUES ($1, $2, 'h', 'U1', 'U', '1')", user1, uuid.New().String()+"@zamk.app")
	require.NoError(t, err)
	_, err = db.Pool.Exec(ctx, "INSERT INTO users (id, email, password_hash, name, first_name, last_name) VALUES ($1, $2, 'h', 'U2', 'U', '2')", user2, uuid.New().String()+"@zamk.app")
	require.NoError(t, err)

	// 1. Session associated with User 1
	req1 := behavior.EventIngestionRequest{
		Events: []behavior.IngestionEvent{
			{
				EventID:    uuid.New(),
				EventType:  "session_started",
				VisitorID:  visitorID,
				SessionID:  &sessionID,
				OccurredAt: t0,
			},
		},
	}
	resp1, err := service.IngestEvents(ctx, &user1, req1)
	require.NoError(t, err)
	assert.Equal(t, 1, resp1.Accepted)

	// 2. Later event arrives with User 2 credentials for SAME session -> MUST FAIL CLOSED
	req2 := behavior.EventIngestionRequest{
		Events: []behavior.IngestionEvent{
			{
				EventID:    uuid.New(),
				EventType:  "page_view",
				VisitorID:  visitorID,
				SessionID:  &sessionID,
				OccurredAt: t0.Add(3 * time.Minute),
			},
		},
	}
	resp2, err := service.IngestEvents(ctx, &user2, req2)
	require.NoError(t, err)
	assert.Equal(t, 0, resp2.Accepted, "must not accept event with conflicting user credentials")
	require.Len(t, resp2.Rejected, 1)
	assert.Equal(t, "session_user_mismatch", resp2.Rejected[0].Code)

	// Verify user_id in DB was NOT overwritten
	var dbUser uuid.UUID
	err = db.Pool.QueryRow(ctx, "SELECT user_id FROM analytics_sessions WHERE id = $1", sessionID).Scan(&dbUser)
	require.NoError(t, err)
	assert.Equal(t, user1, dbUser, "session user_id must remain user1")
}

func TestBehaviorSessionAttribution_MetadataWhitelistAndPIIRejection(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}
	ctx := context.Background()
	db := connectTestDB(t)
	repo := behavior.NewRepository(db)
	service := behavior.NewService(repo)

	visitorID := uuid.New()
	sessionID := uuid.New()
	t0 := time.Now().UTC()

	// Prohibited PII / unwhitelisted keys must be rejected with HTTP 400 invalid_metadata
	forbiddenKeys := []string{"email", "phone", "address", "user_id", "campaign_id", "secretToken", "password"}
	for _, key := range forbiddenKeys {
		t.Run("Forbidden metadata key: "+key, func(t *testing.T) {
			req := behavior.EventIngestionRequest{
				Events: []behavior.IngestionEvent{
					{
						EventID:    uuid.New(),
						EventType:  "session_started",
						VisitorID:  visitorID,
						SessionID:  &sessionID,
						OccurredAt: t0,
						Metadata: map[string]interface{}{
							key: "malicious_or_pii_value",
						},
					},
				},
			}
			_, err := service.IngestEvents(ctx, nil, req)
			require.Error(t, err)
			var structErr *behavior.StructuralError
			require.True(t, errors.As(err, &structErr))
			assert.Equal(t, "invalid_metadata", structErr.Code)
		})
	}

	// Length bounds check on metadata fields
	t.Run("Oversized metadata value rejected", func(t *testing.T) {
		req := behavior.EventIngestionRequest{
			Events: []behavior.IngestionEvent{
				{
					EventID:    uuid.New(),
					EventType:  "session_started",
					VisitorID:  visitorID,
					SessionID:  &sessionID,
					OccurredAt: t0,
					Metadata: map[string]interface{}{
						"utm_source": strings.Repeat("x", 256), // max is 255
					},
				},
			},
		}
		_, err := service.IngestEvents(ctx, nil, req)
		require.Error(t, err)
		var structErr *behavior.StructuralError
		require.True(t, errors.As(err, &structErr))
		assert.Equal(t, "invalid_metadata", structErr.Code)
	})
}

func TestBehaviorSessionAttribution_OrderSnapshotImmutability(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}
	ctx := context.Background()
	db := connectTestDB(t)
	repo := behavior.NewRepository(db)

	orderID := uuid.New()
	visitorID := uuid.New()
	sessionID := uuid.New()
	now := time.Now().UTC()

	// Seed user and order
	_, err := db.Pool.Exec(ctx, "INSERT INTO users (id, email, password_hash, name, first_name, last_name) VALUES ($1, $2, 'hash', 'Test', 'T', 'T')", visitorID, uuid.New().String()+"@zamk.app")
	require.NoError(t, err)
	_, err = db.Pool.Exec(ctx, "INSERT INTO orders (id, user_id, status, total_price_cents, currency, customer_name, customer_phone, customer_email, delivery_address) VALUES ($1, $2, 'created', 100, 'RUB', 'T', '1', 't@e.com', 'A')", orderID, visitorID)
	// Seed session so session_id FK is valid
	err = repo.UpsertSessions(ctx, []behavior.AnalyticsSession{{
		ID:         sessionID,
		VisitorID:  visitorID,
		StartedAt:  now,
		LastSeenAt: now,
	}})
	require.NoError(t, err)

	googleSource := "google"
	attr1 := behavior.OrderAttribution{
		OrderID:      orderID,
		SessionID:    &sessionID,
		VisitorID:    &visitorID,
		UTMSource:    &googleSource,
		AttributedAt: now,
	}

	// First insert persists snapshot
	err = repo.CreateOrderAttributionSnapshot(ctx, db.Pool, attr1)
	require.NoError(t, err)

	snapshot1, err := repo.GetOrderAttribution(ctx, orderID)
	require.NoError(t, err)
	require.NotNil(t, snapshot1)
	assert.Equal(t, "google", *snapshot1.UTMSource)

	// Second insert with different attribution for same order -> must NOT overwrite
	vkSource := "vk"
	attr2 := behavior.OrderAttribution{
		OrderID:      orderID,
		SessionID:    &sessionID,
		VisitorID:    &visitorID,
		UTMSource:    &vkSource,
		AttributedAt: now.Add(1 * time.Hour),
	}
	err = repo.CreateOrderAttributionSnapshot(ctx, db.Pool, attr2)
	require.NoError(t, err)

	snapshot2, err := repo.GetOrderAttribution(ctx, orderID)
	require.NoError(t, err)
	require.NotNil(t, snapshot2)
	assert.Equal(t, "google", *snapshot2.UTMSource, "second insert must not mutate original snapshot")
}

func TestBehaviorSessionAttribution_LifetimesAndHelpers(t *testing.T) {
	t0 := time.Now().UTC()

	// 30-minute inactivity timeout
	assert.Equal(t, 30*time.Minute, behavior.SessionInactivityTimeout)
	assert.False(t, behavior.IsSessionExpired(t0, t0.Add(29*time.Minute)))
	assert.True(t, behavior.IsSessionExpired(t0, t0.Add(31*time.Minute)))

	// 30-day attribution lifetime
	assert.Equal(t, 30*24*time.Hour, behavior.AttributionLifetime)
	expiry := behavior.CalculateAttributionExpiry(t0)
	assert.Equal(t, t0.Add(30*24*time.Hour), expiry)
	assert.False(t, behavior.IsAttributionExpired(&expiry, t0.Add(29*24*time.Hour)))
	assert.True(t, behavior.IsAttributionExpired(&expiry, t0.Add(31*24*time.Hour)))
	assert.True(t, behavior.IsAttributionExpired(nil, t0))
}

func TestBehaviorSessionAttribution_ServerResolvedCampaignID(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}
	ctx := context.Background()
	db := connectTestDB(t)
	repo := behavior.NewRepository(db)

	visitorID := uuid.New()
	sessionID := uuid.New()
	now := time.Now().UTC()
	campaignID := uuid.New()

	// Create test seller and marketing campaign
	sellerID := uuid.New()
	sellerSlug := fmt.Sprintf("seller-campaign-%s", sellerID.String()[:8])
	_, err := db.Pool.Exec(ctx, `
		INSERT INTO sellers (id, brand_name, slug, status)
		VALUES ($1, 'Test Seller Brand', $2, 'active')
	`, sellerID, sellerSlug)
	require.NoError(t, err)

	_, err = db.Pool.Exec(ctx, `
		INSERT INTO marketing_campaigns (id, seller_id, title, funding_mode, status, discount_type, seller_discount_bps, created_at, updated_at)
		VALUES ($1, $2, 'Summer Ad Campaign', 'seller', 'active', 'percent', 1000, now(), now())
	`, campaignID, sellerID)
	require.NoError(t, err)

	src := "yandex"
	cAt := now
	eAt := behavior.CalculateAttributionExpiry(now)
	sess := behavior.AnalyticsSession{
		ID:                    sessionID,
		VisitorID:             visitorID,
		StartedAt:             now,
		LastSeenAt:            now,
		Source:                &src,
		UTMSource:             &src,
		CampaignID:            &campaignID,
		AttributionCapturedAt: &cAt,
		AttributionExpiresAt:  &eAt,
	}

	err = repo.UpsertSessions(ctx, []behavior.AnalyticsSession{sess})
	require.NoError(t, err)

	active, err := repo.GetActiveAttribution(ctx, visitorID, now)
	require.NoError(t, err)
	require.NotNil(t, active)
	assert.Equal(t, campaignID, *active.CampaignID, "server-resolved campaign_id must be persisted and retrieved")
}

func TestBehaviorSessionAttribution_ShopIngestionStorageProof(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}
	ctx := context.Background()
	db := connectTestDB(t)
	validator := &mockSessionValidator{sessions: make(map[string]uuid.UUID)}
	router, _, tokenSvc := setupTestRouter(t, db, validator)

	visitorID := uuid.New()
	sessionID := uuid.New()
	eventID1 := uuid.New()
	eventID2 := uuid.New()
	eventID3 := uuid.New()
	now := time.Now().UTC()

	t.Cleanup(func() {
		cleanupCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		db.Pool.Exec(cleanupCtx, "DELETE FROM behavioral_events WHERE session_id = $1", sessionID)
		db.Pool.Exec(cleanupCtx, "DELETE FROM analytics_sessions WHERE id = $1", sessionID)
	})

	// 1. Anonymous Shop Landing:
	// session_started with UTM + page_view
	landingPayload := map[string]interface{}{
		"events": []map[string]interface{}{
			{
				"eventId":    eventID1.String(),
				"eventType":  "session_started",
				"visitorId":  visitorID.String(),
				"sessionId":  sessionID.String(),
				"occurredAt": now.Format(time.RFC3339Nano),
				"metadata": map[string]string{
					"utm_source":   "vk",
					"utm_medium":   "paid_social",
					"utm_campaign": "drop2",
					"landing_path": "/catalog",
				},
			},
			{
				"eventId":    eventID2.String(),
				"eventType":  "page_view",
				"visitorId":  visitorID.String(),
				"sessionId":  sessionID.String(),
				"occurredAt": now.Add(500 * time.Millisecond).Format(time.RFC3339Nano),
				"route":      "/catalog",
			},
		},
	}

	bodyBytes, err := json.Marshal(landingPayload)
	require.NoError(t, err)

	req1 := httptest.NewRequest(http.MethodPost, "/api/behavior/events", bytes.NewReader(bodyBytes))
	req1.Header.Set("Content-Type", "application/json")
	w1 := httptest.NewRecorder()
	router.ServeHTTP(w1, req1)

	require.True(t, w1.Code == http.StatusOK || w1.Code == http.StatusAccepted, "ingestion status: %d, body: %s", w1.Code, w1.Body.String())

	var resp1 behavior.EventIngestionResponse
	err = json.Unmarshal(w1.Body.Bytes(), &resp1)
	require.NoError(t, err)
	assert.Equal(t, 2, resp1.Accepted)
	assert.Empty(t, resp1.Rejected)

	// Verify analytics_sessions storage row
	var sessRow struct {
		ID                  uuid.UUID
		VisitorID           uuid.UUID
		UserID              *uuid.UUID
		UTMSource           *string
		UTMMedium           *string
		UTMCampaign         *string
		LandingPath         *string
		AttributionCaptured *time.Time
		AttributionExpires  *time.Time
	}
	err = db.Pool.QueryRow(ctx, `
		SELECT id, visitor_id, user_id, utm_source, utm_medium, utm_campaign, landing_path,
		       attribution_captured_at, attribution_expires_at
		FROM analytics_sessions
		WHERE id = $1
	`, sessionID).Scan(
		&sessRow.ID, &sessRow.VisitorID, &sessRow.UserID,
		&sessRow.UTMSource, &sessRow.UTMMedium, &sessRow.UTMCampaign, &sessRow.LandingPath,
		&sessRow.AttributionCaptured, &sessRow.AttributionExpires,
	)
	require.NoError(t, err)
	assert.Equal(t, sessionID, sessRow.ID)
	assert.Equal(t, visitorID, sessRow.VisitorID)
	assert.Nil(t, sessRow.UserID, "anonymous session user_id must be nil initially")
	require.NotNil(t, sessRow.UTMSource)
	assert.Equal(t, "vk", *sessRow.UTMSource)
	require.NotNil(t, sessRow.UTMMedium)
	assert.Equal(t, "paid_social", *sessRow.UTMMedium)
	require.NotNil(t, sessRow.UTMCampaign)
	assert.Equal(t, "drop2", *sessRow.UTMCampaign)
	require.NotNil(t, sessRow.LandingPath)
	assert.Equal(t, "/catalog", *sessRow.LandingPath)
	assert.NotNil(t, sessRow.AttributionCaptured)
	assert.NotNil(t, sessRow.AttributionExpires)

	// Verify behavioral_events rows have same session_id and visitor_id
	rows, err := db.Pool.Query(ctx, `
		SELECT id, event_type, visitor_id, session_id, user_id
		FROM behavioral_events
		WHERE session_id = $1
		ORDER BY occurred_at ASC
	`, sessionID)
	require.NoError(t, err)
	defer rows.Close()

	var eventCount int
	for rows.Next() {
		var eid uuid.UUID
		var etype string
		var vid, sid uuid.UUID
		var uid *uuid.UUID
		err := rows.Scan(&eid, &etype, &vid, &sid, &uid)
		require.NoError(t, err)
		assert.Equal(t, visitorID, vid)
		assert.Equal(t, sessionID, sid)
		assert.Nil(t, uid)
		eventCount++
	}
	assert.Equal(t, 2, eventCount)

	// 2. Authenticated event using same visitor/session:
	// Server associates user_id without changing visitor/session identity
	userID := uuid.New()
	t.Cleanup(func() {
		cleanupCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		db.Pool.Exec(cleanupCtx, "DELETE FROM users WHERE id = $1", userID)
	})

	_, err = db.Pool.Exec(ctx, `
		INSERT INTO users (id, email, password_hash, role, status, name)
		VALUES ($1, $2, 'hash', 'customer', 'active', 'Auth Customer')
	`, userID, fmt.Sprintf("cust-%s@zamk.me", userID.String()[:8]))
	require.NoError(t, err)

	accessToken, err := tokenSvc.GenerateAccessToken(userID, "customer", "active")
	require.NoError(t, err)

	authPayload := map[string]interface{}{
		"events": []map[string]interface{}{
			{
				"eventId":    eventID3.String(),
				"eventType":  "page_view",
				"visitorId":  visitorID.String(), // SAME visitor_id
				"sessionId":  sessionID.String(), // SAME session_id
				"occurredAt": now.Add(2 * time.Minute).Format(time.RFC3339Nano),
				"route":      "/account",
			},
		},
	}
	authBytes, err := json.Marshal(authPayload)
	require.NoError(t, err)

	req2 := httptest.NewRequest(http.MethodPost, "/api/behavior/events", bytes.NewReader(authBytes))
	req2.Header.Set("Content-Type", "application/json")
	req2.Header.Set("Authorization", "Bearer "+accessToken)
	w2 := httptest.NewRecorder()
	router.ServeHTTP(w2, req2)

	require.True(t, w2.Code == http.StatusOK || w2.Code == http.StatusAccepted, "auth ingestion status: %d, body: %s", w2.Code, w2.Body.String())

	// Verify server associated user_id on analytics_sessions without changing visitor_id or session_id
	var updatedSess struct {
		ID        uuid.UUID
		VisitorID uuid.UUID
		UserID    *uuid.UUID
		UTMSource *string
	}
	err = db.Pool.QueryRow(ctx, `
		SELECT id, visitor_id, user_id, utm_source
		FROM analytics_sessions
		WHERE id = $1
	`, sessionID).Scan(&updatedSess.ID, &updatedSess.VisitorID, &updatedSess.UserID, &updatedSess.UTMSource)
	require.NoError(t, err)

	assert.Equal(t, sessionID, updatedSess.ID, "session_id must remain unchanged")
	assert.Equal(t, visitorID, updatedSess.VisitorID, "visitor_id must remain unchanged")
	require.NotNil(t, updatedSess.UserID, "user_id must be associated by server")
	assert.Equal(t, userID, *updatedSess.UserID)
	require.NotNil(t, updatedSess.UTMSource)
	assert.Equal(t, "vk", *updatedSess.UTMSource, "attribution must be preserved")

	// Verify event 3 has user_id set in behavioral_events
	var evt3UserID *uuid.UUID
	err = db.Pool.QueryRow(ctx, "SELECT user_id FROM behavioral_events WHERE id = $1", eventID3).Scan(&evt3UserID)
	require.NoError(t, err)
	require.NotNil(t, evt3UserID)
	assert.Equal(t, userID, *evt3UserID)
}
