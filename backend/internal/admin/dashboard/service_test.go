package dashboard_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/eirfefrggrvgrgrtbg/shop-zamk/backend/internal/admin/dashboard"
)

type mockRepo struct {
	summary *dashboard.DashboardSummary
	err     error
}

func (m *mockRepo) GetSummary(ctx context.Context) (*dashboard.DashboardSummary, error) {
	if m.err != nil {
		return nil, m.err
	}
	return m.summary, nil
}

func TestService_GetDashboardSummary_PickingAttention(t *testing.T) {
	ctx := context.Background()

	t.Run("zero_requires_picking_no_attention_item", func(t *testing.T) {
		repo := &mockRepo{
			summary: &dashboard.DashboardSummary{
				Orders: dashboard.OrdersMetrics{
					RequiresPicking: 0,
				},
			},
		}
		svc := dashboard.NewService(repo)
		s, err := svc.GetDashboardSummary(ctx)
		require.NoError(t, err)

		for _, att := range s.Attention {
			assert.NotContains(t, att.Title, "требует сборки")
			assert.NotContains(t, att.Title, "требуют сборки")
		}
	})

	t.Run("one_requires_picking", func(t *testing.T) {
		repo := &mockRepo{
			summary: &dashboard.DashboardSummary{
				Orders: dashboard.OrdersMetrics{
					RequiresPicking: 1,
				},
			},
		}
		svc := dashboard.NewService(repo)
		s, err := svc.GetDashboardSummary(ctx)
		require.NoError(t, err)

		var found *dashboard.AttentionItem
		for _, att := range s.Attention {
			if att.Link == "/admin/fulfillment/picking" {
				item := att
				found = &item
				break
			}
		}
		require.NotNil(t, found)
		assert.Equal(t, "1 заказ требует сборки", found.Title)
		assert.Equal(t, 1, found.Count)
		assert.Equal(t, "warning", found.Severity)
	})

	t.Run("three_requires_picking", func(t *testing.T) {
		repo := &mockRepo{
			summary: &dashboard.DashboardSummary{
				Orders: dashboard.OrdersMetrics{
					RequiresPicking: 3,
				},
			},
		}
		svc := dashboard.NewService(repo)
		s, err := svc.GetDashboardSummary(ctx)
		require.NoError(t, err)

		var found *dashboard.AttentionItem
		for _, att := range s.Attention {
			if att.Link == "/admin/fulfillment/picking" {
				item := att
				found = &item
				break
			}
		}
		require.NotNil(t, found)
		assert.Equal(t, "3 заказа требуют сборки", found.Title)
		assert.Equal(t, 3, found.Count)
	})

	t.Run("five_requires_picking", func(t *testing.T) {
		repo := &mockRepo{
			summary: &dashboard.DashboardSummary{
				Orders: dashboard.OrdersMetrics{
					RequiresPicking: 5,
				},
			},
		}
		svc := dashboard.NewService(repo)
		s, err := svc.GetDashboardSummary(ctx)
		require.NoError(t, err)

		var found *dashboard.AttentionItem
		for _, att := range s.Attention {
			if att.Link == "/admin/fulfillment/picking" {
				item := att
				found = &item
				break
			}
		}
		require.NotNil(t, found)
		assert.Equal(t, "5 заказов требуют сборки", found.Title)
		assert.Equal(t, 5, found.Count)
	})
}
