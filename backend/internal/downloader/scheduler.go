package downloader

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/nacl-dev/tanuki/internal/database"
	"github.com/nacl-dev/tanuki/internal/models"
	"github.com/robfig/cron/v3"
	"go.uber.org/zap"
)

// Scheduler wraps robfig/cron to trigger scheduled downloads.
type Scheduler struct {
	mu           sync.Mutex
	cron         *cron.Cron
	db           *database.DB
	manager      *Manager
	log          *zap.Logger
	entries      map[string]cron.EntryID
	fingerprints map[string]string
}

// NewScheduler creates and starts a cron scheduler.
func NewScheduler(db *database.DB, manager *Manager, log *zap.Logger) *Scheduler {
	c := cron.New(cron.WithParser(newCronParser()))
	s := &Scheduler{
		cron:         c,
		db:           db,
		manager:      manager,
		log:          log,
		entries:      map[string]cron.EntryID{},
		fingerprints: map[string]string{},
	}
	return s
}

// Start loads all enabled schedules from the database and registers them.
func (s *Scheduler) Start(ctx context.Context) error {
	if err := s.syncSchedules(); err != nil {
		return err
	}
	s.cron.Start()
	s.log.Info("scheduler: started")

	ticker := time.NewTicker(15 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			stopCtx := s.cron.Stop()
			<-stopCtx.Done()
			return nil
		case <-ticker.C:
			if err := s.syncSchedules(); err != nil {
				s.log.Warn("scheduler: sync failed", zap.Error(err))
			}
		}
	}
}

func (s *Scheduler) syncSchedules() error {
	var schedules []models.DownloadSchedule
	if err := s.db.Select(&schedules, `SELECT * FROM download_schedules WHERE enabled = true`); err != nil {
		return err
	}

	seen := make(map[string]struct{}, len(schedules))

	s.mu.Lock()
	defer s.mu.Unlock()

	for _, sched := range schedules {
		seen[sched.ID] = struct{}{}

		fingerprint := scheduleFingerprint(sched)
		if current, ok := s.fingerprints[sched.ID]; ok && current == fingerprint {
			continue
		}

		if entryID, ok := s.entries[sched.ID]; ok {
			s.cron.Remove(entryID)
			delete(s.entries, sched.ID)
			delete(s.fingerprints, sched.ID)
		}

		if err := s.addLocked(sched, fingerprint); err != nil {
			s.log.Warn("scheduler: add failed", zap.String("id", sched.ID), zap.Error(err))
		}
	}

	for id, entryID := range s.entries {
		if _, ok := seen[id]; ok {
			continue
		}
		s.cron.Remove(entryID)
		delete(s.entries, id)
		delete(s.fingerprints, id)
		_, _ = s.db.Exec(`UPDATE download_schedules SET next_run = NULL WHERE id = $1`, id)
	}

	return nil
}

func (s *Scheduler) addLocked(sched models.DownloadSchedule, fingerprint string) error {
	_, nextRun, err := ValidateCronExpression(sched.CronExpression)
	if err != nil {
		return fmt.Errorf("invalid cron expression: %w", err)
	}

	var entryID cron.EntryID
	entryID, err = s.cron.AddFunc(sched.CronExpression, func() {
		now := time.Now()
		next := nextScheduledRun(sched.CronExpression, now)
		s.log.Info("scheduler: triggering", zap.String("name", sched.Name), zap.String("url", sched.URLPattern))

		_, _ = s.db.Exec(`
			UPDATE download_schedules
			SET last_run = $2, next_run = $3, updated_at = NOW()
			WHERE id = $1
		`, sched.ID, now, next)

		_, _ = s.db.Exec(`
			INSERT INTO download_jobs (id, user_id, url, source_type, status, progress, target_directory, retry_count)
			VALUES (gen_random_uuid(), $1, $2, $3, 'queued', 0, $4, 0)
		`, sched.UserID, sched.URLPattern, sched.SourceType, sched.TargetDirectory)
	})
	if err != nil {
		return err
	}

	s.entries[sched.ID] = entryID
	s.fingerprints[sched.ID] = fingerprint
	_, _ = s.db.Exec(`
		UPDATE download_schedules
		SET next_run = $2, updated_at = NOW()
		WHERE id = $1
	`, sched.ID, nextRun)

	return nil
}

func nextScheduledRun(spec string, from time.Time) *time.Time {
	normalized, _, err := ValidateCronExpression(spec)
	if err != nil {
		return nil
	}
	schedule, err := newCronParser().Parse(normalized)
	if err != nil {
		return nil
	}
	next := schedule.Next(from)
	return &next
}

func scheduleFingerprint(sched models.DownloadSchedule) string {
	return fmt.Sprintf("%s|%s|%s|%s|%t", sched.CronExpression, sched.URLPattern, sched.SourceType, sched.TargetDirectory, sched.Enabled)
}
