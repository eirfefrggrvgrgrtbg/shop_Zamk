package main

import (
	"context"
	"fmt"
	"log"
	"os"

	"github.com/eirfefrggrvgrgrtbg/shop-zamk/backend/internal/config"
	"github.com/eirfefrggrvgrgrtbg/shop-zamk/backend/internal/platform/postgres"
	"github.com/google/uuid"
	"github.com/joho/godotenv"
)

func main() {
	_ = godotenv.Load()

	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("failed to load config: %v", err)
	}

	ctx := context.Background()
	pgClient, err := postgres.NewClient(ctx, cfg.Postgres.DSN)
	if err != nil {
		fmt.Println("Error connecting to DB:", err)
		os.Exit(1)
	}
	defer pgClient.Close()
	db := pgClient.Pool

	// Get seller belonging to seller@zamk.local
	var sellerID uuid.UUID
	err = db.QueryRow(ctx, "SELECT su.seller_id FROM seller_users su JOIN users u ON u.id = su.user_id WHERE u.email = 'seller@zamk.local' LIMIT 1").Scan(&sellerID)
	if err != nil {
		fmt.Println("Error getting seller:", err)
		os.Exit(1)
	}

	_, err = db.Exec(ctx, `
		INSERT INTO seller_balance_ledger (id, seller_id, type, amount_cents, currency)
		VALUES ($1, $2, 'manual_adjustment', 500000, 'RUB')
	`, uuid.New(), sellerID)
	
	if err != nil {
		fmt.Println("Error inserting balance:", err)
		os.Exit(1)
	}
	fmt.Println("Balance seeded successfully.")
}
