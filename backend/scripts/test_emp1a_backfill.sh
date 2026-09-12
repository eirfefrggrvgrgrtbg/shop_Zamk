#!/bin/bash
set -e

# Canonical test database URL
TEST_DB_URL=${TEST_DATABASE_URL:-"postgres://zamk:zamk_password@localhost:5433/zamk_test?sslmode=disable"}
REQUIRED_DB_NAME="zamk_test"

# Explicit non-floating golang-migrate version
MIGRATE_PKG="github.com/golang-migrate/migrate/v4/cmd/migrate@v4.17.0"

echo "=== EMP.1A1 Migration Acceptance Script ==="

# Dynamically derive current repository latest migration version
get_latest_migration_version() {
	ls migrations/*_*.up.sql | sed -E 's/.*\/([0-9]+)_.*/\1/' | sort -n | tail -n 1 | sed 's/^0*//'
}

# Exact DB guard: must fail closed if connection fails or database is not strictly zamk_test
verify_exact_database() {
	cat << 'GOEOF' > /tmp/verify_exact_test_db.go
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
	go run /tmp/verify_exact_test_db.go "$TEST_DB_URL" "$REQUIRED_DB_NAME"
}

# Safely drop and recreate strictly zamk_test
recreate_test_database() {
	cat << 'GOEOF' > /tmp/recreate_exact_test_db.go
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
	go run /tmp/recreate_exact_test_db.go "$TEST_DB_URL"
}

# Verify schema_migrations is clean at expected version
verify_migration_status() {
	cat << 'GOEOF' > /tmp/verify_clean_mig_status.go
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
	go run /tmp/verify_clean_mig_status.go "$TEST_DB_URL" "$1"
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

# 3. Establish schema through 000083
echo "Migrating up to version 000083..."
go run -tags 'postgres' "$MIGRATE_PKG" -path migrations -database "$TEST_DB_URL" goto 83
verify_migration_status 83

# 4. Seed exact fixture rows at schema 83
echo "Seeding exact fixture rows at schema 83 (before 000084)..."
cat << 'GOEOF' > /tmp/seed_fixtures_83.go
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

	// Ensure staff_member_permissions does NOT exist at schema 83
	var exists bool
	err = conn.QueryRow(ctx, `
		SELECT EXISTS (
			SELECT 1 FROM information_schema.tables
			WHERE table_name = 'staff_member_permissions'
		)
	`).Scan(&exists)
	if err != nil {
		panic(err)
	}
	if exists {
		panic("Invalid state: staff_member_permissions already exists at version 83")
	}

	// Roles
	roleAID := uuid.MustParse("11111111-1111-1111-1111-111111111111")
	roleBID := uuid.MustParse("22222222-2222-2222-2222-222222222222")
	roleEmptyID := uuid.MustParse("33333333-3333-3333-3333-333333333333")

	_, err = conn.Exec(ctx, `
		INSERT INTO staff_roles (id, code, name, created_at, updated_at) VALUES
		($1, 'role_a', 'Role A', NOW(), NOW()),
		($2, 'role_b', 'Role B', NOW(), NOW()),
		($3, 'role_empty', 'Role Empty', NOW(), NOW())
	`, roleAID, roleBID, roleEmptyID)
	if err != nil {
		panic(err)
	}

	// Permissions for Role A (perm.a, perm.b) and Role B (perm.c)
	_, err = conn.Exec(ctx, `
		INSERT INTO staff_role_permissions (role_id, permission, created_at) VALUES
		($1, 'perm.a', NOW()),
		($1, 'perm.b', NOW()),
		($2, 'perm.c', NOW())
	`, roleAID, roleBID)
	if err != nil {
		panic(err)
	}

	// Staff Users A, B, C
	userAID := uuid.MustParse("aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa")
	userBID := uuid.MustParse("bbbbbbbb-bbbb-bbbb-bbbb-bbbbbbbbbbbb")
	userCID := uuid.MustParse("cccccccc-cccc-cccc-cccc-cccccccccccc")

	_, err = conn.Exec(ctx, `
		INSERT INTO users (id, phone, name, email, password_hash, created_at, updated_at) VALUES
		($1, '+79991111111', 'Staff A', 'staff_a@zamk.test', 'hash', NOW(), NOW()),
		($2, '+79992222222', 'Staff B', 'staff_b@zamk.test', 'hash', NOW(), NOW()),
		($3, '+79993333333', 'Staff C', 'staff_c@zamk.test', 'hash', NOW(), NOW())
	`, userAID, userBID, userCID)
	if err != nil {
		panic(err)
	}

	// Staff members: Staff A -> Role A, Staff B -> Role B, Staff C -> Role Empty
	_, err = conn.Exec(ctx, `
		INSERT INTO staff_members (user_id, staff_role_id, status, created_at, updated_at) VALUES
		($1, $4, 'active', NOW(), NOW()),
		($2, $5, 'active', NOW(), NOW()),
		($3, $6, 'active', NOW(), NOW())
	`, userAID, userBID, userCID, roleAID, roleBID, roleEmptyID)
	if err != nil {
		panic(err)
	}

	fmt.Println("Seeded Staff A (Role A), Staff B (Role B), Staff C (Role Empty) at schema 83.")
}
GOEOF
go run /tmp/seed_fixtures_83.go "$TEST_DB_URL"

# Test failure hook for failure-path verification
if [ -n "$TEST_SIMULATE_FAILURE" ]; then
	echo "TEST_SIMULATE_FAILURE is set: triggering intentional failure for failure-path proof."
	exit 42
fi

# 5. Apply ACTUAL 000084 UP migration via real migration runner
echo "Running ACTUAL 000084 UP migration via golang-migrate..."
go run -tags 'postgres' "$MIGRATE_PKG" -path migrations -database "$TEST_DB_URL" up 1
verify_migration_status 84

# 6. Assert resulting rows in staff_member_permissions
echo "Asserting backfill results after 000084..."
cat << 'GOEOF' > /tmp/assert_backfill_84.go
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

	userAID := uuid.MustParse("aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa")
	userBID := uuid.MustParse("bbbbbbbb-bbbb-bbbb-bbbb-bbbbbbbbbbbb")
	userCID := uuid.MustParse("cccccccc-cccc-cccc-cccc-cccccccccccc")

	getPerms := func(userID uuid.UUID) []string {
		rows, err := conn.Query(ctx, "SELECT permission FROM staff_member_permissions WHERE user_id = $1 ORDER BY permission", userID)
		if err != nil {
			panic(err)
		}
		defer rows.Close()

		var perms []string
		for rows.Next() {
			var p string
			if err := rows.Scan(&p); err != nil {
				panic(err)
			}
			perms = append(perms, p)
		}
		return perms
	}

	permsA := getPerms(userAID)
	permsB := getPerms(userBID)
	permsC := getPerms(userCID)

	fmt.Printf("Staff A direct permissions: %v\n", permsA)
	fmt.Printf("Staff B direct permissions: %v\n", permsB)
	fmt.Printf("Staff C direct permissions: %v\n", permsC)

	// Assert Staff A direct permissions == {perm.a, perm.b}
	if len(permsA) != 2 || permsA[0] != "perm.a" || permsA[1] != "perm.b" {
		panic(fmt.Sprintf("ASSERTION FAILED: Staff A expected [perm.a, perm.b], got %v", permsA))
	}

	// Assert Staff B direct permissions == {perm.c}
	if len(permsB) != 1 || permsB[0] != "perm.c" {
		panic(fmt.Sprintf("ASSERTION FAILED: Staff B expected [perm.c], got %v", permsB))
	}

	// Assert Staff C direct permissions == {}
	if len(permsC) != 0 {
		panic(fmt.Sprintf("ASSERTION FAILED: Staff C expected [], got %v", permsC))
	}

	// Assert total count in staff_member_permissions equals 3
	var totalCount int
	err = conn.QueryRow(ctx, "SELECT count(*) FROM staff_member_permissions").Scan(&totalCount)
	if err != nil {
		panic(err)
	}
	if totalCount != 3 {
		panic(fmt.Sprintf("ASSERTION FAILED: expected 3 rows in staff_member_permissions, got %d", totalCount))
	}

	// Verify foreign key references staff_members(user_id)
	var fkTargetTable string
	err = conn.QueryRow(ctx, `
		SELECT ccu.table_name
		FROM information_schema.table_constraints tc
		JOIN information_schema.constraint_column_usage ccu ON ccu.constraint_name = tc.constraint_name
		WHERE tc.table_name = 'staff_member_permissions'
		  AND tc.constraint_type = 'FOREIGN KEY'
	`).Scan(&fkTargetTable)
	if err != nil {
		panic(fmt.Sprintf("FK check error: %v", err))
	}
	if fkTargetTable != "staff_members" {
		panic(fmt.Sprintf("ASSERTION FAILED: expected FK target table 'staff_members', got %q", fkTargetTable))
	}

	fmt.Println("SUCCESS: All migration 000084 backfill assertions passed.")
}
GOEOF
go run /tmp/assert_backfill_84.go "$TEST_DB_URL"

# 7. Safe cleanup: restore clean test database with repository latest migration
restore_clean_zamk_test
CLEANUP_DONE=1

echo "=== EMP.1A1 Migration Acceptance Script COMPLETE: SUCCESS ==="
