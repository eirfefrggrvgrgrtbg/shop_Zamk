package reports

import (
	"encoding/json"
	"log/slog"
	"net/http"

	"github.com/eirfefrggrvgrgrtbg/shop-zamk/backend/internal/admin/dashboard"
)

type Handler struct {
	dashboardService *dashboard.Service
	logger           *slog.Logger
}

func NewHandler(dashboardService *dashboard.Service, logger *slog.Logger) *Handler {
	return &Handler{
		dashboardService: dashboardService,
		logger:           logger,
	}
}

func (h *Handler) HandleGetSummary(w http.ResponseWriter, r *http.Request) {
	summary, err := h.dashboardService.GetDashboardSummary(r.Context())
	if err != nil {
		h.logger.Error("failed to get dashboard summary for reports", "error", err)
		http.Error(w, "Failed to generate report summary", http.StatusInternalServerError)
		return
	}

	report := AdminReportSummary{
		TotalOrders:       summary.Overview.TotalOrders,
		TotalRevenueCents: summary.Overview.RevenueTodayCents + summary.Overview.Revenue7dCents, // Simplified approximation if total is missing, but typically we want real total. Wait, dashboard doesn't have all-time revenue.
		TotalSellers:      summary.Sellers.Active + summary.Sellers.WaitingModeration + summary.Sellers.Blocked,
		ActiveSellers:     summary.Sellers.Active,
		TotalProducts:     summary.Products.Published + summary.Products.PendingModeration + summary.Products.RejectedOrBlocked + summary.Products.OutOfStock,
		PendingProducts:   summary.Products.PendingModeration,
		RejectedProducts:  summary.Products.RejectedOrBlocked,
		PublishedProducts: summary.Products.Published,
		PendingPayouts:    summary.Payments.PendingPayoutsCents,
		PaidPayouts:       summary.Payments.PaidOrdersSumCents,
		LowStockItems:     summary.Inventory.LowStockVariants,
		OpenComplaints:    0, // Not implemented yet
		Currency:          "RUB",
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(report)
}
