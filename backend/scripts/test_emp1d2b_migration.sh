#!/bin/bash
set -e

# Canonical test database URL
TEST_DB_URL=${TEST_DATABASE_URL:-"postgres://zamk:zamk_password@localhost:5433/zamk_test?sslmode=disable"}
REQUIRED_DB_NAME="zamk_test"

# Explicit non-floating golang-migrate version
MIGRATE_PKG="github.com/golang-migrate/migrate/v4/cmd/migrate@v4.17.0"

echo "=== EMP.1D2B Migration Acceptance Script ==="

# Dynamically derive current repository latest migration version
get_latest_migration_version() {
	ls migrations/*_*.up.sql | sed -E 's/.*\/([0-9]+)_.*/\1/' | sort -n | tail -n 1 | sed 's/^0*//'
}

# Exact DB guard: must fail closed if connection fails or database is not strictly zamk_test
verify_exact_database() {
	cat << 'GOEOF' > /tmp/verify_exact_test_db_1d2b.go
package main

import (
	"context"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
)

func main() {
	if len(os.Args) < 3 {
		fmt.Fprintf(os.Stderr, "FATAL: database URL and expected database name required\n")
		os.Exit(1)
	}
	dbURL := os.Args[1]
	expectedDB := os.Args[2]

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	conn, err := pgx.Connect(ctx, dbURL)
	if err != nil {
		fmt.Fprintf(os.Stderr, "FATAL: database connection failed: %v\n", err)
		os.Exit(1)
	}
	defer conn.Close(ctx)

	var currentDB string
	err = conn.QueryRow(ctx, "SELECT current_database()").Scan(&currentDB)
	if err != nil {
		fmt.Fprintf(os.Stderr, "FATAL: failed to query current_database(): %v\n", err)
		os.Exit(1)
	}

	currentDB = strings.TrimSpace(currentDB)
	if currentDB != expectedDB {
		fmt.Fprintf(os.Stderr, "FATAL: REFUSING OPERATION: expected database %q, connected to %q\n", expectedDB, currentDB)
		os.Exit(1)
	}

	fmt.Printf("Exact database guard PASS: current_database() == %q\n", currentDB)
}
GOEOF
	go run /tmp/verify_exact_test_db_1d2b.go "$TEST_DB_URL" "$REQUIRED_DB_NAME"
}

# Safely drop and recreate strictly zamk_test
recreate_test_database() {
	cat << 'GOEOF' > /tmp/recreate_exact_test_db_1d2b.go
package main

import (
	"context"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
)

const targetDB = "zamk_test"

func main() {
	if len(os.Args) < 2 {
		fmt.Fprintf(os.Stderr, "FATAL: database URL required\n")
		os.Exit(1)
	}
	dbURL := os.Args[1]

	parts := strings.Split(dbURL, "/")
	if len(parts) < 4 {
		fmt.Fprintf(os.Stderr, "FATAL: malformed database URL\n")
		os.Exit(1)
	}
	base := strings.Join(parts[:len(parts)-1], "/") + "/postgres?sslmode=disable"

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	conn, err := pgx.Connect(ctx, base)
	if err != nil {
		fmt.Fprintf(os.Stderr, "FATAL: failed to connect to maintenance database: %v\n", err)
		os.Exit(1)
	}
	defer conn.Close(ctx)

	// Terminate any active connections to zamk_test before dropping
	_, _ = conn.Exec(ctx, `
		SELECT pg_terminate_backend(pid)
		FROM pg_stat_activity
		WHERE datname = 'zamk_test' AND pid <> pg_backend_pid()
	`)

	// Drop and recreate strictly 'zamk_test'
	_, err = conn.Exec(ctx, "DROP DATABASE IF EXISTS zamk_test")
	if err != nil {
		fmt.Fprintf(os.Stderr, "FATAL: failed to drop database %s: %v\n", targetDB, err)
		os.Exit(1)
	}

	_, err = conn.Exec(ctx, "CREATE DATABASE zamk_test")
	if err != nil {
		fmt.Fprintf(os.Stderr, "FATAL: failed to create database %s: %v\n", targetDB, err)
		os.Exit(1)
	}

	fmt.Printf("Safely recreated clean %s database\n", targetDB)
}
GOEOF
	go run /tmp/recreate_exact_test_db_1d2b.go "$TEST_DB_URL"
}

# Verify schema_migrations is clean at expected version
verify_migration_status() {
	cat << 'GOEOF' > /tmp/verify_clean_mig_status_1d2b.go
package main

import (
	"context"
	"fmt"
	"os"
	"strconv"
	"time"

	"github.com/jackc/pgx/v5"
)

func main() {
	if len(os.Args) < 3 {
		fmt.Fprintf(os.Stderr, "FATAL: database URL and expected version required\n")
		os.Exit(1)
	}
	dbURL := os.Args[1]
	expectedVer, err := strconv.Atoi(os.Args[2])
	if err != nil {
		fmt.Fprintf(os.Stderr, "FATAL: invalid expected version: %v\n", err)
		os.Exit(1)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	conn, err := pgx.Connect(ctx, dbURL)
	if err != nil {
		fmt.Fprintf(os.Stderr, "FATAL: connection failed: %v\n", err)
		os.Exit(1)
	}
	defer conn.Close(ctx)

	var version int
	var dirty bool
	err = conn.QueryRow(ctx, "SELECT version, dirty FROM schema_migrations LIMIT 1").Scan(&version, &dirty)
	if err != nil {
		fmt.Fprintf(os.Stderr, "FATAL: failed to query schema_migrations: %v\n", err)
		os.Exit(1)
	}

	if dirty {
		fmt.Fprintf(os.Stderr, "FATAL: schema_migrations dirty=true (version %d)\n", version)
		os.Exit(1)
	}
	if version != expectedVer {
		fmt.Fprintf(os.Stderr, "FATAL: expected version %d, got %d\n", expectedVer, version)
		os.Exit(1)
	}

	fmt.Printf("Migration status verified: version=%d dirty=false\n", version)
}
GOEOF
	go run /tmp/verify_clean_mig_status_1d2b.go "$TEST_DB_URL" "$1"
}

restore_clean_zamk_test() {
	LATEST_VER=$(get_latest_migration_version)
	echo "Restoring zamk_test to repository latest clean schema (version $LATEST_VER)..."
	recreate_test_database
	go run -tags 'postgres' "$MIGRATE_PKG" -path migrations -database "$TEST_DB_URL" up
	verify_migration_status "$LATEST_VER"
}

RESET_PERFORMED=0
CLEANUP_DONE=0

cleanup_trap() {
	EXIT_CODE=$?
	if [ "$CLEANUP_DONE" -eq 1 ]; then
		return
	fi
	CLEANUP_DONE=1

	if [ "$EXIT_CODE" -ne 0 ] && [ "$RESET_PERFORMED" -eq 1 ]; then
		echo "================================================================="
		echo "FAILURE DETECTED. RUNNING EMERGENCY RESTORE..."
		echo "================================================================="
		restore_clean_zamk_test
		exit "$EXIT_CODE"
	fi
}

trap cleanup_trap EXIT

# 1. Exact Database Guard
verify_exact_database

# 2. Destructive Reset
recreate_test_database
RESET_PERFORMED=1

# 3. Establish schema through 000085
echo "Migrating up to version 000085..."
go run -tags 'postgres' "$MIGRATE_PKG" -path migrations -database "$TEST_DB_URL" goto 85
verify_migration_status 85

# 4. Seed exact fixture rows at schema 85 (before 000086)
echo "Seeding pre-existing staff member at schema 85..."
cat << 'GOEOF' > /tmp/seed_fixtures_85.go
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

	// Ensure responsibilities column does NOT exist at schema 85
	var exists bool
	err = conn.QueryRow(ctx, `
		SELECT EXISTS (
			SELECT 1 FROM information_schema.columns
			WHERE table_name = 'staff_members' AND column_name = 'responsibilities'
		)
	`).Scan(&exists)
	if err != nil {
		panic(err)
	}
	if exists {
		panic("Invalid state: responsibilities column already exists at version 85")
	}

	var roleID uuid.UUID
	err = conn.QueryRow(ctx, `SELECT id FROM staff_roles WHERE code = 'owner'`).Scan(&roleID)
	if err != nil {
		panic(err)
	}

	userID := uuid.MustParse("44444444-dddd-dddd-dddd-dddddddddddd")
	_, err = conn.Exec(ctx, `
		INSERT INTO users (id, phone, name, email, password_hash, role, status, created_at, updated_at)
		VALUES ($1, '+79994444444', 'Pre-existing Staff', 'preexisting@zamk.test', 'hash', 'admin', 'active', NOW(), NOW())
	`, userID)
	if err != nil {
		panic(err)
	}

	_, err = conn.Exec(ctx, `
		INSERT INTO staff_members (user_id, staff_role_id, status, created_at, updated_at)
		VALUES ($1, $2, 'active', NOW(), NOW())
	`, userID, roleID)
	if err != nil {
		panic(err)
	}

	fmt.Println("Seeded pre-existing staff member at schema 85.")
}
GOEOF
go run /tmp/seed_fixtures_85.go "$TEST_DB_URL"

# 5. Apply ACTUAL 000086 UP migration via real migration runner
echo "Running ACTUAL 000086 UP migration via golang-migrate..."
go run -tags 'postgres' "$MIGRATE_PKG" -path migrations -database "$TEST_DB_URL" up 1
verify_migration_status 86

# 6. Assert schema 86: columns exist, NULL accepted, 4000 char accepted, 4001 char rejected, existing row preserved
echo "Asserting migration 000086 guarantees..."
cat << 'GOEOF' > /tmp/assert_migration_86.go
package main

import (
	"context"
	"fmt"
	"os"
	"strings"

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

	// 1. Verify columns exist in information_schema
	checkCol := func(col string) {
		var exists bool
		err = conn.QueryRow(ctx, `
			SELECT EXISTS (
				SELECT 1 FROM information_schema.columns
				WHERE table_name = 'staff_members' AND column_name = $1
			)
		`, col).Scan(&exists)
		if err != nil {
			panic(err)
		}
		if !exists {
			panic(fmt.Sprintf("ASSERTION FAILED: column %s does not exist on staff_members", col))
		}
	}
	checkCol("responsibilities")
	checkCol("work_note")
	fmt.Println("Columns responsibilities and work_note exist PASS")

	// 2. Pre-existing staff row preserved with NULLs
	userID := uuid.MustParse("44444444-dddd-dddd-dddd-dddddddddddd")
	var status string
	var resp, note *string
	err = conn.QueryRow(ctx, `
		SELECT status, responsibilities, work_note
		FROM staff_members
		WHERE user_id = $1
	`, userID).Scan(&status, &resp, &note)
	if err != nil {
		panic(fmt.Sprintf("Failed to query pre-existing staff member: %v", err))
	}
	if status != "active" || resp != nil || note != nil {
		panic("ASSERTION FAILED: pre-existing staff member was corrupted or values not NULL")
	}
	fmt.Println("Pre-existing staff member preserved with NULL values PASS")

	// 3. NULL accepted
	_, err = conn.Exec(ctx, `
		UPDATE staff_members
		SET responsibilities = NULL, work_note = NULL
		WHERE user_id = $1
	`, userID)
	if err != nil {
		panic(fmt.Sprintf("ASSERTION FAILED: updating to NULL failed: %v", err))
	}
	fmt.Println("Explicit NULL accepted PASS")

	// 4. Exactly 4000 multibyte characters accepted
	multibyte4000 := strings.Repeat("Ж", 4000)
	_, err = conn.Exec(ctx, `
		UPDATE staff_members
		SET responsibilities = $1, work_note = $1
		WHERE user_id = $2
	`, multibyte4000, userID)
	if err != nil {
		panic(fmt.Sprintf("ASSERTION FAILED: updating with 4000 multibyte characters failed: %v", err))
	}
	fmt.Println("4000 multibyte Unicode characters accepted PASS")

	// 5. 4001 characters rejected by DB CHECK constraint for responsibilities
	multibyte4001 := strings.Repeat("Ж", 4001)
	_, err = conn.Exec(ctx, `
		UPDATE staff_members
		SET responsibilities = $1
		WHERE user_id = $2
	`, multibyte4001, userID)
	if err == nil {
		panic("ASSERTION FAILED: 4001 characters in responsibilities should have been rejected by CHECK constraint")
	}
	if !strings.Contains(err.Error(), "responsibilities_length") {
		panic(fmt.Sprintf("ASSERTION FAILED: expected error to mention responsibilities_length, got %v", err))
	}
	fmt.Println("4001 characters in responsibilities rejected by DB constraint PASS")

	// 6. 4001 characters rejected by DB CHECK constraint for work_note
	_, err = conn.Exec(ctx, `
		UPDATE staff_members
		SET work_note = $1
		WHERE user_id = $2
	`, multibyte4001, userID)
	if err == nil {
		panic("ASSERTION FAILED: 4001 characters in work_note should have been rejected by CHECK constraint")
	}
	if !strings.Contains(err.Error(), "work_note_length") {
		panic(fmt.Sprintf("ASSERTION FAILED: expected error to mention work_note_length, got %v", err))
	}
	fmt.Println("4001 characters in work_note rejected by DB constraint PASS")

	fmt.Println("SUCCESS: All migration 000086 assertions passed.")
}
GOEOF
go run /tmp/assert_migration_86.go "$TEST_DB_URL"

# 7. Safe cleanup: restore clean test database with repository latest migration
restore_clean_zamk_test
CLEANUP_DONE=1

echo "=== EMP.1D2B Migration Acceptance Script COMPLETE: SUCCESS ==="
