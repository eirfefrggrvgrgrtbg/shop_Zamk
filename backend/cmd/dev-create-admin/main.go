package main

import (
	"context"
	"log/slog"
	"os"
	"time"

	"github.com/eirfefrggrvgrgrtbg/shop-zamk/backend/internal/auth"
	"github.com/eirfefrggrvgrgrtbg/shop-zamk/backend/internal/config"
	"github.com/eirfefrggrvgrgrtbg/shop-zamk/backend/internal/platform/postgres"
	"github.com/eirfefrggrvgrgrtbg/shop-zamk/backend/internal/users"
	"github.com/google/uuid"
	"github.com/joho/godotenv"
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

	adminEmail := os.Getenv("ADMIN_EMAIL")
	adminPassword := os.Getenv("ADMIN_PASSWORD")
	adminName := os.Getenv("ADMIN_NAME")

	if adminEmail == "" || adminPassword == "" || adminName == "" {
		logger.Error("ADMIN_EMAIL, ADMIN_PASSWORD, and ADMIN_NAME must be set in the environment")
		os.Exit(1)
	}

	ctx := context.Background()

	pgClient, err := postgres.NewClient(ctx, cfg.Postgres.DSN)
	if err != nil {
		logger.Error("failed to connect to postgres", "error", err)
		os.Exit(1)
	}
	defer pgClient.Close()

	userRepo := users.NewRepository(pgClient.Pool)

	existingUser, err := userRepo.GetUserByEmail(ctx, adminEmail)
	if err == nil && existingUser != nil {
		logger.Info("admin user already exists with this email")
		os.Exit(0)
	}

	hashedPassword, err := auth.HashPassword(adminPassword)
	if err != nil {
		logger.Error("failed to hash password", "error", err)
		os.Exit(1)
	}

	user := &users.User{
		ID:           uuid.New(),
		Name:         adminName,
		Email:        adminEmail,
		PasswordHash: hashedPassword,
		Role:         users.RoleAdmin,
		Status:       users.StatusActive,
		CreatedAt:    time.Now(),
		UpdatedAt:    time.Now(),
	}

	if err := userRepo.CreateUser(ctx, user); err != nil {
		logger.Error("failed to create admin user", "error", err)
		os.Exit(1)
	}

	logger.Info("successfully created admin user", "id", user.ID, "email", user.Email)
}
