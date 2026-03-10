package downloader

import (
	"context"
	"time"

	"github.com/nacl-dev/tanuki/internal/database"
	"github.com/nacl-dev/tanuki/internal/models"
	"github.com/robfig/cron/v3"
	"go.uber.org/zap"
)

// Scheduler wraps robfig/cron to trigger scheduled downloads.
type Scheduler struct {
	cron    *cron.Cron
	db      *database.DB
	manager *Manager
	log     *zap.Logger
}

// NewScheduler creates and starts a cron scheduler.
func NewScheduler(db *database.DB, manager *Manager, log *zap.Logger) *Scheduler {
	c := cron.New(cron.WithSeconds())
	s := &Scheduler{cron: c, db: db, manager: manager, log: log}
	return s
}

// Start loads all enabled schedules from the database and registers them.
func (s *Scheduler) Start(ctx context.Context) error {
	var schedules []models.DownloadSchedule
	if err := s.db.Select(&schedules, `SELECT * FROM download_schedules WHERE enabled = true`); err != nil {
		return err
	}

	for _, sched := range schedules {
		if err := s.add(sched); err != nil {
			s.log.Warn("scheduler: add failed", zap.String("id", sched.ID), zap.Error(err))
		}
	}

	s.cron.Start()
	s.log.Info("scheduler: started", zap.Int("schedules", len(schedules)))

	<-ctx.Done()
	s.cron.Stop()
	return nil
}

func (s *Scheduler) add(sched models.DownloadSchedule) error {
	_, err := s.cron.AddFunc(sched.CronExpression, func() {
		s.log.Info("scheduler: triggering", zap.String("name", sched.Name), zap.String("url", sched.URLPattern))

		now := time.Now()
		s.db.Exec(`UPDATE download_schedules SET last_run = $2 WHERE id = $1`, sched.ID, now) //nolint:errcheck

		// Enqueue a new download job.
		manager := s.manager
		_ = manager
		s.db.Exec(`
			INSERT INTO download_jobs (id, user_id, url, source_type, status, progress, target_directory, retry_count)
			VALUES (gen_random_uuid(), $1, $2, $3, 'queued', 0, $4, 0)
		`, sched.UserID, sched.URLPattern, sched.SourceType, sched.TargetDirectory) //nolint:errcheck
	})
	return err
}
