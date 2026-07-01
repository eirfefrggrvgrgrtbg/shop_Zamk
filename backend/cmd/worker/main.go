package main

import (
	"context"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/joho/godotenv"

	"github.com/eirfefrggrvgrgrtbg/shop-zamk/backend/internal/cart"
	"github.com/eirfefrggrvgrgrtbg/shop-zamk/backend/internal/config"
	"github.com/eirfefrggrvgrgrtbg/shop-zamk/backend/internal/inventory"
	"github.com/eirfefrggrvgrgrtbg/shop-zamk/backend/internal/orders"
	"github.com/eirfefrggrvgrgrtbg/shop-zamk/backend/internal/platform/postgres"
	"github.com/eirfefrggrvgrgrtbg/shop-zamk/backend/internal/platform/redis"
	"github.com/eirfefrggrvgrgrtbg/shop-zamk/backend/internal/returns"
	"github.com/eirfefrggrvgrgrtbg/shop-zamk/backend/internal/payouts"
	"github.com/eirfefrggrvgrgrtbg/shop-zamk/backend/internal/sellers"
)

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	slog.SetDefault(logger)

	_ = godotenv.Load()

	cfg, err := config.Load()
	if err != nil {
		logger.Error("failed to load config", "error", err)
		os.Exit(1)
	}

	ctx := context.Background()

	pgClient, err := postgres.NewClient(ctx, cfg.Postgres.DSN)
	if err != nil {
		logger.Error("failed to connect to postgres", "error", err)
		os.Exit(1)
	}
	defer pgClient.Close()
	logger.Info("connected to postgres")

	redisClient, err := redis.NewClient(ctx, cfg.Redis.Addr, cfg.Redis.Password, cfg.Redis.DB)
	if err != nil {
		logger.Error("failed to connect to redis", "error", err)
		os.Exit(1)
	}
	defer redisClient.Close()
	logger.Info("connected to redis")

	inventoryRepo := inventory.NewRepository(pgClient.Pool)
	sellersRepo := sellers.NewRepository(pgClient.Pool)
	inventoryService := inventory.NewService(inventoryRepo, sellersRepo, pgClient)

	cartRepo := cart.NewRepository(pgClient.Pool)
	ordersRepo := orders.NewRepository(pgClient.Pool)
	ordersService := orders.NewService(ordersRepo, cartRepo, inventoryService, pgClient)

	returnsRepo := returns.NewRepository(pgClient.Pool)
	payoutsRepo := payouts.NewRepository(pgClient.Pool)
	payoutsService := payouts.NewService(payoutsRepo, pgClient, returnsRepo, ordersRepo, cfg)

	logger.Info("worker started successfully")

	shutdown := make(chan os.Signal, 1)
	signal.Notify(shutdown, os.Interrupt, syscall.SIGTERM)

	expirationTicker := time.NewTicker(time.Duration(cfg.Worker.OrderExpirationIntervalSeconds) * time.Second)
	defer expirationTicker.Stop()

	go func() {
		for {
			select {
			case <-ctx.Done():
				return
			case <-expirationTicker.C:
				timeout := time.Duration(cfg.Worker.OrderPaymentTimeoutMinutes) * time.Minute
				olderThan := time.Now().Add(-timeout)

				res, err := ordersService.ExpireAwaitingPaymentOrders(ctx, olderThan, 100)
				if err != nil {
					logger.Error("failed to expire orders", "error", err)
				} else if res.Checked > 0 {
					logger.Info("order expiration run completed",
						"checked", res.Checked,
						"expired", res.Expired,
						"releasedReservations", res.ReleasedReservations,
					)
				}
			}
		}
	}()

	balanceTicker := time.NewTicker(time.Duration(cfg.Worker.SellerBalanceIntervalSeconds) * time.Second)
	defer balanceTicker.Stop()

	go func() {
		for {
			select {
			case <-ctx.Done():
				return
			case t := <-balanceTicker.C:
				processed, err := payoutsService.MakeSellerFundsAvailable(ctx, t, 100)
				if err != nil {
					logger.Error("failed to make seller funds available", "error", err)
				} else if processed > 0 {
					logger.Info("seller funds availability run completed", "processed", processed)
				}
			}
		}
	}()

	sig := <-shutdown
	logger.Info("shutting down worker", "signal", sig)
}
