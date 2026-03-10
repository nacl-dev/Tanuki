// Command worker processes background jobs: media scanning, thumbnails, hashing.
package main

import (
	"context"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/nacl-dev/tanuki/internal/autotag"
	"github.com/nacl-dev/tanuki/internal/config"
	"github.com/nacl-dev/tanuki/internal/database"
	"github.com/nacl-dev/tanuki/internal/dedup"
	"github.com/nacl-dev/tanuki/internal/models"
	"github.com/nacl-dev/tanuki/internal/plugins"
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

	sc := scanner.New(db, cfg.MediaPath, cfg.ThumbnailsPath, log)
	gen := thumbnails.New(cfg.ThumbnailsPath, log)

	// Services for v0.4 / v0.5
	dedupSvc := dedup.NewService(db, cfg.DuplicateThreshold, log)
	autotagSvc := autotag.NewService(db, autotag.Config{
		SauceNAOAPIKey: cfg.SauceNAOAPIKey,
		IQDBEnabled:    cfg.IQDBEnabled,
		Threshold:      float64(cfg.AutoTagSimilarityThreshold),
		RateLimitMs:    cfg.AutoTagRateLimitMs,
	}, log)

	// Plugin registry (v1.0)
	if cfg.PluginsEnabled {
		pluginRegistry := plugins.NewRegistry(db, cfg.PluginsPath, log)
		if err := pluginRegistry.LoadAll(); err != nil {
			log.Warn("worker: plugin load failed", zap.Error(err))
		}
		_ = pluginRegistry // available for future metadata-fetch hooks
	}

	ticker := time.NewTicker(time.Duration(cfg.ScanInterval) * time.Second)
	defer ticker.Stop()

	// Run an initial scan immediately on start.
	if err := sc.Run(ctx); err != nil && ctx.Err() == nil {
		log.Error("initial scan failed", zap.Error(err))
	}
	generateMissingThumbnails(ctx, db, gen, log)
	if cfg.PHashOnScan {
		computeMissingPHashes(ctx, db, dedupSvc, log)
	}
	if cfg.AutoTagOnScan {
		autoTagUntagged(ctx, db, autotagSvc, log)
	}

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
			if cfg.PHashOnScan {
				computeMissingPHashes(ctx, db, dedupSvc, log)
			}
			if cfg.AutoTagOnScan {
				autoTagUntagged(ctx, db, autotagSvc, log)
			}
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

// computeMissingPHashes calculates perceptual hashes for media items that don't have one.
func computeMissingPHashes(ctx context.Context, db *database.DB, svc *dedup.Service, log *zap.Logger) {
	var items []models.Media
	if err := db.SelectContext(ctx, &items, `
		SELECT * FROM media
		WHERE deleted_at IS NULL AND phash IS NULL
		ORDER BY created_at DESC
		LIMIT 500
	`); err != nil {
		log.Error("worker: query media without phash", zap.Error(err))
		return
	}

	total := len(items)
	for n, item := range items {
		if ctx.Err() != nil {
			return
		}
		log.Info("worker: computing phash",
			zap.String("title", item.Title),
			zap.Int("n", n+1),
			zap.Int("total", total),
		)
		if err := svc.ComputeAndStore(ctx, &item); err != nil {
			log.Warn("worker: phash computation failed",
				zap.String("title", item.Title),
				zap.Error(err),
			)
		}
	}
}

// autoTagUntagged runs auto-tagging on items that have not been tagged yet.
func autoTagUntagged(ctx context.Context, db *database.DB, svc *autotag.Service, log *zap.Logger) {
	var items []models.Media
	if err := db.SelectContext(ctx, &items, `
		SELECT * FROM media
		WHERE deleted_at IS NULL
		  AND auto_tag_status IN ('pending', 'failed')
		  AND (type = 'image' OR thumbnail_path != '')
		ORDER BY created_at DESC
		LIMIT 100
	`); err != nil {
		log.Error("worker: query untagged media", zap.Error(err))
		return
	}

	total := len(items)
	for n, item := range items {
		if ctx.Err() != nil {
			return
		}
		log.Info("worker: auto-tagging",
			zap.String("title", item.Title),
			zap.Int("n", n+1),
			zap.Int("total", total),
		)
		result, err := svc.AutoTag(ctx, &item)
		if err != nil {
			log.Warn("worker: auto-tag failed", zap.String("title", item.Title), zap.Error(err))
			_ = svc.MarkFailed(ctx, item.ID)
			continue
		}
		if result.Source == "none" {
			_ = svc.MarkFailed(ctx, item.ID)
			continue
		}
		if err := svc.ApplyTags(ctx, item.ID, result, result.SuggestedTags); err != nil {
			log.Warn("worker: apply tags failed", zap.String("title", item.Title), zap.Error(err))
		}
	}
}
