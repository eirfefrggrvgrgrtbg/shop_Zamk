package observability

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"regexp"
	"strconv"

	"github.com/jackc/pgx/v5/pgxpool"
	goredis "github.com/redis/go-redis/v9"
)

var migrationFileRegex = regexp.MustCompile(`^(\d+)_.+\.up\.sql$`)

// StartupDiagnosticsResult contains summary information from startup checks.
type StartupDiagnosticsResult struct {
	DatabaseName                 string
	MigrationCurrentVersion      int64
	MigrationDirty               bool
	ApplicationLatestMigrationVer int64
	PostgresConnected            bool
	RedisConnected               bool
}

// FindLatestApplicationMigration scans the migrations directory and finds the highest migration version.
func FindLatestApplicationMigration(migrationsDir string) (int64, error) {
	entries, err := os.ReadDir(migrationsDir)
	if err != nil {
		// Fallback check: maybe we are running inside cmd/api or root
		altDir := filepath.Join("..", migrationsDir)
		if altEntries, altErr := os.ReadDir(altDir); altErr == nil {
			entries = altEntries
			migrationsDir = altDir
		} else {
			return 0, fmt.Errorf("failed to read migrations directory %s: %w", migrationsDir, err)
		}
	}

	var latest int64
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		matches := migrationFileRegex.FindStringSubmatch(entry.Name())
		if len(matches) == 2 {
			ver, err := strconv.ParseInt(matches[1], 10, 64)
			if err == nil && ver > latest {
				latest = ver
			}
		}
	}
	return latest, nil
}

// RunStartupDiagnostics performs non-destructive connectivity and schema diagnostics on startup.
// Crucially, it NEVER logs credentials or full DSNs.
func RunStartupDiagnostics(
	ctx context.Context,
	pool *pgxpool.Pool,
	rdb *goredis.Client,
	databaseName string,
	redisAddr string,
	migrationsPath string,
	logger *slog.Logger,
) (*StartupDiagnosticsResult, error) {
	if logger == nil {
		logger = slog.Default()
	}

	result := &StartupDiagnosticsResult{
		DatabaseName: databaseName,
	}

	// 1. PostgreSQL connectivity check
	if pool != nil {
		if err := pool.Ping(ctx); err != nil {
			logger.Error("postgres connectivity check failed",
				"component", "postgres",
				"database_name", databaseName,
				"error", err.Error(),
			)
			return result, fmt.Errorf("postgres connectivity failed: %w", err)
		}
		result.PostgresConnected = true
		logger.Info("postgres connectivity ready",
			"component", "postgres",
			"database_name", databaseName,
		)
	}

	// 2. Redis connectivity check
	if rdb != nil {
		if err := rdb.Ping(ctx).Err(); err != nil {
			logger.Error("redis connectivity check failed",
				"component", "redis",
				"addr", redisAddr,
				"error", err.Error(),
			)
			return result, fmt.Errorf("redis connectivity failed: %w", err)
		}
		result.RedisConnected = true
		logger.Info("redis connectivity ready",
			"component", "redis",
			"addr", redisAddr,
		)
	}

	// 3. Schema and Migration State Check
	if pool != nil {
		var currentVer int64
		var dirty bool
		row := pool.QueryRow(ctx, "SELECT version, dirty FROM schema_migrations LIMIT 1")
		if err := row.Scan(&currentVer, &dirty); err != nil {
			logger.Warn("could not query schema_migrations table (table may not exist yet)",
				"component", "postgres",
				"database_name", databaseName,
				"error", err.Error(),
			)
		} else {
			result.MigrationCurrentVersion = currentVer
			result.MigrationDirty = dirty
		}

		latestVer, err := FindLatestApplicationMigration(migrationsPath)
		if err != nil {
			logger.Warn("could not determine latest application migration from disk",
				"component", "postgres",
				"migrations_path", migrationsPath,
				"error", err.Error(),
			)
		} else {
			result.ApplicationLatestMigrationVer = latestVer
		}

		if result.ApplicationLatestMigrationVer > 0 {
			if result.MigrationCurrentVersion < result.ApplicationLatestMigrationVer {
				logger.Error("database schema is behind application migrations",
					"component", "postgres",
					"database_name", databaseName,
					"migration_current_version", result.MigrationCurrentVersion,
					"migration_dirty", result.MigrationDirty,
					"application_latest_migration_version", result.ApplicationLatestMigrationVer,
				)
			} else if result.MigrationDirty {
				logger.Warn("database schema is in dirty state",
					"component", "postgres",
					"database_name", databaseName,
					"migration_current_version", result.MigrationCurrentVersion,
					"migration_dirty", result.MigrationDirty,
					"application_latest_migration_version", result.ApplicationLatestMigrationVer,
				)
			} else {
				logger.Info("database schema is up to date",
					"component", "postgres",
					"database_name", databaseName,
					"migration_current_version", result.MigrationCurrentVersion,
					"migration_dirty", result.MigrationDirty,
					"application_latest_migration_version", result.ApplicationLatestMigrationVer,
				)
			}
		}
	}

	return result, nil
}
