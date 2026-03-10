package downloader

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/nacl-dev/tanuki/internal/database"
	"github.com/nacl-dev/tanuki/internal/models"
	"github.com/nacl-dev/tanuki/internal/scanner"
	"github.com/nacl-dev/tanuki/internal/thumbnails"
	"go.uber.org/zap"
)

// Manager processes queued download jobs up to a configurable concurrency limit.
type Manager struct {
	db           *database.DB
	engines      []Engine
	maxWorkers   int
	rateDelay    time.Duration
	log          *zap.Logger
	downloadsDir string
	mediaPath    string
	thumbPath    string
}

// NewManager creates a Manager with the given engines.
func NewManager(db *database.DB, engines []Engine, maxWorkers int, rateDelay time.Duration, downloadsDir, mediaPath, thumbPath string, log *zap.Logger) *Manager {
	m := &Manager{
		db:           db,
		engines:      engines,
		maxWorkers:   maxWorkers,
		rateDelay:    rateDelay,
		log:          log,
		downloadsDir: downloadsDir,
		mediaPath:    mediaPath,
		thumbPath:    thumbPath,
	}
	for _, engine := range engines {
		if aware, ok := engine.(ProgressAwareEngine); ok {
			aware.SetProgressUpdater(m.UpdateProgress)
		}
	}
	return m
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
				job.TargetDirectory = m.resolveTargetDirectory(job.TargetDirectory)
				claimed, err := m.claimJob(ctx, job)
				if err != nil {
					m.log.Error("manager: claim job", zap.String("id", job.ID), zap.Error(err))
					continue
				}
				if !claimed {
					continue
				}

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

func (m *Manager) claimJob(ctx context.Context, job models.DownloadJob) (bool, error) {
	var claimedID string
	err := m.db.GetContext(ctx, &claimedID, `
		UPDATE download_jobs AS dj
		SET status = 'downloading', error_message = '', updated_at = NOW()
		WHERE dj.id = $1
		  AND dj.status = 'queued'
		  AND NOT EXISTS (
			SELECT 1
			FROM download_jobs AS active
			WHERE active.id <> dj.id
			  AND active.status IN ('downloading', 'processing')
			  AND active.url = dj.url
			  AND active.target_directory = $2
			  AND active.user_id IS NOT DISTINCT FROM $3
		  )
		RETURNING dj.id
	`, job.ID, job.TargetDirectory, job.UserID)
	if err != nil {
		if err == sql.ErrNoRows {
			return false, nil
		}
		return false, err
	}
	return claimedID != "", nil
}

func (m *Manager) process(ctx context.Context, job models.DownloadJob) {
	m.log.Info("download: start", zap.String("id", job.ID), zap.String("url", job.URL))

	engines := m.matchingEngines(job.URL)
	if len(engines) == 0 {
		m.log.Warn("download: no engine found", zap.String("url", job.URL))
		m.setStatus(job.ID, models.DownloadStatusFailed, "no suitable download engine found")
		return
	}

	// Apply per-source rate limit delay.
	time.Sleep(m.rateDelay)

	m.prefetchMetadata(job.ID, job.URL, engines)

	targetRoot := job.TargetDirectory
	stagingDir := filepath.Join(targetRoot, ".tanuki-job-"+job.ID)
	if err := removePathIfExists(stagingDir); err != nil {
		m.log.Warn("download: cleanup stale staging dir", zap.String("path", stagingDir), zap.Error(err))
	}
	if err := os.MkdirAll(stagingDir, 0o755); err != nil {
		m.setStatus(job.ID, models.DownloadStatusFailed, "create staging dir: "+err.Error())
		return
	}
	job.TargetDirectory = stagingDir

	var lastErr error
	for _, engine := range engines {
		if err := engine.Download(ctx, &job); err != nil {
			lastErr = err
			if isUnsupportedURLError(err) {
				m.log.Warn("download: engine unsupported", zap.String("id", job.ID), zap.Error(err))
				continue
			}

			m.log.Error("download: failed", zap.String("id", job.ID), zap.Error(err))
			m.setStatus(job.ID, models.DownloadStatusFailed, err.Error())
			m.db.Exec(`UPDATE download_jobs SET retry_count = retry_count + 1 WHERE id = $1`, job.ID) //nolint:errcheck
			return
		}

		movedPaths, err := organizeDownloadedFiles(stagingDir, targetRoot)
		if err != nil {
			m.log.Error("download: organize failed", zap.String("id", job.ID), zap.Error(err))
			m.setStatus(job.ID, models.DownloadStatusFailed, "organize download: "+err.Error())
			return
		}
		_ = os.RemoveAll(stagingDir)
		if len(movedPaths) == 0 {
			m.setStatus(job.ID, models.DownloadStatusFailed, "download completed but no supported media files were produced")
			return
		}
		if len(movedPaths) > 0 {
			m.UpdateProgress(job.ID, 1, 1, len(movedPaths), len(movedPaths))
			if err := m.refreshLibrary(ctx, movedPaths); err != nil {
				m.log.Warn("download: refresh library failed", zap.String("id", job.ID), zap.Error(err))
			}
		}

		m.setStatus(job.ID, models.DownloadStatusCompleted, "")
		m.db.Exec(`UPDATE download_jobs SET completed_at = NOW() WHERE id = $1`, job.ID) //nolint:errcheck
		m.log.Info("download: completed", zap.String("id", job.ID))
		return
	}

	if lastErr == nil {
		lastErr = fmt.Errorf("no suitable download engine found")
	}
	m.log.Error("download: failed", zap.String("id", job.ID), zap.Error(lastErr))
	_ = os.RemoveAll(stagingDir)
	m.setStatus(job.ID, models.DownloadStatusFailed, lastErr.Error())
	m.db.Exec(`UPDATE download_jobs SET retry_count = retry_count + 1 WHERE id = $1`, job.ID) //nolint:errcheck
}

func (m *Manager) matchingEngines(rawURL string) []Engine {
	matches := make([]Engine, 0, len(m.engines))
	for _, e := range m.engines {
		if e.CanHandle(rawURL) {
			matches = append(matches, e)
		}
	}
	return matches
}

func (m *Manager) setStatus(id string, status models.DownloadStatus, errMsg string) {
	m.db.Exec(`
		UPDATE download_jobs SET status = $2, error_message = $3, updated_at = NOW()
		WHERE id = $1
	`, id, string(status), errMsg) //nolint:errcheck
}

func (m *Manager) resolveTargetDirectory(targetDirectory string) string {
	if targetDirectory != "" {
		return targetDirectory
	}
	if m.downloadsDir != "" {
		return m.downloadsDir
	}
	return "/downloads"
}

// UpdateProgress writes progress fields to the database.
func (m *Manager) UpdateProgress(id string, downloaded, total int64, files, totalFiles int) {
	var progress float64
	if total > 0 {
		progress = float64(downloaded) / float64(total) * 100
	} else if totalFiles > 0 {
		progress = float64(files) / float64(totalFiles) * 100
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

func (m *Manager) prefetchMetadata(id, rawURL string, engines []Engine) {
	for _, engine := range engines {
		meta, err := engine.FetchMetadata(rawURL)
		if err != nil || meta == nil {
			continue
		}
		payload, err := json.Marshal(meta)
		if err != nil {
			return
		}
		totalFiles := meta.TotalFiles
		m.db.Exec(`
			UPDATE download_jobs SET source_metadata = $2, total_files = CASE WHEN total_files = 0 THEN $3 ELSE total_files END, updated_at = NOW()
			WHERE id = $1
		`, id, payload, totalFiles) //nolint:errcheck
		return
	}
}

func (m *Manager) refreshLibrary(ctx context.Context, movedPaths []string) error {
	if strings.TrimSpace(m.mediaPath) == "" || len(movedPaths) == 0 {
		return nil
	}
	for _, path := range movedPaths {
		rel, err := filepath.Rel(m.mediaPath, path)
		if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
			return nil
		}
	}

	sc := scanner.New(m.db, m.mediaPath, m.thumbPath, m.log)
	if err := sc.Run(ctx); err != nil {
		return err
	}
	if strings.TrimSpace(m.thumbPath) == "" {
		return nil
	}
	gen := thumbnails.New(m.thumbPath, m.log)
	for _, path := range movedPaths {
		var media models.Media
		if err := m.db.GetContext(ctx, &media, `SELECT * FROM media WHERE file_path = $1 AND deleted_at IS NULL`, path); err != nil {
			continue
		}
		if strings.TrimSpace(media.ThumbnailPath) != "" {
			continue
		}
		thumbPath, err := gen.GenerateForMedia(ctx, &media)
		if err != nil {
			continue
		}
		m.db.Exec(`UPDATE media SET thumbnail_path = $1, updated_at = NOW() WHERE id = $2`, thumbPath, media.ID) //nolint:errcheck
	}
	return nil
}

func removePathIfExists(path string) error {
	if strings.TrimSpace(path) == "" {
		return nil
	}
	if err := os.RemoveAll(path); err != nil && !os.IsNotExist(err) {
		return err
	}
	return nil
}
