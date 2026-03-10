// Command worker processes background jobs: media scanning, thumbnails, hashing.
package main

import (
	"context"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/nacl-dev/tanuki/internal/config"
	"github.com/nacl-dev/tanuki/internal/database"
	"github.com/nacl-dev/tanuki/internal/models"
	"github.com/nacl-dev/tanuki/internal/scanner"
	"github.com/nacl-dev/tanuki/internal/thumbnails"
	"go.uber.org/zap"
)

func main() {
	log, _ := zap.NewProduction()
	defer log.Sync() //nolint:errcheck

	cfg, err := config.Load()
	if err != nil {
		log.Fatal("load config", zap.Error(err))
	}

	db, err := database.Connect(cfg.DatabaseURL)
	if err != nil {
		log.Fatal("connect database", zap.Error(err))
	}
	defer db.Close()

	// Ensure schema is up to date.
	if err := db.Migrate(); err != nil {
		log.Fatal("migrate", zap.Error(err))
	}

	ctx, cancel := context.WithCancel(context.Background())

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-quit
		log.Info("worker: shutting down")
		cancel()
	}()

	sc := scanner.New(db, cfg.MediaPath, log)
	gen := thumbnails.New(cfg.ThumbnailsPath, log)

	ticker := time.NewTicker(time.Duration(cfg.ScanInterval) * time.Second)
	defer ticker.Stop()

	// Run an initial scan immediately on start.
	if err := sc.Run(ctx); err != nil && ctx.Err() == nil {
		log.Error("initial scan failed", zap.Error(err))
	}
	generateMissingThumbnails(ctx, db, gen, log)

	for {
		select {
		case <-ctx.Done():
			log.Info("worker: stopped")
			return
		case <-ticker.C:
			if err := sc.Run(ctx); err != nil && ctx.Err() == nil {
				log.Error("periodic scan failed", zap.Error(err))
			}
			generateMissingThumbnails(ctx, db, gen, log)
		}
	}
}

// generateMissingThumbnails creates thumbnails for all media records that do
// not yet have one.
func generateMissingThumbnails(ctx context.Context, db *database.DB, gen *thumbnails.Generator, log *zap.Logger) {
	var items []models.Media
	if err := db.SelectContext(ctx, &items, `
		SELECT * FROM media
		WHERE deleted_at IS NULL AND (thumbnail_path = '' OR thumbnail_path IS NULL)
	`); err != nil {
		log.Error("worker: query media without thumbnails", zap.Error(err))
		return
	}

	total := len(items)
	for n, item := range items {
		if ctx.Err() != nil {
			return
		}
		log.Info("worker: generating thumbnail",
			zap.String("title", item.Title),
			zap.Int("n", n+1),
			zap.Int("total", total),
		)
		thumbPath, err := gen.GenerateForMedia(ctx, &item)
		if err != nil {
			log.Warn("worker: thumbnail generation failed",
				zap.String("title", item.Title),
				zap.Error(err),
			)
			continue
		}
		if _, err := db.ExecContext(ctx,
			`UPDATE media SET thumbnail_path = $1, updated_at = NOW() WHERE id = $2`,
			thumbPath, item.ID,
		); err != nil {
			log.Error("worker: update thumbnail_path", zap.String("id", item.ID), zap.Error(err))
		}
	}
}

