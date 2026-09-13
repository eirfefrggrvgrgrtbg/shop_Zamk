#!/bin/bash
set -e

# Canonical test database URL
TEST_DB_URL=${TEST_DATABASE_URL:-"postgres://zamk:zamk_password@localhost:5433/zamk_test?sslmode=disable"}
REQUIRED_DB_NAME="zamk_test"

# Explicit non-floating golang-migrate version (v4.17.0 required)
MIGRATE_PKG="github.com/golang-migrate/migrate/v4/cmd/migrate@v4.17.0"

echo "=== EMP.1B2 Migration Acceptance Script ==="

# Dynamically derive current repository latest migration version
get_latest_migration_version() {
	ls migrations/*_*.up.sql | sed -E 's/.*\/([0-9]+)_.*/\1/' | sort -n | tail -n 1 | sed 's/^0*//'
}

# Exact DB guard: must fail closed if connection fails or database is not strictly zamk_test
verify_exact_database() {
	cat << 'GOEOF' > /tmp/verify_exact_test_db_1b2.go
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
	go run /tmp/verify_exact_test_db_1b2.go "$TEST_DB_URL" "$REQUIRED_DB_NAME"
}

# Safely drop and recreate strictly zamk_test
recreate_test_database() {
	cat << 'GOEOF' > /tmp/recreate_exact_test_db_1b2.go
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

	// Drop and recreate strictly 'zamk_test' (literal, safe against injection or misdirection)
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
	go run /tmp/recreate_exact_test_db_1b2.go "$TEST_DB_URL"
}

# Verify schema_migrations is clean at expected version
verify_migration_status() {
	cat << 'GOEOF' > /tmp/verify_clean_mig_status_1b2.go
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
	go run /tmp/verify_clean_mig_status_1b2.go "$TEST_DB_URL" "$1"
}

# Restores zamk_test to repository latest clean schema
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
		echo ""
		echo "================================================================="
		echo "FAILURE DETECTED (exit code $EXIT_CODE). RUNNING EMERGENCY CLEANUP..."
		echo "================================================================="
		if restore_clean_zamk_test; then
			echo "Emergency cleanup SUCCEEDED: zamk_test restored to repository latest clean schema."
		else
			echo "Emergency cleanup FAILED!"
		fi
		exit "$EXIT_CODE"
	fi
}

trap cleanup_trap EXIT

# 1. Exact Database Guard
verify_exact_database

# 2. Destructive Reset
recreate_test_database
RESET_PERFORMED=1

# 3. Establish schema through 000084
echo "Migrating up to version 000084..."
go run -tags 'postgres' "$MIGRATE_PKG" -path migrations -database "$TEST_DB_URL" goto 84
verify_migration_status 84

# 4. Seed exact fixture rows at schema 84 (before 000085)
echo "Seeding exact fixture rows at schema 84 (before 000085)..."
cat << 'GOEOF' > /tmp/seed_fixtures_84.go
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

	// Ensure staff.permissions.manage does NOT exist in staff_role_permissions at schema 84
	var exists bool
	err = conn.QueryRow(ctx, `
		SELECT EXISTS (
			SELECT 1 FROM staff_role_permissions
			WHERE permission = 'staff.permissions.manage'
		)
	`).Scan(&exists)
	if err != nil {
		panic(err)
	}
	if exists {
		panic("Invalid state: staff.permissions.manage already exists in staff_role_permissions at version 84")
	}

	// Fetch IDs for owner, co_owner, admin roles
	var ownerRoleID, coOwnerRoleID, adminRoleID uuid.UUID
	err = conn.QueryRow(ctx, `SELECT id FROM staff_roles WHERE code = 'owner'`).Scan(&ownerRoleID)
	if err != nil {
		panic(fmt.Sprintf("fetch owner role: %v", err))
	}
	err = conn.QueryRow(ctx, `SELECT id FROM staff_roles WHERE code = 'co_owner'`).Scan(&coOwnerRoleID)
	if err != nil {
		panic(fmt.Sprintf("fetch co_owner role: %v", err))
	}
	err = conn.QueryRow(ctx, `SELECT id FROM staff_roles WHERE code = 'admin'`).Scan(&adminRoleID)
	if err != nil {
		panic(fmt.Sprintf("fetch admin role: %v", err))
	}

	// Staff Users: Owner, CoOwner, Admin
	ownerUserID := uuid.MustParse("11111111-aaaa-aaaa-aaaa-aaaaaaaaaaaa")
	coOwnerUserID := uuid.MustParse("22222222-bbbb-bbbb-bbbb-bbbbbbbbbbbb")
	adminUserID := uuid.MustParse("33333333-cccc-cccc-cccc-cccccccccccc")

	_, err = conn.Exec(ctx, `
		INSERT INTO users (id, phone, name, email, password_hash, role, status, created_at, updated_at) VALUES
		($1, '+79991111111', 'Fixture Owner', 'fixture_owner@zamk.test', 'hash', 'admin', 'active', NOW(), NOW()),
		($2, '+79992222222', 'Fixture CoOwner', 'fixture_coowner@zamk.test', 'hash', 'admin', 'active', NOW(), NOW()),
		($3, '+79993333333', 'Fixture Admin', 'fixture_admin@zamk.test', 'hash', 'admin', 'active', NOW(), NOW())
	`, ownerUserID, coOwnerUserID, adminUserID)
	if err != nil {
		panic(err)
	}

	// Staff members
	_, err = conn.Exec(ctx, `
		INSERT INTO staff_members (user_id, staff_role_id, status, created_at, updated_at) VALUES
		($1, $4, 'active', NOW(), NOW()),
		($2, $5, 'active', NOW(), NOW()),
		($3, $6, 'active', NOW(), NOW())
	`, ownerUserID, coOwnerUserID, adminUserID, ownerRoleID, coOwnerRoleID, adminRoleID)
	if err != nil {
		panic(err)
	}

	// Ensure direct permissions do NOT contain staff.permissions.manage before 85
	var directExists bool
	err = conn.QueryRow(ctx, `
		SELECT EXISTS (
			SELECT 1 FROM staff_member_permissions
			WHERE permission = 'staff.permissions.manage'
		)
	`).Scan(&directExists)
	if err != nil {
		panic(err)
	}
	if directExists {
		panic("Invalid state: staff.permissions.manage already present in staff_member_permissions before 85")
	}

	fmt.Println("Seeded Fixture Owner, Fixture CoOwner, and Fixture Admin at schema 84.")
}
GOEOF
go run /tmp/seed_fixtures_84.go "$TEST_DB_URL"

# 5. Apply ACTUAL 000085 UP migration via real migration runner
echo "Running ACTUAL 000085 UP migration via golang-migrate (v4.17.0)..."
go run -tags 'postgres' "$MIGRATE_PKG" -path migrations -database "$TEST_DB_URL" up 1
verify_migration_status 85

# 6. Assert resulting rows in staff_role_permissions and staff_member_permissions
echo "Asserting migration and backfill results after 000085..."
cat << 'GOEOF' > /tmp/assert_backfill_85.go
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

	ownerUserID := uuid.MustParse("11111111-aaaa-aaaa-aaaa-aaaaaaaaaaaa")
	coOwnerUserID := uuid.MustParse("22222222-bbbb-bbbb-bbbb-bbbbbbbbbbbb")
	adminUserID := uuid.MustParse("33333333-cccc-cccc-cccc-cccccccccccc")

	// 1. Role Presets Assertions
	var ownerPresetHas, coOwnerPresetHas, adminPresetHas bool

	err = conn.QueryRow(ctx, `
		SELECT EXISTS (
			SELECT 1 FROM staff_role_permissions srp
			JOIN staff_roles sr ON sr.id = srp.role_id
			WHERE sr.code = 'owner' AND srp.permission = 'staff.permissions.manage'
		)
	`).Scan(&ownerPresetHas)
	if err != nil {
		panic(err)
	}

	err = conn.QueryRow(ctx, `
		SELECT EXISTS (
			SELECT 1 FROM staff_role_permissions srp
			JOIN staff_roles sr ON sr.id = srp.role_id
			WHERE sr.code = 'co_owner' AND srp.permission = 'staff.permissions.manage'
		)
	`).Scan(&coOwnerPresetHas)
	if err != nil {
		panic(err)
	}

	err = conn.QueryRow(ctx, `
		SELECT EXISTS (
			SELECT 1 FROM staff_role_permissions srp
			JOIN staff_roles sr ON sr.id = srp.role_id
			WHERE sr.code = 'admin' AND srp.permission = 'staff.permissions.manage'
		)
	`).Scan(&adminPresetHas)
	if err != nil {
		panic(err)
	}

	if !ownerPresetHas {
		panic("ASSERTION FAILED: 'owner' role template must have 'staff.permissions.manage'")
	}
	if !coOwnerPresetHas {
		panic("ASSERTION FAILED: 'co_owner' role template must have 'staff.permissions.manage'")
	}
	if adminPresetHas {
		panic("ASSERTION FAILED: 'admin' role template must NOT have 'staff.permissions.manage'")
	}
	fmt.Println("Role templates assertion PASS: owner=PRESENT, co_owner=PRESENT, admin=ABSENT")

	// 2. Direct Member Permissions Assertions
	hasDirectPerm := func(userID uuid.UUID) bool {
		var exists bool
		err := conn.QueryRow(ctx, `
			SELECT EXISTS (
				SELECT 1 FROM staff_member_permissions
				WHERE user_id = $1 AND permission = 'staff.permissions.manage'
			)
		`, userID).Scan(&exists)
		if err != nil {
			panic(err)
		}
		return exists
	}

	ownerDirect := hasDirectPerm(ownerUserID)
	coOwnerDirect := hasDirectPerm(coOwnerUserID)
	adminDirect := hasDirectPerm(adminUserID)

	if !ownerDirect {
		panic("ASSERTION FAILED: Existing owner user must have gained direct 'staff.permissions.manage'")
	}
	if !coOwnerDirect {
		panic("ASSERTION FAILED: Existing co_owner user must have gained direct 'staff.permissions.manage'")
	}
	if adminDirect {
		panic("ASSERTION FAILED: Existing admin user must NOT have gained direct 'staff.permissions.manage'")
	}
	fmt.Println("Direct permissions assertion PASS: existing_owner=PRESENT, existing_co_owner=PRESENT, existing_admin=ABSENT")

	fmt.Println("SUCCESS: All migration 000085 assertions passed.")
}
GOEOF
go run /tmp/assert_backfill_85.go "$TEST_DB_URL"

# 7. Safe cleanup: restore clean test database with repository latest migration
restore_clean_zamk_test
CLEANUP_DONE=1

echo "=== EMP.1B2 Migration Acceptance Script COMPLETE: SUCCESS ==="
