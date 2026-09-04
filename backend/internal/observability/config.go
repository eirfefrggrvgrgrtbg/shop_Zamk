package observability

import (
	"os"
	"strconv"
)

// Config holds settings for backend observability.
type Config struct {
	ServiceName    string
	Environment    string
	ServiceVersion string
	OTLPEndpoint   string
	OTLPInsecure   bool
	Enabled        bool
}

// DefaultConfig returns default observability configuration.
func DefaultConfig() Config {
	return Config{
		ServiceName:    getEnv("OTEL_SERVICE_NAME", "zamk-api"),
		Environment:    getEnv("ZAMK_ENVIRONMENT", getEnv("APP_ENV", "local")),
		ServiceVersion: getEnv("SERVICE_VERSION", "1.0.0"),
		OTLPEndpoint:   getEnv("OTEL_EXPORTER_OTLP_ENDPOINT", "localhost:4317"),
		OTLPInsecure:   getEnvAsBool("OTEL_EXPORTER_OTLP_INSECURE", true),
		Enabled:        getEnvAsBool("OBSERVABILITY_ENABLED", true),
	}
}

func getEnv(key, defaultVal string) string {
	if val, ok := os.LookupEnv(key); ok && val != "" {
		return val
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
