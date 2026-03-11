// Package database manages the PostgreSQL connection pool and schema migrations.
package database

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/jmoiron/sqlx"
	_ "github.com/lib/pq" // PostgreSQL driver
	"github.com/nacl-dev/tanuki/migrations"
)

// DB wraps sqlx.DB with convenience helpers.
type DB struct {
	*sqlx.DB
}

// Connect opens a connection pool to PostgreSQL and verifies it with a ping.
func Connect(databaseURL string) (*DB, error) {
	db, err := sqlx.Open("postgres", databaseURL)
	if err != nil {
		return nil, fmt.Errorf("open database: %w", err)
	}

	db.SetMaxOpenConns(25)
	db.SetMaxIdleConns(5)
	db.SetConnMaxLifetime(5 * time.Minute)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := db.PingContext(ctx); err != nil {
		return nil, fmt.Errorf("ping database: %w", err)
	}

	return &DB{DB: db}, nil
}

// Migrate runs all *.up.sql migration files in lexicographic order.
// It creates a simple schema_migrations table to track applied migrations.
func (d *DB) Migrate() error {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	tx, err := d.BeginTxx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin migration tx: %w", err)
	}
	defer tx.Rollback() //nolint:errcheck

	// Use a transaction-scoped advisory lock so app, worker and downloader
	// cannot apply migrations concurrently on first boot.
	if _, err := tx.ExecContext(ctx, `SELECT pg_advisory_xact_lock($1, $2)`, 0x54414E55, 0x4B49); err != nil {
		return fmt.Errorf("lock migrations: %w", err)
	}

	if _, err := tx.ExecContext(ctx, `
CREATE TABLE IF NOT EXISTS schema_migrations (
	version    TEXT        PRIMARY KEY,
	applied_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
)
`); err != nil {
		return fmt.Errorf("create migrations table: %w", err)
	}

	entries, err := migrations.FS.ReadDir(".")
	if err != nil {
		return fmt.Errorf("read migrations: %w", err)
	}

	for _, entry := range entries {
		name := entry.Name()
		if entry.IsDir() || !strings.HasSuffix(name, ".up.sql") {
			continue
		}

		version := strings.TrimSuffix(name, ".up.sql")

		var applied bool
		if err := tx.GetContext(ctx, &applied, `SELECT EXISTS(SELECT 1 FROM schema_migrations WHERE version = $1)`, version); err != nil {
			return fmt.Errorf("check migration %s: %w", version, err)
		}
		if applied {
			continue
		}

		data, err := migrations.FS.ReadFile(name)
		if err != nil {
			return fmt.Errorf("read migration %s: %w", name, err)
		}

		if _, err := tx.ExecContext(ctx, string(data)); err != nil {
			return fmt.Errorf("apply migration %s: %w", name, err)
		}

		if _, err := tx.ExecContext(ctx, `INSERT INTO schema_migrations (version) VALUES ($1)`, version); err != nil {
			return fmt.Errorf("record migration %s: %w", name, err)
		}
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit migrations: %w", err)
	}

	return nil
}
