package router_test

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/google/uuid"

	"github.com/eirfefrggrvgrgrtbg/shop-zamk/backend/internal/app"
	"github.com/eirfefrggrvgrgrtbg/shop-zamk/backend/internal/auth"
	"github.com/eirfefrggrvgrgrtbg/shop-zamk/backend/internal/config"
	"github.com/eirfefrggrvgrgrtbg/shop-zamk/backend/internal/platform/postgres"
	"github.com/eirfefrggrvgrgrtbg/shop-zamk/backend/internal/platform/redis"
	"github.com/eirfefrggrvgrgrtbg/shop-zamk/backend/internal/supplies"
)

const testDBURL = "postgres://zamk:zamk_password@localhost:5433/zamk_test?sslmode=disable"

func TestSuppliesAuth(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test")
	}

	ctx := context.Background()
	cfg := &config.Config{
		JWT: config.JWTConfig{
			AccessTokenSecret:     "test-secret",
			RefreshTokenSecret:    "test-secret-refresh",
			AccessTokenTTLMinutes: 60,
			RefreshTokenTTLDays:   7,
		},
		Auth: config.AuthConfig{},
		App:  config.AppConfig{Env: "test"},
	}
	pgClient, err := postgres.NewClient(ctx, testDBURL)
	if err != nil {
		t.Fatalf("failed to connect to db: %v", err)
	}
	defer pgClient.Close()

	redisClient, err := redis.NewClient(ctx, "localhost:6379", "", 0)
	if err != nil {
		t.Fatalf("failed to connect to redis: %v", err)
	}
	defer redisClient.Close()

	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	r, cancel := app.BuildRouter(ctx, cfg, pgClient, redisClient, logger)
	defer cancel()

	tokenService := auth.NewTokenService("test-secret", "test-secret-refresh", 60)

	// ---- Fixture helpers -----------------------------------------------

	insertUser := func(t *testing.T, role string) uuid.UUID {
		t.Helper()
		id := uuid.New()
		phone := "7999" + id.String()[:7]
		_, err := pgClient.Pool.Exec(ctx, `
			INSERT INTO users (id, email, phone, name, password_hash, role, status, created_at, updated_at)
			VALUES ($1, $2, $3, 'Test', 'hash', $4, 'active', NOW(), NOW())
		`, id, id.String()+"@test.com", phone, role)
		if err != nil {
			t.Fatalf("insertUser: %v", err)
		}
		return id
	}

	insertSeller := func(t *testing.T, ownerUserID uuid.UUID) uuid.UUID {
		t.Helper()
		sid := uuid.New()
		slug := fmt.Sprintf("test-slug-%s", sid.String()[:8])
		_, err := pgClient.Pool.Exec(ctx, `
			INSERT INTO sellers (id, brand_name, slug, contact_email, status, created_at, updated_at)
			VALUES ($1, 'Test Brand', $2, $3, 'active', NOW(), NOW())
		`, sid, slug, sid.String()+"@seller.com")
		if err != nil {
			t.Fatalf("insertSeller: %v", err)
		}
		_, err = pgClient.Pool.Exec(ctx, `
			INSERT INTO seller_users (id, seller_id, user_id, role, created_at)
			VALUES ($1, $2, $3, 'owner', NOW())
		`, uuid.New(), sid, ownerUserID)
		if err != nil {
			t.Fatalf("insertSellerUser: %v", err)
		}
		return sid
	}

	insertAdminWithPerms := func(t *testing.T, userID uuid.UUID, perms []string) {
		t.Helper()
		roleID := uuid.New()
		code := roleID.String()[:8]
		_, err := pgClient.Pool.Exec(ctx, `INSERT INTO staff_roles (id, code, name) VALUES ($1, $2, 'TestRole')`, roleID, code)
		if err != nil {
			t.Fatalf("insertAdminRole: %v", err)
		}
		for _, p := range perms {
			_, err = pgClient.Pool.Exec(ctx, `INSERT INTO staff_role_permissions (role_id, permission) VALUES ($1, $2)`, roleID, p)
			if err != nil {
				t.Fatalf("insertPerm %s: %v", p, err)
			}
		}
		_, err = pgClient.Pool.Exec(ctx, `INSERT INTO staff_members (user_id, staff_role_id, status) VALUES ($1, $2, 'active')`, userID, roleID)
		if err != nil {
			t.Fatalf("insertStaffMember: %v", err)
		}
	}

	makeToken := func(t *testing.T, userID uuid.UUID, role string) string {
		t.Helper()
		tok, err := tokenService.GenerateAccessToken(userID, userID.String()+"@test.com", role)
		if err != nil {
			t.Fatalf("makeToken: %v", err)
		}
		return tok
	}

	insertProductAndVariant := func(t *testing.T, sellerID uuid.UUID) (uuid.UUID, uuid.UUID) {
		t.Helper()
		productID := uuid.New()
		_, err := pgClient.Pool.Exec(ctx, `
			INSERT INTO products (id, seller_id, title, slug, price_cents, status, created_at, updated_at)
			VALUES ($1, $2, 'Auth Test Product', $3, 100, 'published', NOW(), NOW())
		`, productID, sellerID, "auth-test-product-"+productID.String()[:8])
		if err != nil {
			t.Fatalf("insert product: %v", err)
		}
		variantID := uuid.New()
		_, err = pgClient.Pool.Exec(ctx, `
			INSERT INTO product_variants (id, product_id, sku, seller_sku, barcode, price_cents, created_at, updated_at)
			VALUES ($1, $2, $3, $4, $5, 100, NOW(), NOW())
		`, variantID, productID, "AUTH-SKU-"+variantID.String()[:8], "SKU-"+variantID.String()[:8], "ZMK-"+variantID.String()[:8])
		if err != nil {
			t.Fatalf("insert variant: %v", err)
		}
		_, err = pgClient.Pool.Exec(ctx, `
			INSERT INTO inventory_items (id, product_id, product_variant_id, seller_id, total_stock)
			VALUES ($1, $2, $3, $4, 0)
		`, uuid.New(), productID, variantID, sellerID)
		if err != nil {
			t.Fatalf("insert inventory: %v", err)
		}
		return productID, variantID
	}

	// Create a shipped supply for use in cases H and I
	// We need a full supply fixture: seller, product, variant, supply
	setupShippedSupplyFixture := func(t *testing.T) (sellerUserID uuid.UUID, sellerID uuid.UUID, qrToken string, sessionID uuid.UUID) {
		t.Helper()

		// Seller user + seller
		sellerUserID = insertUser(t, "seller")
		sellerID = insertSeller(t, sellerUserID)

		// Product + variant + inventory
		_, variantID := insertProductAndVariant(t, sellerID)

		// Create supply via service
		repo := supplies.NewRepository(pgClient.Pool)
		svc := supplies.NewService(pgClient.Pool, repo)
		carrier := "СДЭК"
		tracking := "121212123241"
		req := supplies.CreateSupplyRequest{
			HandoffMethod:  "carrier_delivery",
			CarrierName:    &carrier,
			TrackingNumber: &tracking,
			Items:          []supplies.CreateSupplyItemRequest{{VariantID: variantID, ExpectedQuantity: 5}},
		}
		supply, err := svc.CreateSupply(ctx, sellerID, req)
		if err != nil {
			t.Fatalf("CreateSupply: %v", err)
		}

		if _, err = svc.MarkShipped(ctx, sellerID, supply.ID); err != nil {
			t.Fatalf("MarkShipped: %v", err)
		}

		// Reload to get QR token
		supply, err = repo.GetSupplyByID(ctx, supply.ID)
		if err != nil {
			t.Fatalf("GetSupplyByID: %v", err)
		}
		qrToken = *supply.QRToken
		return sellerUserID, sellerID, qrToken, uuid.Nil // sessionID set after StartSession in case H
	}

	// ====================================================================
	// A. Unauthenticated → 401
	// ====================================================================
	t.Run("A. unauthenticated GET seller/supplies -> 401", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/api/seller/supplies", nil)
		rr := httptest.NewRecorder()
		r.ServeHTTP(rr, req)
		if rr.Code != http.StatusUnauthorized {
			t.Errorf("expected 401, got %d body=%s", rr.Code, rr.Body.String())
		}
	})

	// ====================================================================
	// B. Customer → 403 on seller route
	// ====================================================================
	t.Run("B. customer GET seller/supplies -> 403", func(t *testing.T) {
		uid := insertUser(t, "customer")
		tok := makeToken(t, uid, "customer")
		req := httptest.NewRequest("GET", "/api/seller/supplies", nil)
		req.Header.Set("Authorization", "Bearer "+tok)
		rr := httptest.NewRecorder()
		r.ServeHTTP(rr, req)
		if rr.Code != http.StatusForbidden {
			t.Errorf("expected 403, got %d body=%s", rr.Code, rr.Body.String())
		}
	})

	// ====================================================================
	// C. Seller A GET own supply → 200
	// ====================================================================
	t.Run("C. seller GET own supply -> 200", func(t *testing.T) {
		sellerUserID, sellerID, qrToken, _ := setupShippedSupplyFixture(t)
		_ = qrToken

		// Retrieve the supply ID from DB
		var supplyID uuid.UUID
		err := pgClient.Pool.QueryRow(ctx,
			`SELECT id FROM seller_supplies WHERE seller_id = $1 ORDER BY created_at DESC LIMIT 1`, sellerID,
		).Scan(&supplyID)
		if err != nil {
			t.Fatalf("get supply id: %v", err)
		}

		tok := makeToken(t, sellerUserID, "seller")
		req := httptest.NewRequest("GET", "/api/seller/supplies/"+supplyID.String(), nil)
		req.Header.Set("Authorization", "Bearer "+tok)
		rr := httptest.NewRecorder()
		r.ServeHTTP(rr, req)
		if rr.Code != http.StatusOK {
			t.Errorf("expected 200, got %d body=%s", rr.Code, rr.Body.String())
		}
	})

	// ====================================================================
	// D. Seller A GET supply of Seller B → 403
	// ====================================================================
	t.Run("D. seller GET other seller supply -> 403", func(t *testing.T) {
		// Seller B creates the supply
		_, sellerBID, qrToken, _ := setupShippedSupplyFixture(t)
		_ = qrToken

		var supplyID uuid.UUID
		err := pgClient.Pool.QueryRow(ctx,
			`SELECT id FROM seller_supplies WHERE seller_id = $1 ORDER BY created_at DESC LIMIT 1`, sellerBID,
		).Scan(&supplyID)
		if err != nil {
			t.Fatalf("get supply id: %v", err)
		}

		// Seller A tries to read it
		sellerAUserID := insertUser(t, "seller")
		_ = insertSeller(t, sellerAUserID)
		tok := makeToken(t, sellerAUserID, "seller")

		req := httptest.NewRequest("GET", "/api/seller/supplies/"+supplyID.String(), nil)
		req.Header.Set("Authorization", "Bearer "+tok)
		rr := httptest.NewRecorder()
		r.ServeHTTP(rr, req)
		if rr.Code != http.StatusForbidden && rr.Code != http.StatusNotFound {
			t.Errorf("expected 403 or 404, got %d body=%s", rr.Code, rr.Body.String())
		}
	})

	// ====================================================================
	// E. Seller A POST receiving/start → 403 (admin-only route)
	// ====================================================================
	t.Run("E. seller POST admin/receiving/sessions -> 403", func(t *testing.T) {
		uid := insertUser(t, "seller")
		tok := makeToken(t, uid, "seller")
		req := httptest.NewRequest("POST", "/api/admin/receiving/sessions", nil)
		req.Header.Set("Authorization", "Bearer "+tok)
		rr := httptest.NewRecorder()
		r.ServeHTTP(rr, req)
		if rr.Code != http.StatusForbidden && rr.Code != http.StatusUnauthorized {
			t.Errorf("expected 403/401, got %d body=%s", rr.Code, rr.Body.String())
		}
	})

	// ====================================================================
	// F. Seller A POST receiving/finalize → 403 (admin-only route)
	// ====================================================================
	t.Run("F. seller POST admin/receiving/finalize -> 403", func(t *testing.T) {
		uid := insertUser(t, "seller")
		tok := makeToken(t, uid, "seller")
		fakeSessionID := uuid.New()
		req := httptest.NewRequest("POST", "/api/admin/receiving/sessions/"+fakeSessionID.String()+"/finalize", nil)
		req.Header.Set("Authorization", "Bearer "+tok)
		rr := httptest.NewRecorder()
		r.ServeHTTP(rr, req)
		if rr.Code != http.StatusForbidden && rr.Code != http.StatusUnauthorized {
			t.Errorf("expected 403/401, got %d body=%s", rr.Code, rr.Body.String())
		}
	})

	// ====================================================================
	// G. Admin without inventory.receipt → 403
	// ====================================================================
	t.Run("G. admin without receipt perm POST receiving/sessions -> 403", func(t *testing.T) {
		uid := insertUser(t, "admin")
		insertAdminWithPerms(t, uid, []string{"inventory.read"})
		tok := makeToken(t, uid, "admin")
		req := httptest.NewRequest("POST", "/api/admin/receiving/sessions?qr_token=anything", nil)
		req.Header.Set("Authorization", "Bearer "+tok)
		rr := httptest.NewRecorder()
		r.ServeHTTP(rr, req)
		if rr.Code != http.StatusForbidden {
			t.Errorf("expected 403, got %d body=%s", rr.Code, rr.Body.String())
		}
	})

	// ====================================================================
	// H. Admin with inventory.receipt + valid shipped supply QR → 200
	// ====================================================================
	var sessionIDForI uuid.UUID
	t.Run("H. admin with receipt perm POST receiving/sessions valid QR -> 200", func(t *testing.T) {
		_, _, qrToken, _ := setupShippedSupplyFixture(t)
			pgClient.Pool.Exec(context.Background(), "UPDATE seller_supplies SET status = 'arrived_at_zamk' WHERE qr_token = $1", qrToken)

		uid := insertUser(t, "admin")
		insertAdminWithPerms(t, uid, []string{"inventory.receipt"})
		tok := makeToken(t, uid, "admin")

		req := httptest.NewRequest("POST", "/api/admin/receiving/sessions?qr_token="+qrToken, nil)
		req.Header.Set("Authorization", "Bearer "+tok)
		rr := httptest.NewRecorder()
		r.ServeHTTP(rr, req)

		if rr.Code != http.StatusOK {
			t.Errorf("expected 200, got %d body=%s", rr.Code, rr.Body.String())
			return
		}

		// Parse session ID for case I
		var sess struct {
			ID string `json:"id"`
		}
		if err := json.NewDecoder(rr.Body).Decode(&sess); err == nil && sess.ID != "" {
			sessionIDForI, _ = uuid.Parse(sess.ID)
		}
	})

	// ====================================================================
	// I. Admin with inventory.receipt + valid sessionId → finalize → 204
	// ====================================================================
	t.Run("I. admin with receipt perm POST finalize valid session -> 204", func(t *testing.T) {
		if sessionIDForI == uuid.Nil {
			t.Skip("sessionIDForI not set (case H may have failed)")
		}
		uid := insertUser(t, "admin")
		insertAdminWithPerms(t, uid, []string{"inventory.receipt"})
		tok := makeToken(t, uid, "admin")

		body, _ := json.Marshal(map[string]interface{}{})
		req := httptest.NewRequest("POST", "/api/admin/receiving/sessions/"+sessionIDForI.String()+"/finalize", bytes.NewReader(body))
		req.Header.Set("Authorization", "Bearer "+tok)
		req.Header.Set("Content-Type", "application/json")
		rr := httptest.NewRecorder()
		r.ServeHTTP(rr, req)

		if rr.Code != http.StatusNoContent && rr.Code != http.StatusOK {
			t.Errorf("expected 204 or 200, got %d body=%s", rr.Code, rr.Body.String())
		}
	})

	// ====================================================================
	// J. Seller A GET /api/seller/supplies/{id}/unit-labels -> 200
	// ====================================================================
	t.Run("J. seller A GET unit-labels own supply -> 200", func(t *testing.T) {
		sellerUserA := insertUser(t, "seller")
		sellerA := insertSeller(t, sellerUserA)
		tokenA := makeToken(t, sellerUserA, "seller")

		_, variantA := insertProductAndVariant(t, sellerA)

		createReq := supplies.CreateSupplyRequest{
			HandoffMethod:  "carrier_delivery",
			CarrierName:    func(s string) *string { return &s }("СДЭК"),
			TrackingNumber: func(s string) *string { return &s }("TRK-ROUTER-TEST"),
			Items: []supplies.CreateSupplyItemRequest{
				{VariantID: variantA, ExpectedQuantity: 2},
			},
		}
		body, _ := json.Marshal(createReq)
		postReq := httptest.NewRequest("POST", "/api/seller/supplies", bytes.NewReader(body))
		postReq.Header.Set("Authorization", "Bearer "+tokenA)
		postReq.Header.Set("Content-Type", "application/json")
		postRR := httptest.NewRecorder()
		r.ServeHTTP(postRR, postReq)

		if postRR.Code != http.StatusCreated && postRR.Code != http.StatusOK {
			t.Fatalf("failed creating supply: %d body=%s", postRR.Code, postRR.Body.String())
		}

		var createdSupply struct {
			ID string `json:"id"`
		}
		json.NewDecoder(postRR.Body).Decode(&createdSupply)

		getReq := httptest.NewRequest("GET", "/api/seller/supplies/"+createdSupply.ID+"/unit-labels", nil)
		getReq.Header.Set("Authorization", "Bearer "+tokenA)
		getRR := httptest.NewRecorder()
		r.ServeHTTP(getRR, getReq)

		if getRR.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d body=%s", getRR.Code, getRR.Body.String())
		}

		var res supplies.SupplyUnitLabelsResponse
		if err := json.NewDecoder(getRR.Body).Decode(&res); err != nil {
			t.Fatalf("failed decoding labels response: %v", err)
		}
		if !res.Serialized || res.TotalUnits != 2 || len(res.Units) != 2 {
			t.Errorf("unexpected response structure: %+v", res)
		}

		// K. Seller B GET /api/seller/supplies/{id}/unit-labels -> 403 Forbidden
		sellerUserB := insertUser(t, "seller")
		_ = insertSeller(t, sellerUserB)
		tokenB := makeToken(t, sellerUserB, "seller")

		getReqB := httptest.NewRequest("GET", "/api/seller/supplies/"+createdSupply.ID+"/unit-labels", nil)
		getReqB.Header.Set("Authorization", "Bearer "+tokenB)
		getRRB := httptest.NewRecorder()
		r.ServeHTTP(getRRB, getReqB)

		if getRRB.Code != http.StatusForbidden {
			t.Errorf("expected 403 Forbidden for seller B, got %d body=%s", getRRB.Code, getRRB.Body.String())
		}

		// L. Unauthenticated GET /api/seller/supplies/{id}/unit-labels -> 401 Unauthorized
		getReqAnon := httptest.NewRequest("GET", "/api/seller/supplies/"+createdSupply.ID+"/unit-labels", nil)
		getRRAnon := httptest.NewRecorder()
		r.ServeHTTP(getRRAnon, getReqAnon)

		if getRRAnon.Code != http.StatusUnauthorized {
			t.Errorf("expected 401 Unauthorized, got %d body=%s", getRRAnon.Code, getRRAnon.Body.String())
		}
	})
}
