package scanner

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/nacl-dev/tanuki/internal/database"
	"github.com/nacl-dev/tanuki/internal/models"
	"github.com/nacl-dev/tanuki/internal/thumbnails"
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

// MediaTypeForExtension returns the configured media type for a file extension.
func MediaTypeForExtension(ext string) (models.MediaType, bool) {
	mediaType, ok := mediaExtensions[strings.ToLower(ext)]
	return mediaType, ok
}

// Scanner walks a media directory and synchronises it with the database.
type Scanner struct {
	db        *database.DB
	mediaPath string
	thumbPath string
	log       *zap.Logger
	client    *http.Client
}

// New creates a Scanner instance.
func New(db *database.DB, mediaPath, thumbPath string, log *zap.Logger) *Scanner {
	return &Scanner{
		db:        db,
		mediaPath: mediaPath,
		thumbPath: thumbPath,
		log:       log,
		client:    &http.Client{Timeout: 30 * time.Second},
	}
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

	defaultTitle := strings.TrimSuffix(filepath.Base(path), filepath.Ext(path))
	title := defaultTitle
	importMeta, err := s.loadImportMetadata(path)
	if err != nil {
		s.log.Warn("scanner: load import metadata failed", zap.String("path", path), zap.Error(err))
	}
	if importMeta != nil && strings.TrimSpace(importMeta.Title) != "" {
		title = strings.TrimSpace(importMeta.Title)
	}

	sourceURL := ""
	if importMeta != nil {
		sourceURL = strings.TrimSpace(importMeta.SourceURL)
	}

	var mediaID string
	var thumbnailPath string
	err = s.db.QueryRowxContext(ctx, `
		INSERT INTO media (id, title, type, file_path, file_size, checksum, source_url)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
		ON CONFLICT (file_path) DO UPDATE SET
			type       = EXCLUDED.type,
			file_size  = EXCLUDED.file_size,
			checksum   = EXCLUDED.checksum,
			title      = CASE
				WHEN media.title = $8 OR media.title = '' THEN EXCLUDED.title
				ELSE media.title
			END,
			source_url = CASE
				WHEN media.source_url = '' THEN EXCLUDED.source_url
				ELSE media.source_url
			END,
			deleted_at = NULL,
			updated_at = NOW()
		RETURNING id, thumbnail_path
	`,
		uuid.NewString(),
		title,
		string(mediaType),
		path,
		info.Size(),
		checksum,
		sourceURL,
		defaultTitle,
	).Scan(&mediaID, &thumbnailPath)
	if err != nil {
		return err
	}

	if importMeta != nil {
		if err := s.applyImportedTags(ctx, mediaID, importMeta.Tags); err != nil {
			s.log.Warn("scanner: apply imported tags failed", zap.String("path", path), zap.Error(err))
		}
		if strings.TrimSpace(importMeta.PosterURL) != "" && strings.TrimSpace(thumbnailPath) == "" {
			if err := s.downloadPosterThumbnail(ctx, mediaID, importMeta.PosterURL); err != nil {
				s.log.Warn("scanner: import poster failed", zap.String("path", path), zap.Error(err))
			}
		}
	}

	if err := s.ensureThumbnail(ctx, mediaID, path, mediaType, thumbnailPath); err != nil {
		s.log.Warn("scanner: ensure thumbnail failed", zap.String("path", path), zap.Error(err))
	}

	return nil
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

func (s *Scanner) loadImportMetadata(mediaPath string) (*models.ImportMetadata, error) {
	sidecarPath := mediaPath + ".tanuki.json"
	body, err := os.ReadFile(sidecarPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}

	var metadata models.ImportMetadata
	if err := json.Unmarshal(body, &metadata); err != nil {
		return nil, err
	}
	return &metadata, nil
}

func (s *Scanner) applyImportedTags(ctx context.Context, mediaID string, tags []string) error {
	for _, raw := range tags {
		name := strings.ToLower(strings.TrimSpace(raw))
		if name == "" {
			continue
		}

		tagID, err := ensureScannerTag(ctx, s.db, name)
		if err != nil {
			return err
		}
		if _, err := s.db.ExecContext(ctx, `
			INSERT INTO media_tags (media_id, tag_id)
			VALUES ($1, $2)
			ON CONFLICT DO NOTHING
		`, mediaID, tagID); err != nil {
			return err
		}
	}
	return nil
}

func ensureScannerTag(ctx context.Context, db *database.DB, name string) (string, error) {
	var existing struct {
		ID string `db:"id"`
	}
	if err := db.GetContext(ctx, &existing, `SELECT id FROM tags WHERE name = $1`, name); err == nil {
		return existing.ID, nil
	}

	id := uuid.NewString()
	if _, err := db.ExecContext(ctx, `
		INSERT INTO tags (id, name, category, usage_count)
		VALUES ($1, $2, $3, 0)
		ON CONFLICT (name) DO NOTHING
	`, id, name, models.TagCategoryGeneral); err != nil {
		return "", err
	}

	if err := db.GetContext(ctx, &existing, `SELECT id FROM tags WHERE name = $1`, name); err != nil {
		return "", err
	}
	return existing.ID, nil
}

func (s *Scanner) downloadPosterThumbnail(ctx context.Context, mediaID, posterURL string) error {
	if strings.TrimSpace(s.thumbPath) == "" {
		return nil
	}
	if err := os.MkdirAll(s.thumbPath, 0o755); err != nil {
		return err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, posterURL, nil)
	if err != nil {
		return err
	}
	req.Header.Set("User-Agent", "Mozilla/5.0 (Tanuki)")

	resp, err := s.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("poster status: %d", resp.StatusCode)
	}

	thumbPath := filepath.Join(s.thumbPath, mediaID+".jpg")
	out, err := os.Create(thumbPath)
	if err != nil {
		return err
	}
	defer out.Close()

	if _, err := io.Copy(out, resp.Body); err != nil {
		return err
	}

	_, err = s.db.ExecContext(ctx, `
		UPDATE media
		SET thumbnail_path = $2, updated_at = NOW()
		WHERE id = $1 AND deleted_at IS NULL AND (thumbnail_path = '' OR thumbnail_path IS NULL)
	`, mediaID, thumbPath)
	return err
}

func (s *Scanner) ensureThumbnail(ctx context.Context, mediaID, filePath string, mediaType models.MediaType, existingPath string) error {
	if strings.TrimSpace(s.thumbPath) == "" {
		return nil
	}
	if strings.TrimSpace(existingPath) != "" {
		if _, err := os.Stat(existingPath); err == nil {
			return nil
		}
	}

	gen := thumbnails.New(s.thumbPath, s.log)
	thumbPath, err := gen.GenerateForMedia(ctx, &models.Media{
		ID:            mediaID,
		Type:          mediaType,
		FilePath:      filePath,
		ThumbnailPath: existingPath,
	})
	if err != nil {
		return err
	}

	_, err = s.db.ExecContext(ctx, `
		UPDATE media
		SET thumbnail_path = $2, updated_at = NOW()
		WHERE id = $1 AND deleted_at IS NULL
	`, mediaID, thumbPath)
	return err
}
