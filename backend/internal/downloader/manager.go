// Package downloader manages concurrent download jobs.
package downloader

import (
	"context"
	"fmt"
	"time"

	"github.com/nacl-dev/tanuki/internal/database"
	"github.com/nacl-dev/tanuki/internal/models"
	"go.uber.org/zap"
)

// Manager processes queued download jobs up to a configurable concurrency limit.
type Manager struct {
	db          *database.DB
	engines     []Engine
	maxWorkers  int
	rateDelay   time.Duration
	log         *zap.Logger
	downloadsDir string
}

// NewManager creates a Manager with the given engines.
func NewManager(db *database.DB, engines []Engine, maxWorkers int, rateDelay time.Duration, downloadsDir string, log *zap.Logger) *Manager {
	return &Manager{
		db:           db,
		engines:      engines,
		maxWorkers:   maxWorkers,
		rateDelay:    rateDelay,
		log:          log,
		downloadsDir: downloadsDir,
	}
}

// Run starts the download processing loop. It blocks until ctx is cancelled.
func (m *Manager) Run(ctx context.Context) {
	sem := make(chan struct{}, m.maxWorkers)

	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			jobs, err := m.fetchQueued(ctx)
			if err != nil {
				m.log.Error("manager: fetch queued", zap.Error(err))
				continue
			}

			for _, job := range jobs {
				sem <- struct{}{}
				go func(j models.DownloadJob) {
					defer func() { <-sem }()
					m.process(ctx, j)
				}(job)
			}
		}
	}
}

func (m *Manager) fetchQueued(ctx context.Context) ([]models.DownloadJob, error) {
	var jobs []models.DownloadJob
	err := m.db.SelectContext(ctx, &jobs, `
		SELECT * FROM download_jobs
		WHERE status = 'queued'
		ORDER BY created_at ASC
		LIMIT $1
	`, m.maxWorkers)
	return jobs, err
}

func (m *Manager) process(ctx context.Context, job models.DownloadJob) {
	m.log.Info("download: start", zap.String("id", job.ID), zap.String("url", job.URL))
	m.setStatus(job.ID, models.DownloadStatusDownloading, "")

	engine := m.selectEngine(job.URL)
	if engine == nil {
		m.log.Warn("download: no engine found", zap.String("url", job.URL))
		m.setStatus(job.ID, models.DownloadStatusFailed, "no suitable download engine found")
		return
	}

	// Apply per-source rate limit delay.
	time.Sleep(m.rateDelay)

	if err := engine.Download(ctx, &job); err != nil {
		m.log.Error("download: failed", zap.String("id", job.ID), zap.Error(err))
		m.setStatus(job.ID, models.DownloadStatusFailed, err.Error())
		m.db.Exec(`UPDATE download_jobs SET retry_count = retry_count + 1 WHERE id = $1`, job.ID) //nolint:errcheck
		return
	}

	m.setStatus(job.ID, models.DownloadStatusCompleted, "")
	m.db.Exec(`UPDATE download_jobs SET completed_at = NOW() WHERE id = $1`, job.ID) //nolint:errcheck
	m.log.Info("download: completed", zap.String("id", job.ID))
}

func (m *Manager) selectEngine(rawURL string) Engine {
	for _, e := range m.engines {
		if e.CanHandle(rawURL) {
			return e
		}
	}
	return nil
}

func (m *Manager) setStatus(id string, status models.DownloadStatus, errMsg string) {
	m.db.Exec(`
		UPDATE download_jobs SET status = $2, error_message = $3, updated_at = NOW()
		WHERE id = $1
	`, id, string(status), errMsg) //nolint:errcheck
}

// UpdateProgress writes progress fields to the database.
func (m *Manager) UpdateProgress(id string, downloaded, total int64, files, totalFiles int) {
	var progress float64
	if total > 0 {
		progress = float64(downloaded) / float64(total) * 100
	}

	m.db.Exec(`
		UPDATE download_jobs SET
			downloaded_bytes = $2,
			total_bytes      = $3,
			downloaded_files = $4,
			total_files      = $5,
			progress         = $6,
			updated_at       = NOW()
		WHERE id = $1
	`, id, downloaded, total, files, totalFiles, fmt.Sprintf("%.2f", progress)) //nolint:errcheck
}
