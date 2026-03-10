package scanner

import (
	"context"
	"crypto/sha256"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/nacl-dev/tanuki/internal/database"
	"github.com/nacl-dev/tanuki/internal/models"
	"go.uber.org/zap"
)

// mediaExtensions maps file extensions to their MediaType.
var mediaExtensions = map[string]models.MediaType{
	".mp4":  models.MediaTypeVideo,
	".mkv":  models.MediaTypeVideo,
	".webm": models.MediaTypeVideo,
	".avi":  models.MediaTypeVideo,
	".mov":  models.MediaTypeVideo,
	".jpg":  models.MediaTypeImage,
	".jpeg": models.MediaTypeImage,
	".png":  models.MediaTypeImage,
	".webp": models.MediaTypeImage,
	".gif":  models.MediaTypeImage,
	".zip":  models.MediaTypeManga,
	".cbz":  models.MediaTypeManga,
	".cbr":  models.MediaTypeComic,
	".rar":  models.MediaTypeComic,
}

// Scanner walks a media directory and synchronises it with the database.
type Scanner struct {
	db        *database.DB
	mediaPath string
	log       *zap.Logger
}

// New creates a Scanner instance.
func New(db *database.DB, mediaPath string, log *zap.Logger) *Scanner {
	return &Scanner{db: db, mediaPath: mediaPath, log: log}
}

// Run executes a full scan of the media directory.
func (s *Scanner) Run(ctx context.Context) error {
	s.log.Info("scanner: starting scan", zap.String("path", s.mediaPath))
	start := time.Now()

	seen := map[string]bool{}

	err := filepath.WalkDir(s.mediaPath, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			s.log.Warn("scanner: walk error", zap.String("path", path), zap.Error(err))
			return nil
		}
		if d.IsDir() {
			return nil
		}
		if ctx.Err() != nil {
			return ctx.Err()
		}

		ext := strings.ToLower(filepath.Ext(d.Name()))
		mediaType, ok := mediaExtensions[ext]
		if !ok {
			return nil
		}

		seen[path] = true

		if err := s.upsert(ctx, path, mediaType); err != nil {
			s.log.Error("scanner: upsert failed", zap.String("path", path), zap.Error(err))
		}

		return nil
	})
	if err != nil {
		return fmt.Errorf("walk: %w", err)
	}

	if err := s.removeStale(ctx, seen); err != nil {
		s.log.Warn("scanner: remove stale failed", zap.Error(err))
	}

	s.log.Info("scanner: scan complete",
		zap.Int("seen", len(seen)),
		zap.Duration("elapsed", time.Since(start)),
	)
	return nil
}

// upsert inserts a new Media record or updates the checksum/size if the file
// has changed since the last scan.
func (s *Scanner) upsert(ctx context.Context, path string, mediaType models.MediaType) error {
	info, err := os.Stat(path)
	if err != nil {
		return fmt.Errorf("stat %s: %w", path, err)
	}

	checksum, err := sha256File(path)
	if err != nil {
		return fmt.Errorf("checksum %s: %w", path, err)
	}

	title := strings.TrimSuffix(filepath.Base(path), filepath.Ext(path))

	_, err = s.db.ExecContext(ctx, `
		INSERT INTO media (id, title, type, file_path, file_size, checksum)
		VALUES ($1, $2, $3, $4, $5, $6)
		ON CONFLICT (file_path) DO UPDATE SET
			title      = EXCLUDED.title,
			type       = EXCLUDED.type,
			file_size  = EXCLUDED.file_size,
			checksum   = EXCLUDED.checksum,
			deleted_at = NULL,
			updated_at = NOW()
	`,
		uuid.NewString(),
		title,
		string(mediaType),
		path,
		info.Size(),
		checksum,
	)
	return err
}

// removeStale soft-deletes database records that no longer have a file on disk.
func (s *Scanner) removeStale(ctx context.Context, seen map[string]bool) error {
	rows, err := s.db.QueryxContext(ctx, `
		SELECT id, file_path FROM media WHERE deleted_at IS NULL
	`)
	if err != nil {
		return err
	}
	defer rows.Close()

	type row struct {
		ID       string `db:"id"`
		FilePath string `db:"file_path"`
	}

	for rows.Next() {
		var r row
		if err := rows.StructScan(&r); err != nil {
			continue
		}
		if !seen[r.FilePath] {
			s.db.ExecContext(ctx, `UPDATE media SET deleted_at = NOW() WHERE id = $1`, r.ID) //nolint:errcheck
		}
	}

	return rows.Err()
}

// sha256File returns the hex-encoded SHA-256 digest of a file.
func sha256File(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()

	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", err
	}
	return fmt.Sprintf("%x", h.Sum(nil)), nil
}
