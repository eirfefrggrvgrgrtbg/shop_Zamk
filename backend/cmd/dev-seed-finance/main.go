package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"time"

	"github.com/eirfefrggrvgrgrtbg/shop-zamk/backend/internal/config"
	"github.com/eirfefrggrvgrgrtbg/shop-zamk/backend/internal/payouts"
	"github.com/eirfefrggrvgrgrtbg/shop-zamk/backend/internal/platform/postgres"
	"github.com/google/uuid"
	"github.com/joho/godotenv"
)

func main() {
	_ = godotenv.Load()
	cfg, err := config.Load()
	if err != nil {
		fmt.Printf("Config error: %v\n", err)
		os.Exit(1)
	}

	sellerIDStr := flag.String("seller", "", "UUID of the seller")
	flag.Parse()

	if *sellerIDStr == "" {
		fmt.Println("Usage: dev-seed-finance -seller <uuid>")
		os.Exit(1)
	}
	sellerID, err := uuid.Parse(*sellerIDStr)
	if err != nil {
		fmt.Printf("Invalid seller UUID: %v\n", err)
		os.Exit(1)
	}

	ctx := context.Background()
	pgClient, err := postgres.NewClient(ctx, cfg.Postgres.DSN)
	if err != nil {
		fmt.Printf("DB connect error: %v\n", err)
		os.Exit(1)
	}
	defer pgClient.Close()

	if err := SeedFinanceScenario(ctx, pgClient, sellerID); err != nil {
		fmt.Printf("Seed error: %v\n", err)
		os.Exit(1)
	}

	fmt.Println("Finance seeding successful for seller", sellerID)
}

func SeedFinanceScenario(ctx context.Context, client *postgres.Client, sellerID uuid.UUID) error {
	repo := payouts.NewRepository(client.Pool)

	// Clean up previous finance records for this seller for idempotency
	_, err := client.Pool.Exec(ctx, "DELETE FROM payout_batches WHERE seller_id = $1", sellerID)
	if err != nil { return err }
	_, err = client.Pool.Exec(ctx, "DELETE FROM seller_ledger_entries WHERE seller_id = $1", sellerID)
	if err != nil { return err }
	_, err = client.Pool.Exec(ctx, "DELETE FROM seller_commission_rules WHERE seller_id = $1", sellerID)
	if err != nil { return err }

	// Seed commission rule
	adminID := uuid.New()
	_, _ = client.Pool.Exec(ctx, "INSERT INTO users (id, name, email, password_hash, role) VALUES ($1, 'Admin', $2, 'hash', 'admin') ON CONFLICT DO NOTHING", adminID, uuid.New().String()+"@zamk.local")
	rule := &payouts.SellerCommissionRule{
		ID:        uuid.New(),
		SellerID:  sellerID,
		RateBPS:   850,
		Reason:    "initial seeding",
		CreatedBy: adminID,
		CreatedAt: time.Now().Add(-30 * 24 * time.Hour),
	}
	if err := repo.CreateCommissionRule(ctx, rule); err != nil {
		return err
	}

	// 1. Available earnings (Older than 14 days)
	// We'll create two orders and associate earnings.
	order1 := uuid.New()
	_, _ = client.Pool.Exec(ctx, "INSERT INTO users (id, name, email, password_hash, role) VALUES ($1, 'Customer', $2, 'hash', 'customer') ON CONFLICT DO NOTHING", order1, uuid.New().String()+"@zamk.local")
	_, err = client.Pool.Exec(ctx, "INSERT INTO orders (id, user_id, total_price_cents, customer_name, customer_phone, customer_email, delivery_address) VALUES ($1, $1, 20000, 'Customer', 'phone', 'email', 'addr')", order1)
	if err != nil { return err }

	tAvailable1 := time.Now().Add(-5 * 24 * time.Hour)
	err = createLedgerEntryTx(ctx, client, repo, &payouts.SellerLedgerEntry{
		ID:          uuid.New(),
		SellerID:    sellerID,
		OrderID:     &order1,
		Type:        "seller_earning",
		AmountCents: 1700000, // We want the sum to be large enough for testing
		Currency:    "RUB",
		AvailableAt: &tAvailable1,
		CreatedAt:   time.Now().Add(-25 * 24 * time.Hour),
	})
	if err != nil { return err }

	err = createLedgerEntryTx(ctx, client, repo, &payouts.SellerLedgerEntry{
		ID:          uuid.New(),
		SellerID:    sellerID,
		OrderID:     &order1,
		Type:        "sale_gross",
		AmountCents: 1857923,
		Currency:    "RUB",
		AvailableAt: &tAvailable1,
		CreatedAt:   time.Now().Add(-25 * 24 * time.Hour),
	})
	if err != nil { return err }

	err = createLedgerEntryTx(ctx, client, repo, &payouts.SellerLedgerEntry{
		ID:          uuid.New(),
		SellerID:    sellerID,
		OrderID:     &order1,
		Type:        "zamk_commission",
		AmountCents: -157923,
		Currency:    "RUB",
		AvailableAt: &tAvailable1,
		CreatedAt:   time.Now().Add(-25 * 24 * time.Hour),
	})
	if err != nil { return err }


	// 2. Frozen earning (Delivered yesterday, not yet available)
	order2 := uuid.New()
	_, err = client.Pool.Exec(ctx, "INSERT INTO orders (id, user_id, total_price_cents, customer_name, customer_phone, customer_email, delivery_address) VALUES ($1, $2, 50000, 'Customer', 'phone', 'email', 'addr')", order2, order1)
	if err != nil { return err }

	tFrozen := time.Now().Add(13 * 24 * time.Hour)
	err = createLedgerEntryTx(ctx, client, repo, &payouts.SellerLedgerEntry{
		ID:          uuid.New(),
		SellerID:    sellerID,
		OrderID:     &order2,
		Type:        "seller_earning",
		AmountCents: 450000,
		Currency:    "RUB",
		AvailableAt: &tFrozen,
		CreatedAt:   time.Now().Add(-1 * 24 * time.Hour),
	})
	if err != nil { return err }
	
	// Create another un-frozen earning for Unfreeze task
	order3 := uuid.New()
	_, err = client.Pool.Exec(ctx, "INSERT INTO orders (id, user_id, total_price_cents, customer_name, customer_phone, customer_email, delivery_address) VALUES ($1, $2, 10000, 'Customer', 'phone', 'email', 'addr')", order3, order1)
	if err != nil { return err }
	tReadyToUnfreeze := time.Now().Add(-1 * time.Hour)
	err = createLedgerEntryTx(ctx, client, repo, &payouts.SellerLedgerEntry{
		ID:          uuid.New(),
		SellerID:    sellerID,
		OrderID:     &order3,
		Type:        "seller_earning",
		AmountCents: 90000,
		Currency:    "RUB",
		AvailableAt: &tReadyToUnfreeze, // Eligible to be unfrozen
		CreatedAt:   time.Now().Add(-15 * 24 * time.Hour),
	})
	if err != nil { return err }

	// Seed previous payout batch (paid)
	batchID := uuid.New()
	paidAt := time.Now().Add(-10 * 24 * time.Hour)
	err = repo.CreatePayoutBatchTx(ctx, nil, &payouts.PayoutBatch{ // this won't work easily with nil tx if we don't have a wrapper, but we can just use normal sql or tx
		ID:          batchID,
		SellerID:    sellerID,
		AmountCents: 1000000,
		Status:      "paid",
		ProcessedAt: &paidAt,
		CreatedAt:   time.Now().Add(-10 * 24 * time.Hour),
	}) // wait, CreatePayoutBatchTx needs tx
	
	// Better to use raw SQL for seeding batches if Tx is needed, or open a Tx:
	tx, err := client.Pool.Begin(ctx)
	if err != nil { return err }
	defer tx.Rollback(ctx)

	if err := repo.CreatePayoutBatchTx(ctx, tx, &payouts.PayoutBatch{
		ID:          batchID,
		SellerID:    sellerID,
		AmountCents: 1000000,
		Status:      "paid",
		ProcessedAt: &paidAt,
		CreatedAt:   time.Now().Add(-10 * 24 * time.Hour),
	}); err != nil {
		return err
	}
	
	// And a deduction ledger entry for that batch
	if err := repo.CreateLedgerEntryTx(ctx, tx, &payouts.SellerLedgerEntry{
		ID:            uuid.New(),
		SellerID:      sellerID,
		PayoutBatchID: &batchID,
		Type:          "payout",
		AmountCents:   -1000000,
		Currency:      "RUB",
		CreatedAt:     time.Now().Add(-10 * 24 * time.Hour),
	}); err != nil {
		return err
	}

	return tx.Commit(ctx)
}

func createLedgerEntryTx(ctx context.Context, client *postgres.Client, repo *payouts.Repository, entry *payouts.SellerLedgerEntry) error {
	tx, err := client.Pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	if err := repo.CreateLedgerEntryTx(ctx, tx, entry); err != nil {
		return err
	}
	return tx.Commit(ctx)
}
