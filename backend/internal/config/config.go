// Package config loads application configuration from environment variables.
package config

import (
	"fmt"
	"os"
	"strconv"
)

// Config holds all runtime configuration for Tanuki.
type Config struct {
	// Database
	DatabaseURL string

	// Cache / queue
	RedisURL string

	// Filesystem paths
	MediaPath      string
	ThumbnailsPath string
	DownloadsPath  string

	// Security
	SecretKey string

	// HTTP server
	Port string

	// Logging
	LogLevel string

	// Scanner
	ScanInterval int // seconds

	// Download manager
	MaxConcurrentDownloads int
	RateLimitDelay         int // milliseconds
}

// Load reads configuration from environment variables, applying defaults where
// values are absent.
func Load() (*Config, error) {
	cfg := &Config{
		DatabaseURL:            getEnv("DATABASE_URL", "postgresql://tanuki:secret@localhost:5432/tanuki"),
		RedisURL:               getEnv("REDIS_URL", "redis://localhost:6379"),
		MediaPath:              getEnv("MEDIA_PATH", "/media"),
		ThumbnailsPath:         getEnv("THUMBNAILS_PATH", "/thumbnails"),
		DownloadsPath:          getEnv("DOWNLOADS_PATH", "/downloads"),
		SecretKey:              getEnv("SECRET_KEY", "change-me"),
		Port:                   getEnv("PORT", "8080"),
		LogLevel:               getEnv("LOG_LEVEL", "info"),
		ScanInterval:           getEnvInt("SCAN_INTERVAL", 300),
		MaxConcurrentDownloads: getEnvInt("MAX_CONCURRENT_DOWNLOADS", 3),
		RateLimitDelay:         getEnvInt("RATE_LIMIT_DELAY", 1000),
	}

	if err := cfg.validate(); err != nil {
		return nil, err
	}

	return cfg, nil
}

func (c *Config) validate() error {
	if c.DatabaseURL == "" {
		return fmt.Errorf("DATABASE_URL is required")
	}
	if c.SecretKey == "change-me" {
		// Allow in development but warn (caller should log this).
		_ = c.SecretKey
	}
	return nil
}

// ServerAddr returns the TCP address string for net/http.
func (c *Config) ServerAddr() string {
	return ":" + c.Port
}

// ─── helpers ─────────────────────────────────────────────────────────────────

func getEnv(key, fallback string) string {
	if v, ok := os.LookupEnv(key); ok {
		return v
	}
	return fallback
}

func getEnvInt(key string, fallback int) int {
	v := os.Getenv(key)
	if v == "" {
		return fallback
	}
	n, err := strconv.Atoi(v)
	if err != nil {
		return fallback
	}
	return n
}
