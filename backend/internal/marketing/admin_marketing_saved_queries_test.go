package marketing_test

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/eirfefrggrvgrgrtbg/shop-zamk/backend/internal/marketing"
)

func TestSavedQueries(t *testing.T) {
	client, _, service := setupTestMarketingDB(t)
	pool := client.Pool

	// Dependency-safe cleanup function
	var createdIDs []uuid.UUID
	var userIDs []uuid.UUID
	cleanup := func() {
		ctx := context.Background()
		for _, id := range createdIDs {
			_, _ = pool.Exec(ctx, "DELETE FROM marketing_saved_queries WHERE id = $1", id)
		}
		for _, id := range userIDs {
			_, _ = pool.Exec(ctx, "DELETE FROM users WHERE id = $1", id)
		}
	}
	t.Cleanup(cleanup)

	handler := marketing.NewHandler(service, nil)

	r := chi.NewRouter()
	r.Use(func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
			userIDStr := req.Header.Get("X-User-ID")
			if userIDStr != "" {
				userID, _ := uuid.Parse(userIDStr)
				ctx := context.WithValue(req.Context(), "userID", userID)
				req = req.WithContext(ctx)
			}
			next.ServeHTTP(w, req)
		})
	})
	r.Get("/saved-queries", handler.ListSavedQueries)
	r.Post("/saved-queries", handler.CreateSavedQuery)
	r.Get("/saved-queries/{id}", handler.GetSavedQuery)
	r.Patch("/saved-queries/{id}", handler.UpdateSavedQuery)
	r.Delete("/saved-queries/{id}", handler.DeleteSavedQuery)

	ts := httptest.NewServer(r)
	defer ts.Close()

	httpClient := ts.Client()

	// Create user
	userID := uuid.New()
	_, err := pool.Exec(context.Background(), `
		INSERT INTO users (id, name, email, password_hash, role, created_at, updated_at)
		VALUES ($1, 'Test Admin', $2, 'hash', 'admin', now(), now())
	`, userID, "saved_queries_test@zamk.app")
	require.NoError(t, err)
	userIDs = append(userIDs, userID)

	// Create second user
	userID2 := uuid.New()
	_, err = pool.Exec(context.Background(), `
		INSERT INTO users (id, name, email, password_hash, role, created_at, updated_at)
		VALUES ($1, 'Test Admin 2', $2, 'hash', 'admin', now(), now())
	`, userID2, "saved_queries_test2@zamk.app")
	require.NoError(t, err)
	userIDs = append(userIDs, userID2)

	validQuerySpec := marketing.QueryRequest{
		Version:    1,
		Period:     marketing.QueryPeriod{From: time.Now().Add(-24 * time.Hour), To: time.Now()},
		Dimensions: []string{"source"},
		Metrics:    []string{"sessions"},
	}
	specBytes, _ := json.Marshal(validQuerySpec)

	desc := "My description"
	createReq := marketing.SavedQueryCreateRequest{
		Name:        "Test Query",
		Description: &desc,
		QuerySpec:   specBytes,
	}
	createBytes, _ := json.Marshal(createReq)

	var savedID uuid.UUID

	t.Run("Create", func(t *testing.T) {
		req, _ := http.NewRequest(http.MethodPost, ts.URL+"/saved-queries", bytes.NewBuffer(createBytes))
		req.Header.Set("X-User-ID", userID.String())

		res, err := httpClient.Do(req)
		require.NoError(t, err)
		defer res.Body.Close()

		var bodyBytes bytes.Buffer
		bodyBytes.ReadFrom(res.Body)
		t.Log("Create Response Body:", bodyBytes.String())

		assert.Equal(t, http.StatusCreated, res.StatusCode)

		var sq marketing.SavedQuery
		require.NoError(t, json.Unmarshal(bodyBytes.Bytes(), &sq))
		assert.Equal(t, "Test Query", sq.Name)
		assert.Equal(t, "My description", *sq.Description)
		assert.Equal(t, 1, sq.QueryVersion)

		savedID = sq.ID
		createdIDs = append(createdIDs, savedID)
	})

	t.Run("Get", func(t *testing.T) {
		req, _ := http.NewRequest(http.MethodGet, ts.URL+"/saved-queries/"+savedID.String(), nil)
		req.Header.Set("X-User-ID", userID.String())

		res, err := httpClient.Do(req)
		require.NoError(t, err)
		defer res.Body.Close()

		assert.Equal(t, http.StatusOK, res.StatusCode)

		var sq marketing.SavedQuery
		require.NoError(t, json.NewDecoder(res.Body).Decode(&sq))
		assert.Equal(t, savedID, sq.ID)
	})

	t.Run("List", func(t *testing.T) {
		req, _ := http.NewRequest(http.MethodGet, ts.URL+"/saved-queries", nil)
		req.Header.Set("X-User-ID", userID.String())

		res, err := httpClient.Do(req)
		require.NoError(t, err)
		defer res.Body.Close()

		assert.Equal(t, http.StatusOK, res.StatusCode)

		var queries []marketing.SavedQuery
		require.NoError(t, json.NewDecoder(res.Body).Decode(&queries))
		assert.Len(t, queries, 1)
		assert.Equal(t, savedID, queries[0].ID)
	})

	t.Run("Update", func(t *testing.T) {
		newName := "Updated Name"
		newDesc := "Updated desc"
		updateReq := marketing.SavedQueryUpdateRequest{
			Name:        &newName,
			Description: &newDesc,
			QuerySpec:   specBytes,
		}
		updateBytes, _ := json.Marshal(updateReq)

		req, _ := http.NewRequest(http.MethodPatch, ts.URL+"/saved-queries/"+savedID.String(), bytes.NewBuffer(updateBytes))
		req.Header.Set("X-User-ID", userID.String())

		res, err := httpClient.Do(req)
		require.NoError(t, err)
		defer res.Body.Close()

		assert.Equal(t, http.StatusOK, res.StatusCode)

		var sq marketing.SavedQuery
		require.NoError(t, json.NewDecoder(res.Body).Decode(&sq))
		assert.Equal(t, "Updated Name", sq.Name)
		assert.Equal(t, "Updated desc", *sq.Description)
	})

	t.Run("List Ordering and PATCH Timestamp", func(t *testing.T) {
		userTest := uuid.New()
		_, err := pool.Exec(context.Background(), `
			INSERT INTO users (id, name, email, password_hash, role, created_at, updated_at)
			VALUES ($1, 'Ordering User', $2, 'hash', 'admin', now(), now())
		`, userTest, "ordering_test@zamk.app")
		require.NoError(t, err)
		userIDs = append(userIDs, userTest)

		t0 := time.Now().UTC().Add(-10 * time.Minute).Truncate(time.Microsecond)
		t1 := time.Now().UTC().Add(-5 * time.Minute).Truncate(time.Microsecond)

		idA := uuid.New()
		idB := uuid.New()

		// A created first at t0
		_, err = pool.Exec(context.Background(), `
			INSERT INTO marketing_saved_queries (id, name, description, query_version, query_spec, created_by, created_at, updated_at)
			VALUES ($1, 'Query A', 'Desc A', 1, $2, $3, $4, $4)
		`, idA, specBytes, userTest, t0)
		require.NoError(t, err)
		createdIDs = append(createdIDs, idA)

		// B created second at t1
		_, err = pool.Exec(context.Background(), `
			INSERT INTO marketing_saved_queries (id, name, description, query_version, query_spec, created_by, created_at, updated_at)
			VALUES ($1, 'Query B', 'Desc B', 1, $2, $3, $4, $4)
		`, idB, specBytes, userTest, t1)
		require.NoError(t, err)
		createdIDs = append(createdIDs, idB)

		// Initial list: B before A (since t1 > t0)
		req, _ := http.NewRequest(http.MethodGet, ts.URL+"/saved-queries", nil)
		req.Header.Set("X-User-ID", userTest.String())
		res, err := httpClient.Do(req)
		require.NoError(t, err)
		var list1 []marketing.SavedQuery
		require.NoError(t, json.NewDecoder(res.Body).Decode(&list1))
		res.Body.Close()
		require.Len(t, list1, 2)
		assert.Equal(t, idB, list1[0].ID, "Initial list: B before A")
		assert.Equal(t, idA, list1[1].ID)

		// Then PATCH A
		patchDesc := "Patched Desc A"
		patchReq := marketing.SavedQueryUpdateRequest{
			Description: &patchDesc,
		}
		patchBytes, _ := json.Marshal(patchReq)
		patchHTTPReq, _ := http.NewRequest(http.MethodPatch, ts.URL+"/saved-queries/"+idA.String(), bytes.NewBuffer(patchBytes))
		patchHTTPReq.Header.Set("X-User-ID", userTest.String())
		patchRes, err := httpClient.Do(patchHTTPReq)
		require.NoError(t, err)
		assert.Equal(t, http.StatusOK, patchRes.StatusCode)

		var patchedA marketing.SavedQuery
		require.NoError(t, json.NewDecoder(patchRes.Body).Decode(&patchedA))
		patchRes.Body.Close()

		// Verify created_at is unchanged and updated_at is updated
		assert.True(t, patchedA.CreatedAt.Equal(t0), "created_at must remain unchanged")
		assert.True(t, patchedA.UpdatedAt.After(t1), "updated_at must be newer than t1")

		// List after update: A must appear before B
		reqAfter, _ := http.NewRequest(http.MethodGet, ts.URL+"/saved-queries", nil)
		reqAfter.Header.Set("X-User-ID", userTest.String())
		resAfter, err := httpClient.Do(reqAfter)
		require.NoError(t, err)
		var list2 []marketing.SavedQuery
		require.NoError(t, json.NewDecoder(resAfter.Body).Decode(&list2))
		resAfter.Body.Close()
		require.Len(t, list2, 2)
		assert.Equal(t, idA, list2[0].ID, "After update: A must appear before B")
		assert.Equal(t, idB, list2[1].ID)

		// Stable id tie-break test where updated_at timestamps are equal: id ASC
		sameTime := time.Now().UTC().Truncate(time.Microsecond)
		tieUser := uuid.New()
		_, err = pool.Exec(context.Background(), `
			INSERT INTO users (id, name, email, password_hash, role, created_at, updated_at)
			VALUES ($1, 'Tie User', $2, 'hash', 'admin', now(), now())
		`, tieUser, "tie_test@zamk.app")
		require.NoError(t, err)
		userIDs = append(userIDs, tieUser)

		id1 := uuid.MustParse("11111111-1111-1111-1111-111111111111")
		id2 := uuid.MustParse("22222222-2222-2222-2222-222222222222")

		_, err = pool.Exec(context.Background(), `
			INSERT INTO marketing_saved_queries (id, name, description, query_version, query_spec, created_by, created_at, updated_at)
			VALUES
				($1, 'Tie 2', 'Desc 2', 1, $3, $4, $5, $5),
				($2, 'Tie 1', 'Desc 1', 1, $3, $4, $5, $5)
		`, id2, id1, specBytes, tieUser, sameTime)
		require.NoError(t, err)
		createdIDs = append(createdIDs, id1, id2)

		reqTie, _ := http.NewRequest(http.MethodGet, ts.URL+"/saved-queries", nil)
		reqTie.Header.Set("X-User-ID", tieUser.String())
		resTie, err := httpClient.Do(reqTie)
		require.NoError(t, err)
		var listTie []marketing.SavedQuery
		require.NoError(t, json.NewDecoder(resTie.Body).Decode(&listTie))
		resTie.Body.Close()
		require.Len(t, listTie, 2)
		assert.Equal(t, id1, listTie[0].ID, "Stable tie-break: id ASC ordering when updated_at is equal")
		assert.Equal(t, id2, listTie[1].ID)
	})

	t.Run("Conflict (case-insensitive)", func(t *testing.T) {
		createReq2 := marketing.SavedQueryCreateRequest{
			Name:      "UPDATED NAME", // Same name conceptually
			QuerySpec: specBytes,
		}
		createBytes2, _ := json.Marshal(createReq2)

		req, _ := http.NewRequest(http.MethodPost, ts.URL+"/saved-queries", bytes.NewBuffer(createBytes2))
		req.Header.Set("X-User-ID", userID.String())

		res, err := httpClient.Do(req)
		require.NoError(t, err)
		defer res.Body.Close()

		assert.Equal(t, http.StatusConflict, res.StatusCode)
	})

	t.Run("Cross-User Isolation", func(t *testing.T) {
		req, _ := http.NewRequest(http.MethodGet, ts.URL+"/saved-queries/"+savedID.String(), nil)
		req.Header.Set("X-User-ID", userID2.String())

		res, err := httpClient.Do(req)
		require.NoError(t, err)
		defer res.Body.Close()

		assert.Equal(t, http.StatusNotFound, res.StatusCode) // Cannot read other user's query

		// Create same name for user 2 is allowed
		createReq2 := marketing.SavedQueryCreateRequest{
			Name:      "Updated Name",
			QuerySpec: specBytes,
		}
		createBytes2, _ := json.Marshal(createReq2)

		req2, _ := http.NewRequest(http.MethodPost, ts.URL+"/saved-queries", bytes.NewBuffer(createBytes2))
		req2.Header.Set("X-User-ID", userID2.String())

		res2, err := httpClient.Do(req2)
		require.NoError(t, err)
		defer res2.Body.Close()

		assert.Equal(t, http.StatusCreated, res2.StatusCode)

		var sq marketing.SavedQuery
		require.NoError(t, json.NewDecoder(res2.Body).Decode(&sq))
		createdIDs = append(createdIDs, sq.ID)
	})

	t.Run("Validation: Bad Spec", func(t *testing.T) {
		badSpecReq := marketing.SavedQueryCreateRequest{
			Name:      "Bad Spec",
			QuerySpec: []byte(`{"version": 999}`), // version 999
		}
		badBytes, _ := json.Marshal(badSpecReq)

		req, _ := http.NewRequest(http.MethodPost, ts.URL+"/saved-queries", bytes.NewBuffer(badBytes))
		req.Header.Set("X-User-ID", userID.String())

		res, err := httpClient.Do(req)
		require.NoError(t, err)
		defer res.Body.Close()

		assert.Equal(t, http.StatusBadRequest, res.StatusCode)
	})

	t.Run("Consistency Test: Execute via M5", func(t *testing.T) {
		// Read from DB
		sq, err := service.SavedQueriesRepo.GetByID(context.Background(), savedID, userID)
		require.NoError(t, err)

		// Parse the stored spec
		var storedReq marketing.QueryRequest
		require.NoError(t, json.Unmarshal(sq.QuerySpec, &storedReq))

		// Execute the stored req
		resp, err := service.Analytics.ExecuteQuery(context.Background(), storedReq)
		require.NoError(t, err)

		assert.Equal(t, 1, resp.Version)
		assert.Equal(t, "sourceKey", resp.Columns[0].Key)
		assert.Equal(t, "sourceKind", resp.Columns[1].Key)
		assert.Equal(t, "source", resp.Columns[2].Key)
		assert.Equal(t, "sessions", resp.Columns[3].Key)
	})

	t.Run("Validation: Extra JSON Fields Dropped", func(t *testing.T) {
		malformedSpec := []byte(`{"version":1,"period":{"from":"2023-01-01T00:00:00Z","to":"2023-01-02T00:00:00Z"},"dimensions":["source"],"metrics":["sessions"],"sql":"SELECT * FROM users"}`)
		badSpecReq := marketing.SavedQueryCreateRequest{
			Name:      "Extra JSON",
			QuerySpec: malformedSpec,
		}
		badBytes, _ := json.Marshal(badSpecReq)

		req, _ := http.NewRequest(http.MethodPost, ts.URL+"/saved-queries", bytes.NewBuffer(badBytes))
		req.Header.Set("X-User-ID", userID.String())

		res, err := httpClient.Do(req)
		require.NoError(t, err)
		defer res.Body.Close()

		assert.Equal(t, http.StatusCreated, res.StatusCode)

		var sq marketing.SavedQuery
		require.NoError(t, json.NewDecoder(res.Body).Decode(&sq))
		createdIDs = append(createdIDs, sq.ID)

		// Assert "sql" is stripped from the saved spec
		var specMap map[string]interface{}
		require.NoError(t, json.Unmarshal(sq.QuerySpec, &specMap))
		_, hasSQL := specMap["sql"]
		assert.False(t, hasSQL, "extra 'sql' field must be dropped from persisted query spec")
	})

	t.Run("Delete", func(t *testing.T) {
		req, _ := http.NewRequest(http.MethodDelete, ts.URL+"/saved-queries/"+savedID.String(), nil)
		req.Header.Set("X-User-ID", userID.String())

		res, err := httpClient.Do(req)
		require.NoError(t, err)
		defer res.Body.Close()

		assert.Equal(t, http.StatusNoContent, res.StatusCode)

		_, err = service.SavedQueriesRepo.GetByID(context.Background(), savedID, userID)
		assert.ErrorIs(t, err, marketing.ErrSavedQueryNotFound)

		// Second DELETE should return 404 cleanly, not 500
		req2, _ := http.NewRequest(http.MethodDelete, ts.URL+"/saved-queries/"+savedID.String(), nil)
		req2.Header.Set("X-User-ID", userID.String())

		res2, err := httpClient.Do(req2)
		require.NoError(t, err)
		defer res2.Body.Close()

		assert.Equal(t, http.StatusNotFound, res2.StatusCode)
	})
}
