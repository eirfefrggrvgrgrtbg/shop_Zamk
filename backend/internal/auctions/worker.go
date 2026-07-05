package auctions

import (
	"context"
	"log/slog"
	"time"
)

func StartMaintenanceWorker(ctx context.Context, svc *Service, interval time.Duration, limit int, logger *slog.Logger) {
	logger.Info("starting auction maintenance worker", "interval", interval)
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	// Run once on startup
	now := time.Now()
	checked, expired, err := svc.ExpireUnpaidAuctionLots(ctx, now, limit)
	if err != nil {
		logger.Error("auction maintenance startup error", "error", err)
	} else if checked > 0 || expired > 0 {
		logger.Info("auction maintenance startup ran", "checked", checked, "expired", expired)
	}

	for {
		select {
		case <-ctx.Done():
			logger.Info("stopping auction maintenance worker")
			return
		case <-ticker.C:
			now := time.Now()
			checked, expired, err := svc.ExpireUnpaidAuctionLots(ctx, now, limit)
			if err != nil {
				logger.Error("auction maintenance worker error", "error", err)
			} else if checked > 0 || expired > 0 {
				logger.Info("auction maintenance worker ran", "checked", checked, "expired", expired)
			}
		}
	}
}
