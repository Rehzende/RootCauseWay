package config

import (
	"fmt"
	"os"
	"strconv"

	"github.com/joho/godotenv"
)

// Config holds application configuration loaded from environment variables.
type Config struct {
	DBHost    string
	DBPort    string
	DBUser    string
	DBPass    string
	DBName    string
	RedisURL  string
	JWTSecret string
	Port      string

	// EventStreamName is the Redis Stream events are durably XADDed to
	// (dual-written alongside pub/sub). See contracts/events/redis-events.yaml.
	EventStreamName string
	// EventStreamMaxLen bounds the stream via approximate MAXLEN trimming.
	EventStreamMaxLen int64

	// EncryptionKey is the base64-encoded 32-byte AES-256 key used to encrypt
	// credentials at rest (ROOTCAUSEWAY_ENCRYPTION_KEY). Optional: when empty,
	// encryption at rest is disabled. Generate with: openssl rand -base64 32
	EncryptionKey string

	// Embeddings (semantic search). Empty EmbeddingsAPIBase disables embeddings
	// and all vector-search paths fall back to ILIKE matching.
	EmbeddingsAPIBase string
	EmbeddingsAPIKey  string
	EmbeddingsModel   string
}

// Load reads configuration from environment variables, with optional .env file.
func Load() (*Config, error) {
	_ = godotenv.Load()

	cfg := &Config{
		DBHost:    getEnv("DB_HOST", "localhost"),
		DBPort:    getEnv("DB_PORT", "5432"),
		DBUser:    getEnv("DB_USER", "rootcauseway"),
		DBPass:    getEnv("DB_PASS", "rootcauseway"),
		DBName:    getEnv("DB_NAME", "rootcauseway"),
		RedisURL:  getEnv("REDIS_URL", "redis://localhost:6379"),
		JWTSecret: getEnv("JWT_SECRET", ""),
		Port:      getEnv("PORT", "8080"),

		EventStreamName:   getEnv("EVENT_STREAM_NAME", "rootcauseway:events"),
		EventStreamMaxLen: getEnvInt64("EVENT_STREAM_MAXLEN", 100000),

		EncryptionKey: getEnv("ROOTCAUSEWAY_ENCRYPTION_KEY", ""),

		EmbeddingsAPIBase: getEnv("EMBEDDINGS_API_BASE", ""),
		EmbeddingsAPIKey:  getEnv("EMBEDDINGS_API_KEY", ""),
		EmbeddingsModel:   getEnv("EMBEDDINGS_MODEL", "text-embedding-3-small"),
	}

	if cfg.JWTSecret == "" {
		return nil, fmt.Errorf("JWT_SECRET is required")
	}

	return cfg, nil
}

// DatabaseURL returns the PostgreSQL connection string.
func (c *Config) DatabaseURL() string {
	return fmt.Sprintf("postgres://%s:%s@%s:%s/%s?sslmode=disable", c.DBUser, c.DBPass, c.DBHost, c.DBPort, c.DBName)
}

func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func getEnvInt64(key string, fallback int64) int64 {
	if v := os.Getenv(key); v != "" {
		if n, err := strconv.ParseInt(v, 10, 64); err == nil {
			return n
		}
	}
	return fallback
}
