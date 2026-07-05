package dashboard

import "context"

type Service struct {
	repo *Repository
}

func NewService(repo *Repository) *Service {
	return &Service{repo: repo}
}

func (s *Service) GetDashboardSummary(ctx context.Context) (*DashboardSummary, error) {
	summary, err := s.repo.GetSummary(ctx)
	if err != nil {
		return nil, err
	}

	attention := []AttentionItem{}

	if summary.Products.PendingModeration > 0 {
		attention = append(attention, AttentionItem{
			Title:    "Товары на модерации",
			Count:    summary.Products.PendingModeration,
			Severity: "warning",
			Link:     "/admin/inventory",
		})
	}

	if summary.Auctions.UnpaidManualReview > 0 {
		attention = append(attention, AttentionItem{
			Title:    "Лоты без оплаты (ручная проверка)",
			Count:    summary.Auctions.UnpaidManualReview,
			Severity: "danger",
			Link:     "/admin/auctions",
		})
	}

	if summary.Inventory.LowStockVariants > 0 {
		attention = append(attention, AttentionItem{
			Title:    "Мало остатков",
			Count:    summary.Inventory.LowStockVariants,
			Severity: "info",
			Link:     "/admin/inventory",
		})
	}

	if summary.Payments.FailedPaymentsCount > 0 {
		attention = append(attention, AttentionItem{
			Title:    "Ошибки платежей",
			Count:    summary.Payments.FailedPaymentsCount,
			Severity: "danger",
		})
	}

	if summary.Sellers.WaitingModeration > 0 {
		attention = append(attention, AttentionItem{
			Title:    "Продавцы ожидают активации",
			Count:    summary.Sellers.WaitingModeration,
			Severity: "warning",
			Link:     "/admin/sellers",
		})
	}

	if summary.Orders.NewOrPending > 0 {
		attention = append(attention, AttentionItem{
			Title:    "Новые заказы",
			Count:    summary.Orders.NewOrPending,
			Severity: "info",
			Link:     "/admin/orders",
		})
	}

	summary.Attention = attention

	return summary, nil
}
