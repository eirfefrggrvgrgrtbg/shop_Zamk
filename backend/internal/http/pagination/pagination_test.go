package pagination_test

import (
	"net/http/httptest"
	"testing"

	"github.com/eirfefrggrvgrgrtbg/shop-zamk/backend/internal/http/pagination"
	"github.com/stretchr/testify/assert"
)

func TestFromRequest(t *testing.T) {
	t.Run("default params when empty", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/test", nil)
		p := pagination.FromRequest(req)
		assert.Equal(t, 50, p.Limit)
		assert.Equal(t, 0, p.Offset)
	})

	t.Run("respects explicit limit and offset", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/test?limit=25&offset=50", nil)
		p := pagination.FromRequest(req)
		assert.Equal(t, 25, p.Limit)
		assert.Equal(t, 50, p.Offset)
	})

	t.Run("clamps limit to MaxLimit", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/test?limit=500", nil)
		p := pagination.FromRequest(req)
		assert.Equal(t, 100, p.Limit)
	})

	t.Run("calculates offset from page when offset is omitted", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/test?limit=20&page=3", nil)
		p := pagination.FromRequest(req)
		assert.Equal(t, 20, p.Limit)
		assert.Equal(t, 40, p.Offset) // (3 - 1) * 20 = 40
	})

	t.Run("page 1 results in offset 0", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/test?limit=20&page=1", nil)
		p := pagination.FromRequest(req)
		assert.Equal(t, 20, p.Limit)
		assert.Equal(t, 0, p.Offset)
	})

	t.Run("explicit offset overrides page", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/test?limit=20&page=3&offset=15", nil)
		p := pagination.FromRequest(req)
		assert.Equal(t, 20, p.Limit)
		assert.Equal(t, 15, p.Offset)
	})
}
