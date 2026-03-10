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
	"github.com/nacl-dev/tanuki/internal/scanner"
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

	ticker := time.NewTicker(time.Duration(cfg.ScanInterval) * time.Second)
	defer ticker.Stop()

	// Run an initial scan immediately on start.
	if err := sc.Run(ctx); err != nil && ctx.Err() == nil {
		log.Error("initial scan failed", zap.Error(err))
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
		}
	}
}
