package config

import (
	"os"
	"strconv"
	"time"
)

// Config holds all the configuration variables loaded from the environment.
type Config struct {
	Port         string
	UpstreamURL  string
	DatabaseURL  string
	RedisAddress string
	SyncInterval         time.Duration
	LogLevel             string
	OtelExporterEndpoint string
}

// Load reads config variables from the environment with sensible defaults.
func Load() *Config {
	return &Config{
		Port:                 getEnv("PORT", "8080"),
		UpstreamURL:          getEnv("UPSTREAM_URL", "http://localhost:8081"),
		DatabaseURL:          getEnv("DATABASE_URL", "postgres://postgres:postgres@localhost:5432/drlap?sslmode=disable"),
		RedisAddress:         getEnv("REDIS_ADDRESS", "localhost:6379"),
		SyncInterval:         getDurationEnv("SYNC_INTERVAL_MS", 250*time.Millisecond),
		LogLevel:             getEnv("LOG_LEVEL", "info"),
		OtelExporterEndpoint: getEnv("OTEL_EXPORTER_OTLP_ENDPOINT", ""),
	}
}

func getEnv(key, defaultValue string) string {
	if value, exists := os.LookupEnv(key); exists {
		return value
	}
	return defaultValue
}

func getDurationEnv(key string, defaultValue time.Duration) time.Duration {
	if val, exists := os.LookupEnv(key); exists {
		if ms, err := strconv.Atoi(val); err == nil {
			return time.Duration(ms) * time.Millisecond
		}
	}
	return defaultValue
}
