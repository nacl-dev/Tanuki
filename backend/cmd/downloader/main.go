// Command downloader manages gallery-dl / yt-dlp download jobs from the queue.
package main

import (
	"context"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/nacl-dev/tanuki/internal/config"
	"github.com/nacl-dev/tanuki/internal/database"
	dl "github.com/nacl-dev/tanuki/internal/downloader"
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

	if err := db.Migrate(); err != nil {
		log.Fatal("migrate", zap.Error(err))
	}

	configDir := os.Getenv("CONFIG_DIR")
	if configDir == "" {
		configDir = "/app/config"
	}

	// Build engine list: yt-dlp first (video sites), gallery-dl second
	// (everything else), HTTP as fallback.
	engines := []dl.Engine{
		dl.NewRule34ArtEngine(log),
		dl.NewYtDlpEngine(configDir+"/yt-dlp.conf", log),
		dl.NewHentai0Engine(log),
		dl.NewImageGalleryEngine(log),
		dl.NewDanbooruEngine(log),
		dl.NewBooruEngine(log),
		dl.NewGalleryDLEngine(configDir+"/gallery-dl.conf", log),
		dl.NewHTTPEngine(log),
	}

	manager := dl.NewManager(
		db,
		engines,
		cfg.MaxConcurrentDownloads,
		time.Duration(cfg.RateLimitDelay)*time.Millisecond,
		cfg.DownloadsPath,
		cfg.MediaPath,
		cfg.ThumbnailsPath,
		log,
	)

	scheduler := dl.NewScheduler(db, manager, log)

	ctx, cancel := context.WithCancel(context.Background())

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-quit
		log.Info("downloader: shutting down")
		cancel()
	}()

	// Run the scheduler in background.
	go func() {
		if err := scheduler.Start(ctx); err != nil && ctx.Err() == nil {
			log.Error("scheduler error", zap.Error(err))
		}
	}()

	// Run the download manager (blocks until ctx is cancelled).
	manager.Run(ctx)
	log.Info("downloader: stopped")
}
