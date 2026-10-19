package selleranalytics

import (
	"math"
	"strconv"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParsePagination(t *testing.T) {
	t.Run("defaults when omitted", func(t *testing.T) {
		limit, offset, err := parsePagination("", "")
		require.NoError(t, err)
		assert.Equal(t, 50, limit)
		assert.Equal(t, 0, offset)
	})

	t.Run("valid values", func(t *testing.T) {
		limit, offset, err := parsePagination("50", "10")
		require.NoError(t, err)
		assert.Equal(t, 50, limit)
		assert.Equal(t, 10, offset)
	})

	t.Run("limit=0 keeps existing behaviour", func(t *testing.T) {
		limit, _, err := parsePagination("0", "")
		require.NoError(t, err)
		assert.Equal(t, 0, limit)
	})

	t.Run("very large offset is valid", func(t *testing.T) {
		_, offset, err := parsePagination("", strconv.Itoa(math.MaxInt))
		require.NoError(t, err)
		assert.Equal(t, math.MaxInt, offset)
	})

	invalid := []struct{ name, limit, offset string }{
		{"limit=-1", "-1", ""},
		{"offset=-1", "", "-1"},
		{"limit=abc", "abc", ""},
		{"offset=abc", "", "abc"},
		{"limit=1.5", "1.5", ""},
		{"limit exceeds int", "99999999999999999999", ""},
		{"offset exceeds int", "", "99999999999999999999"},
	}
	for _, tc := range invalid {
		t.Run("rejects "+tc.name, func(t *testing.T) {
			_, _, err := parsePagination(tc.limit, tc.offset)
			assert.Error(t, err)
		})
	}
}

func TestPaginateProducts(t *testing.T) {
	items := make([]ProductRow, 5)
	for i := range items {
		items[i].ProductID = strconv.Itoa(i)
	}
	ids := func(rows []ProductRow) []string {
		out := []string{}
		for _, r := range rows {
			out = append(out, r.ProductID)
		}
		return out
	}

	assert.Equal(t, []string{"1", "2"}, ids(paginateProducts(items, 2, 1)))
	assert.Equal(t, []string{"0", "1", "2", "3", "4"}, ids(paginateProducts(items, 50, 0)))
	assert.Equal(t, []string{"3", "4"}, ids(paginateProducts(items, 50, 3)))
	assert.Empty(t, paginateProducts(items, 0, 0))
	assert.Empty(t, paginateProducts(items, 50, 5))
	assert.Empty(t, paginateProducts(items, 50, 1000000))
	assert.Empty(t, paginateProducts(items, 50, math.MaxInt))
	assert.Empty(t, paginateProducts(items, math.MaxInt, math.MaxInt))
	assert.Equal(t, []string{"4"}, ids(paginateProducts(items, math.MaxInt, 4)))
	assert.NotPanics(t, func() {
		paginateProducts(items, -1, -1)
		paginateProducts(nil, 50, 0)
	})
}
