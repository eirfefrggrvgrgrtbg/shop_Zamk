package config

import (
	"os"
	"strconv"
	"strings"
)

type Config struct {
	App       AppConfig
	Postgres  PostgresConfig
	Redis     RedisConfig
	JWT       JWTConfig
	Auth      AuthConfig
	S3        S3Config
	CORS      CORSConfig
	TBank     TBankConfig
	Worker    WorkerConfig
	RateLimit RateLimitConfig
}

type AppConfig struct {
	Env           string
	Port          string
	PublicBaseURL string
	APIBaseURL    string
}

type PostgresConfig struct {
	Host     string
	Port     string
	User     string
	Password string
	Database string
	SSLMode  string
	DSN      string
}

type RedisConfig struct {
	Host     string
	Port     string
	Password string
	DB       int
	Addr     string
}

type JWTConfig struct {
	AccessTokenSecret     string
	RefreshTokenSecret    string
	AccessTokenTTLMinutes int
	RefreshTokenTTLDays   int
}

type AuthConfig struct {
	CookieDomain   string
	CookieSecure   bool
	CookieSameSite string
}

type S3Config struct {
	Endpoint        string
	Port            string
	Region          string
	Bucket          string
	AccessKey       string
	SecretKey       string
	UseSSL          bool
	PublicBaseURL   string
	UploadMaxSizeMB int
}

type CORSConfig struct {
	AllowedOrigins []string
}

type TBankConfig struct {
	TerminalKey string
	Password    string
	APIBaseURL  string
	SuccessURL  string
	FailURL     string
}

type WorkerConfig struct {
	OrderExpirationIntervalSeconds int
	OrderPaymentTimeoutMinutes     int
	ReturnWindowDays               int
	SellerBalanceIntervalSeconds   int
	MarketplaceCommissionBPS       int
}

type RateLimitConfig struct {
	Enabled                        bool
	FailOpenLocal                  bool
	FailOpenOnRedisError           bool
	AuthLoginLimitPerMinute        int
	AuthRegisterLimitPerHour       int
	AuthRefreshLimitPerMinute      int
	AuthChangePasswordLimitPerHour int
	UploadLimitPerMinute           int
	WebhookLimitPerMinute          int
	AdminDangerousLimitPerMinute   int
}

func Load() (*Config, error) {
	cfg := &Config{
		App: AppConfig{
			Env:           getEnv("APP_ENV", "local"),
			Port:          getEnv("APP_PORT", "8080"),
			PublicBaseURL: getEnv("PUBLIC_BASE_URL", "http://localhost:5173"),
			APIBaseURL:    getEnv("API_BASE_URL", "http://localhost:8080"),
		},
		Postgres: PostgresConfig{
			Host:     getEnv("POSTGRES_HOST", "localhost"),
			Port:     getEnv("POSTGRES_PORT", "5432"),
			User:     getEnv("POSTGRES_USER", "zamk"),
			Password: getEnv("POSTGRES_PASSWORD", "zamk_password"),
			Database: getEnv("POSTGRES_DB", "zamk"),
			SSLMode:  getEnv("POSTGRES_SSLMODE", "disable"),
		},
		Redis: RedisConfig{
			Host:     getEnv("REDIS_HOST", "localhost"),
			Port:     getEnv("REDIS_PORT", "6379"),
			Password: getEnv("REDIS_PASSWORD", ""),
			DB:       getEnvAsInt("REDIS_DB", 0),
		},
		JWT: JWTConfig{
			AccessTokenSecret:     getEnv("JWT_ACCESS_SECRET", "secret"),
			RefreshTokenSecret:    getEnv("JWT_REFRESH_SECRET", "secret"),
			AccessTokenTTLMinutes: getEnvAsInt("JWT_ACCESS_TTL_MINUTES", 15),
			RefreshTokenTTLDays:   getEnvAsInt("JWT_REFRESH_TTL_DAYS", 30),
		},
		Auth: AuthConfig{
			CookieDomain:   getEnv("AUTH_COOKIE_DOMAIN", ""),
			CookieSecure:   getEnvAsBool("AUTH_COOKIE_SECURE", false),
			CookieSameSite: getEnv("AUTH_COOKIE_SAMESITE", "Lax"),
		},
		S3: S3Config{
			Endpoint:        getEnv("S3_ENDPOINT", "localhost"),
			Port:            getEnv("S3_PORT", "9000"),
			Region:          getEnv("S3_REGION", ""),
			Bucket:          getEnv("S3_BUCKET", "zamk-products"),
			AccessKey:       getEnv("S3_ACCESS_KEY", "zamk_minio"),
			SecretKey:       getEnv("S3_SECRET_KEY", "zamk_minio_password"),
			UseSSL:          getEnvAsBool("S3_USE_SSL", false),
			PublicBaseURL:   getEnv("S3_PUBLIC_BASE_URL", "http://localhost:9000/zamk-products"),
			UploadMaxSizeMB: getEnvAsInt("S3_UPLOAD_MAX_SIZE_MB", 10),
		},
		CORS: CORSConfig{
			AllowedOrigins: splitCSV(getEnv("CORS_ALLOWED_ORIGINS", "http://localhost:5173,http://localhost:5174,http://localhost:5175")),
		},
		TBank: TBankConfig{
			TerminalKey: getEnv("TBANK_TERMINAL_KEY", "STUB"),
			Password:    getEnv("TBANK_PASSWORD", "STUB"),
			APIBaseURL:  getEnv("TBANK_API_BASE_URL", "https://securepay.tinkoff.ru/v2"),
			SuccessURL:  getEnv("TBANK_SUCCESS_URL", "http://localhost:3000/checkout/success"),
			FailURL:     getEnv("TBANK_FAIL_URL", "http://localhost:3000/checkout/fail"),
		},
		Worker: WorkerConfig{
			OrderExpirationIntervalSeconds: getEnvAsInt("WORKER_ORDER_EXPIRATION_INTERVAL_SECONDS", 60),
			OrderPaymentTimeoutMinutes:     getEnvAsInt("ORDER_PAYMENT_TIMEOUT_MINUTES", 30),
			ReturnWindowDays:               getEnvAsInt("RETURN_WINDOW_DAYS", 14),
			SellerBalanceIntervalSeconds:   getEnvAsInt("WORKER_SELLER_BALANCE_INTERVAL_SECONDS", 300),
			MarketplaceCommissionBPS:       getEnvAsInt("MARKETPLACE_COMMISSION_BPS", 1500),
		},
		RateLimit: RateLimitConfig{
			Enabled:                        getEnvAsBool("RATE_LIMIT_ENABLED", true),
			FailOpenLocal:                  getEnvAsBool("RATE_LIMIT_FAIL_OPEN_LOCAL", true),
			AuthLoginLimitPerMinute:        getEnvAsInt("AUTH_LOGIN_LIMIT_PER_MINUTE", 5),
			AuthRegisterLimitPerHour:       getEnvAsInt("AUTH_REGISTER_LIMIT_PER_HOUR", 10),
			AuthRefreshLimitPerMinute:      getEnvAsInt("AUTH_REFRESH_LIMIT_PER_MINUTE", 30),
			AuthChangePasswordLimitPerHour: getEnvAsInt("AUTH_CHANGE_PASSWORD_LIMIT_PER_HOUR", 5),
			UploadLimitPerMinute:           getEnvAsInt("UPLOAD_LIMIT_PER_MINUTE", 10),
			WebhookLimitPerMinute:          getEnvAsInt("WEBHOOK_LIMIT_PER_MINUTE", 120),
			AdminDangerousLimitPerMinute:   getEnvAsInt("ADMIN_DANGEROUS_ACTION_LIMIT_PER_MINUTE", 30),
		},
	}

	cfg.Postgres.DSN = getEnvNonEmpty("POSTGRES_DSN", "postgres://"+cfg.Postgres.User+":"+cfg.Postgres.Password+"@"+cfg.Postgres.Host+":"+cfg.Postgres.Port+"/"+cfg.Postgres.Database+"?sslmode="+cfg.Postgres.SSLMode)
	cfg.Redis.Addr = getEnvNonEmpty("REDIS_ADDR", cfg.Redis.Host+":"+cfg.Redis.Port)
	cfg.RateLimit.FailOpenOnRedisError = cfg.App.Env == "local" && cfg.RateLimit.FailOpenLocal

	return cfg, nil
}

func getEnv(key, defaultVal string) string {
	if val, ok := os.LookupEnv(key); ok {
		return val
	}
	return defaultVal
}

func getEnvNonEmpty(key, defaultVal string) string {
	if val, ok := os.LookupEnv(key); ok && strings.TrimSpace(val) != "" {
		return val
	}
	return defaultVal
}

func getEnvAsInt(key string, defaultVal int) int {
	if valStr, ok := os.LookupEnv(key); ok {
		if val, err := strconv.Atoi(valStr); err == nil {
			return val
		}
	}
	return defaultVal
}

func getEnvAsBool(key string, defaultVal bool) bool {
	if valStr, ok := os.LookupEnv(key); ok {
		if val, err := strconv.ParseBool(valStr); err == nil {
			return val
		}
	}
	return defaultVal
}

func splitCSV(value string) []string {
	parts := strings.Split(value, ",")
	result := make([]string, 0, len(parts))
	for _, part := range parts {
		trimmed := strings.TrimSpace(part)
		if trimmed != "" {
			result = append(result, trimmed)
		}
	}
	return result
}
