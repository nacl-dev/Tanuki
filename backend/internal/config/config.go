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
	InboxPath      string

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

	// Auto-tagging (v0.4)
	SauceNAOAPIKey             string
	IQDBEnabled                bool
	AutoTagSimilarityThreshold int // percentage 0-100
	AutoTagOnScan              bool
	AutoTagRateLimitMs         int // milliseconds between API requests

	// Duplicate detection (v0.5)
	DuplicateThreshold int  // max Hamming distance to consider duplicate
	PHashOnScan        bool // compute pHash during scan

	// Authentication (v0.6)
	JWTSecret           string
	JWTExpiryHours      int
	RegistrationEnabled bool

	// Plugins (v1.0)
	PluginsPath    string
	PluginsEnabled bool
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
		InboxPath:              getEnv("INBOX_PATH", "/inbox"),
		SecretKey:              getEnv("SECRET_KEY", "change-me"),
		Port:                   getEnv("PORT", "8080"),
		LogLevel:               getEnv("LOG_LEVEL", "info"),
		ScanInterval:           getEnvInt("SCAN_INTERVAL", 300),
		MaxConcurrentDownloads: getEnvInt("MAX_CONCURRENT_DOWNLOADS", 3),
		RateLimitDelay:         getEnvInt("RATE_LIMIT_DELAY", 1000),

		// Auto-tagging (v0.4)
		SauceNAOAPIKey:             getEnv("SAUCENAO_API_KEY", ""),
		IQDBEnabled:                getEnvBool("IQDB_ENABLED", true),
		AutoTagSimilarityThreshold: getEnvInt("AUTOTAG_SIMILARITY_THRESHOLD", 80),
		AutoTagOnScan:              getEnvBool("AUTOTAG_ON_SCAN", false),
		AutoTagRateLimitMs:         getEnvInt("AUTOTAG_RATE_LIMIT_MS", 5000),

		// Duplicate detection (v0.5)
		DuplicateThreshold: getEnvInt("DUPLICATE_THRESHOLD", 10),
		PHashOnScan:        getEnvBool("PHASH_ON_SCAN", true),

		// Authentication (v0.6)
		JWTSecret:           getEnv("JWT_SECRET", getEnv("SECRET_KEY", "change-me")),
		JWTExpiryHours:      getEnvInt("JWT_EXPIRY_HOURS", 24),
		RegistrationEnabled: getEnvBool("REGISTRATION_ENABLED", true),

		// Plugins (v1.0)
		PluginsPath:    getEnv("PLUGINS_PATH", "/app/config/plugins"),
		PluginsEnabled: getEnvBool("PLUGINS_ENABLED", true),
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

func getEnvBool(key string, fallback bool) bool {
	v := os.Getenv(key)
	if v == "" {
		return fallback
	}
	b, err := strconv.ParseBool(v)
	if err != nil {
		return fallback
	}
	return b
}
