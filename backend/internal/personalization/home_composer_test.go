package personalization_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"github.com/eirfefrggrvgrgrtbg/shop-zamk/backend/internal/personalization"
	"github.com/eirfefrggrvgrgrtbg/shop-zamk/backend/internal/platform/postgres"
	"github.com/eirfefrggrvgrgrtbg/shop-zamk/backend/internal/products"
	"github.com/eirfefrggrvgrgrtbg/shop-zamk/backend/internal/testutil"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func makeDummyProduct(id uuid.UUID, title string) products.PublicProduct {
	return products.PublicProduct{
		ID:         id,
		Title:      title,
		Slug:       "slug-" + id.String()[:8],
		PriceCents: 100000,
		Currency:   "RUB",
		Status:     "published",
		SellerSlug: "seller-test",
		SellerName: "Seller Test",
		CreatedAt:  time.Now(),
	}
}

func TestHomeComposer_PERS_2C3A(t *testing.T) {
	p1 := makeDummyProduct(uuid.New(), "P1")
	p2 := makeDummyProduct(uuid.New(), "P2")
	p3 := makeDummyProduct(uuid.New(), "P3")
	p4 := makeDummyProduct(uuid.New(), "P4")
	p5 := makeDummyProduct(uuid.New(), "P5")
	p6 := makeDummyProduct(uuid.New(), "P6")

	// A. три непересекающихся source -> три блока
	t.Run("A. три непересекающихся source -> три блока", func(t *testing.T) {
		forYou := []products.PublicProduct{p1, p2}
		popular := []products.PublicProduct{p3, p4}
		newProds := []products.PublicProduct{p5, p6}

		blocks := personalization.ComposeHomeBlocks(forYou, popular, newProds, 12)
		require.Len(t, blocks, 3)

		assert.Equal(t, personalization.RecommendationBlockTypeForYou, blocks[0].Type)
		assert.Equal(t, personalization.RecommendationBlockTitleForYou, blocks[0].Title)
		require.Len(t, blocks[0].Items, 2)
		assert.Equal(t, p1.ID, blocks[0].Items[0].ID)
		assert.Equal(t, p2.ID, blocks[0].Items[1].ID)

		assert.Equal(t, personalization.RecommendationBlockTypePopular, blocks[1].Type)
		assert.Equal(t, personalization.RecommendationBlockTitlePopular, blocks[1].Title)
		require.Len(t, blocks[1].Items, 2)
		assert.Equal(t, p3.ID, blocks[1].Items[0].ID)
		assert.Equal(t, p4.ID, blocks[1].Items[1].ID)

		assert.Equal(t, personalization.RecommendationBlockTypeNew, blocks[2].Type)
		assert.Equal(t, personalization.RecommendationBlockTitleNew, blocks[2].Title)
		require.Len(t, blocks[2].Items, 2)
		assert.Equal(t, p5.ID, blocks[2].Items[0].ID)
		assert.Equal(t, p6.ID, blocks[2].Items[1].ID)
	})

	// B. product в For You + Popular -> остаётся только в For You
	t.Run("B. product в For You + Popular -> остаётся только в For You", func(t *testing.T) {
		shared := makeDummyProduct(uuid.New(), "Shared ForYou Popular")
		forYou := []products.PublicProduct{shared, p1}
		popular := []products.PublicProduct{shared, p2}
		newProds := []products.PublicProduct{p3}

		blocks := personalization.ComposeHomeBlocks(forYou, popular, newProds, 12)
		require.Len(t, blocks, 3)

		// For You has shared
		var foundInForYou bool
		for _, item := range blocks[0].Items {
			if item.ID == shared.ID {
				foundInForYou = true
			}
		}
		assert.True(t, foundInForYou, "shared product must be present in For You")

		// Popular does NOT have shared
		for _, item := range blocks[1].Items {
			assert.NotEqual(t, shared.ID, item.ID, "shared product must be excluded from Popular")
		}
		assert.Equal(t, p2.ID, blocks[1].Items[0].ID)
	})

	// C. product в Popular + New -> остаётся только в Popular
	t.Run("C. product в Popular + New -> остаётся только в Popular", func(t *testing.T) {
		shared := makeDummyProduct(uuid.New(), "Shared Popular New")
		forYou := []products.PublicProduct{p1}
		popular := []products.PublicProduct{shared, p2}
		newProds := []products.PublicProduct{shared, p3}

		blocks := personalization.ComposeHomeBlocks(forYou, popular, newProds, 12)
		require.Len(t, blocks, 3)

		// Popular has shared
		var foundInPopular bool
		for _, item := range blocks[1].Items {
			if item.ID == shared.ID {
				foundInPopular = true
			}
		}
		assert.True(t, foundInPopular, "shared product must be present in Popular")

		// New does NOT have shared
		for _, item := range blocks[2].Items {
			assert.NotEqual(t, shared.ID, item.ID, "shared product must be excluded from New")
		}
		assert.Equal(t, p3.ID, blocks[2].Items[0].ID)
	})

	// D. product во всех трёх -> только For You
	t.Run("D. product во всех трёх -> только For You", func(t *testing.T) {
		sharedAll := makeDummyProduct(uuid.New(), "Shared In All 3")
		forYou := []products.PublicProduct{sharedAll, p1}
		popular := []products.PublicProduct{sharedAll, p2}
		newProds := []products.PublicProduct{sharedAll, p3}

		blocks := personalization.ComposeHomeBlocks(forYou, popular, newProds, 12)
		require.Len(t, blocks, 3)

		// For You has sharedAll
		assert.Equal(t, sharedAll.ID, blocks[0].Items[0].ID)

		// Popular does not
		for _, item := range blocks[1].Items {
			assert.NotEqual(t, sharedAll.ID, item.ID)
		}
		// New does not
		for _, item := range blocks[2].Items {
			assert.NotEqual(t, sharedAll.ID, item.ID)
		}
	})

	// E. после dedupe Popular дозаполняется следующим candidate
	t.Run("E. после dedupe Popular дозаполняется следующим candidate", func(t *testing.T) {
		target := 2
		dup1 := makeDummyProduct(uuid.New(), "Dup1")
		dup2 := makeDummyProduct(uuid.New(), "Dup2")
		cand1 := makeDummyProduct(uuid.New(), "Cand1")
		cand2 := makeDummyProduct(uuid.New(), "Cand2")

		forYou := []products.PublicProduct{dup1, dup2}
		// Popular has dup1, dup2 first, then cand1, cand2
		popular := []products.PublicProduct{dup1, dup2, cand1, cand2}
		newProds := []products.PublicProduct{}

		blocks := personalization.ComposeHomeBlocks(forYou, popular, newProds, target)
		require.Len(t, blocks, 2)

		require.Len(t, blocks[1].Items, target, "Popular must fill up to target from subsequent candidates")
		assert.Equal(t, cand1.ID, blocks[1].Items[0].ID)
		assert.Equal(t, cand2.ID, blocks[1].Items[1].ID)
	})

	// F. после dedupe New дозаполняется следующим candidate
	t.Run("F. после dedupe New дозаполняется следующим candidate", func(t *testing.T) {
		target := 2
		dupFY := makeDummyProduct(uuid.New(), "DupFY")
		dupPop := makeDummyProduct(uuid.New(), "DupPop")
		cand1 := makeDummyProduct(uuid.New(), "NewCand1")
		cand2 := makeDummyProduct(uuid.New(), "NewCand2")

		forYou := []products.PublicProduct{dupFY}
		popular := []products.PublicProduct{dupPop}
		// New has dupFY, dupPop first, then cand1, cand2
		newProds := []products.PublicProduct{dupFY, dupPop, cand1, cand2}

		blocks := personalization.ComposeHomeBlocks(forYou, popular, newProds, target)
		require.Len(t, blocks, 3)

		require.Len(t, blocks[2].Items, target, "New block must fill up to target from subsequent candidates")
		assert.Equal(t, cand1.ID, blocks[2].Items[0].ID)
		assert.Equal(t, cand2.ID, blocks[2].Items[1].ID)
	})

	// G. порядок внутри source сохраняется
	t.Run("G. порядок внутри source сохраняется", func(t *testing.T) {
		pA := makeDummyProduct(uuid.New(), "PA")
		pB := makeDummyProduct(uuid.New(), "PB")
		pC := makeDummyProduct(uuid.New(), "PC")

		forYou := []products.PublicProduct{pA, pB, pC}
		blocks := personalization.ComposeHomeBlocks(forYou, nil, nil, 12)
		require.Len(t, blocks, 1)
		require.Len(t, blocks[0].Items, 3)
		assert.Equal(t, pA.ID, blocks[0].Items[0].ID)
		assert.Equal(t, pB.ID, blocks[0].Items[1].ID)
		assert.Equal(t, pC.ID, blocks[0].Items[2].ID)
	})

	// H. порядок blocks всегда For You -> Popular -> New
	t.Run("H. порядок blocks всегда For You -> Popular -> New", func(t *testing.T) {
		blocks := personalization.ComposeHomeBlocks([]products.PublicProduct{p1}, []products.PublicProduct{p2}, []products.PublicProduct{p3}, 12)
		require.Len(t, blocks, 3)
		assert.Equal(t, personalization.RecommendationBlockTypeForYou, blocks[0].Type)
		assert.Equal(t, personalization.RecommendationBlockTypePopular, blocks[1].Type)
		assert.Equal(t, personalization.RecommendationBlockTypeNew, blocks[2].Type)
	})

	// I. пустой For You -> Popular/New продолжают работать
	t.Run("I. пустой For You -> Popular/New продолжают работать", func(t *testing.T) {
		blocks := personalization.ComposeHomeBlocks(nil, []products.PublicProduct{p1}, []products.PublicProduct{p2}, 12)
		require.Len(t, blocks, 2)
		assert.Equal(t, personalization.RecommendationBlockTypePopular, blocks[0].Type)
		assert.Equal(t, personalization.RecommendationBlockTypeNew, blocks[1].Type)
	})

	// J. пустые все sources -> blocks []
	t.Run("J. пустые все sources -> blocks []", func(t *testing.T) {
		blocks := personalization.ComposeHomeBlocks(nil, nil, nil, 12)
		assert.NotNil(t, blocks)
		assert.Empty(t, blocks)

		resp := personalization.HomeRecommendationsResponse{Blocks: blocks}
		b, err := json.Marshal(resp)
		require.NoError(t, err)
		assert.JSONEq(t, `{"blocks":[]}`, string(b))
	})

	// K. один product нигде не повторяется между discovery blocks
	t.Run("K. один product нигде не повторяется между discovery blocks", func(t *testing.T) {
		// Multi-overlap complex case
		shared1 := makeDummyProduct(uuid.New(), "S1")
		shared2 := makeDummyProduct(uuid.New(), "S2")
		u1 := makeDummyProduct(uuid.New(), "U1")
		u2 := makeDummyProduct(uuid.New(), "U2")
		u3 := makeDummyProduct(uuid.New(), "U3")

		forYou := []products.PublicProduct{shared1, u1}
		popular := []products.PublicProduct{shared1, shared2, u2}
		newProds := []products.PublicProduct{shared1, shared2, u3}

		blocks := personalization.ComposeHomeBlocks(forYou, popular, newProds, 12)
		seen := make(map[uuid.UUID]string)
		for _, b := range blocks {
			for _, item := range b.Items {
				_, exists := seen[item.ID]
				assert.False(t, exists, "Product %s must not appear in multiple blocks (previously in %s, now in %s)", item.ID, seen[item.ID], b.Type)
				seen[item.ID] = b.Type
			}
		}
	})

	// L. endpoint требует customer auth
	t.Run("L. endpoint требует customer auth", func(t *testing.T) {
		handler := personalization.NewHandler(personalization.NewService(nil))

		// 1. Missing user context
		reqNoAuth := httptest.NewRequest("GET", "/api/customer/recommendations/home", nil)
		recNoAuth := httptest.NewRecorder()
		handler.GetHomeRecommendations(recNoAuth, reqNoAuth)
		assert.Equal(t, http.StatusUnauthorized, recNoAuth.Code)

		// 2. Invalid user context type
		reqBadCtx := httptest.NewRequest("GET", "/api/customer/recommendations/home", nil)
		reqBadCtx = reqBadCtx.WithContext(context.WithValue(reqBadCtx.Context(), "userID", "not-a-uuid"))
		recBadCtx := httptest.NewRecorder()
		handler.GetHomeRecommendations(recBadCtx, reqBadCtx)
		assert.Equal(t, http.StatusUnauthorized, recBadCtx.Code)
	})

	// M. response PublicProduct contract unchanged
	t.Run("M. response PublicProduct contract unchanged", func(t *testing.T) {
		targetProd := makeDummyProduct(uuid.New(), "Contract Product")
		blocks := personalization.ComposeHomeBlocks([]products.PublicProduct{targetProd}, nil, nil, 12)
		resp := personalization.HomeRecommendationsResponse{Blocks: blocks}

		b, err := json.Marshal(resp)
		require.NoError(t, err)

		var parsed struct {
			Blocks []struct {
				Type  string                   `json:"type"`
				Title string                   `json:"title"`
				Items []products.PublicProduct `json:"items"`
			} `json:"blocks"`
		}
		err = json.Unmarshal(b, &parsed)
		require.NoError(t, err)
		require.Len(t, parsed.Blocks, 1)
		assert.Equal(t, "for_you", parsed.Blocks[0].Type)
		assert.Equal(t, "Для вас", parsed.Blocks[0].Title)
		require.Len(t, parsed.Blocks[0].Items, 1)

		item := parsed.Blocks[0].Items[0]
		assert.Equal(t, targetProd.ID, item.ID)
		assert.Equal(t, targetProd.Title, item.Title)
		assert.Equal(t, targetProd.Slug, item.Slug)
		assert.Equal(t, targetProd.PriceCents, item.PriceCents)
		assert.Equal(t, targetProd.Currency, item.Currency)
		assert.Equal(t, targetProd.Status, item.Status)
		assert.Equal(t, targetProd.SellerSlug, item.SellerSlug)
		assert.Equal(t, targetProd.SellerName, item.SellerName)
	})
}

func TestHomeComposer_PERS_2C3A_DB(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping database integration test")
	}

	testDBURL := os.Getenv("TEST_DATABASE_URL")
	if testDBURL == "" {
		testDBURL = testutil.CanonicalTestDatabaseDSN
	}

	ctx := context.Background()
	pgClient, err := postgres.NewClient(ctx, testDBURL)
	require.NoError(t, err)
	defer pgClient.Close()

	// Invariant: strictly assert zamk_test
	var currentDB string
	err = pgClient.Pool.QueryRow(ctx, "SELECT current_database()").Scan(&currentDB)
	require.NoError(t, err)
	require.Equal(t, "zamk_test", currentDB, "integration tests must strictly run against zamk_test")

	repo := personalization.NewRepository(pgClient.Pool)
	svc := personalization.NewService(repo)
	handler := personalization.NewHandler(svc)

	customerID := uuid.New()
	req := httptest.NewRequest("GET", "/api/customer/recommendations/home", nil)
	req = req.WithContext(context.WithValue(req.Context(), "userID", customerID))
	rec := httptest.NewRecorder()

	handler.GetHomeRecommendations(rec, req)
	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Equal(t, "application/json", rec.Header().Get("Content-Type"))

	var resp personalization.HomeRecommendationsResponse
	err = json.Unmarshal(rec.Body.Bytes(), &resp)
	require.NoError(t, err)

	// Verify no duplicates across blocks in the live response
	seen := make(map[uuid.UUID]string)
	for _, b := range resp.Blocks {
		assert.Contains(t, []string{
			personalization.RecommendationBlockTypeForYou,
			personalization.RecommendationBlockTypePopular,
			personalization.RecommendationBlockTypeNew,
		}, b.Type)
		assert.NotEmpty(t, b.Title)
		assert.LessOrEqual(t, len(b.Items), personalization.TargetHomeRecommendationBlockSize)
		for _, item := range b.Items {
			_, exists := seen[item.ID]
			assert.False(t, exists, "Product %s appears in multiple blocks", item.ID)
			seen[item.ID] = b.Type
		}
	}
}
