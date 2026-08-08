#!/bin/bash
set -e

# Uses credentials from TEST_DATABASE_URL, or defaults to a standard test URL
TEST_DB_URL=${TEST_DATABASE_URL:-"postgres://zamk:zamk_password@localhost:5433/zamk_test?sslmode=disable"}

if [[ ! "$TEST_DB_URL" == *"_test"* ]]; then
  echo "Error: TEST_DATABASE_URL must contain '_test' suffix."
  exit 1
fi

echo "Testing clean migrations on $TEST_DB_URL"

# We run a go script to recreate the database to ensure it's clean
cat << 'EOF' > /tmp/recreate_test_db.go
package main

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/jackc/pgx/v5"
)

func main() {
	dbURL := os.Args[1]
	if !strings.Contains(dbURL, "_test") {
		panic("Not a test DB")
	}

	// Extract base url and dbname
	parts := strings.Split(dbURL, "/")
	base := strings.Join(parts[:len(parts)-1], "/") + "/postgres?sslmode=disable"
	dbName := strings.Split(parts[len(parts)-1], "?")[0]

	ctx := context.Background()
	conn, err := pgx.Connect(ctx, base)
	if err != nil {
		panic(err)
	}
	defer conn.Close(ctx)

	conn.Exec(ctx, "DROP DATABASE IF EXISTS " + dbName)
	_, err = conn.Exec(ctx, "CREATE DATABASE " + dbName)
	if err != nil {
		panic(err)
	}
	fmt.Println("Cleaned database", dbName)
}
EOF
go run /tmp/recreate_test_db.go "$TEST_DB_URL"

echo "Running migrate up..."
go run -tags 'postgres' github.com/golang-migrate/migrate/v4/cmd/migrate@latest -path migrations -database "$TEST_DB_URL" up

echo "Checking migration status..."
cat << 'EOF' > /tmp/check_mig_status.go
package main

import (
	"context"
	"fmt"
	"os"

	"github.com/jackc/pgx/v5"
)

func main() {
	dbURL := os.Args[1]
	ctx := context.Background()
	conn, err := pgx.Connect(ctx, dbURL)
	if err != nil {
		panic(err)
	}
	defer conn.Close(ctx)

	var version int
	var dirty bool
	err = conn.QueryRow(ctx, "SELECT version, dirty FROM schema_migrations LIMIT 1").Scan(&version, &dirty)
	if err != nil {
		panic(err)
	}
	if dirty {
		fmt.Printf("Error: migration is dirty (version %d)\n", version)
		os.Exit(1)
	}
	fmt.Printf("Migration check passed: version=%d dirty=false\n", version)
}
EOF
go run /tmp/check_mig_status.go "$TEST_DB_URL"

echo "Checking first PAY- number..."
cat << 'EOF' > /tmp/check_pay_number.go
package main

import (
	"context"
	"fmt"
	"os"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

func main() {
	dbURL := os.Args[1]
	ctx := context.Background()
	conn, err := pgx.Connect(ctx, dbURL)
	if err != nil {
		panic(err)
	}
	defer conn.Close(ctx)

	uid := uuid.New()
	conn.Exec(ctx, "INSERT INTO users (id, name, phone, email, password_hash, created_at, updated_at) VALUES ($1, 'Test', '+123', 'test@example.com', 'h', now(), now())", uid)
	oid := uuid.New()
	conn.Exec(ctx, "INSERT INTO orders (id, user_id, order_number, status, total_price_cents, currency, delivery_address, delivery_method_name, delivery_price_cents, customer_name, customer_email, customer_phone, created_at, updated_at) VALUES ($1, $2, 'ORD-2', 'created', 1000, 'RUB', 'Addr', 'Std', 0, 'Test', 't@e.com', '+12', now(), now())", oid, uid)
	
	pid1 := uuid.New()
	conn.Exec(ctx, "INSERT INTO payments (id, order_id, provider, provider_payment_id, status, amount_cents, currency, idempotency_key, created_at) VALUES ($1, $2, 'tbank', 'P-1', 'created', 1000, 'RUB', 'IDEM-1', now())", pid1, oid)

	var pn string
	err = conn.QueryRow(ctx, "SELECT payment_number FROM payments WHERE id = $1", pid1).Scan(&pn)
	if err != nil {
		panic(err)
	}
	if pn != "PAY-000001" {
		fmt.Printf("Error: expected PAY-000001, got %s\n", pn)
		os.Exit(1)
	}
	fmt.Printf("First payment number check passed: %s\n", pn)
}
EOF
go run /tmp/check_pay_number.go "$TEST_DB_URL"

echo "SUCCESS: Clean migration test passed."
