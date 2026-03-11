package scanner

import (
	"context"
	"crypto/sha256"
	"database/sql"
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
	"github.com/nacl-dev/tanuki/internal/remotehttp"
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

type scanStats struct {
	Seen     int
	Hashed   int
	Reused   int
	Inserted int
	Updated  int
}

type upsertResult struct {
	Hashed   bool
	Inserted bool
}

type existingMedia struct {
	ID            string     `db:"id"`
	FileSize      int64      `db:"file_size"`
	Checksum      string     `db:"checksum"`
	ThumbnailPath string     `db:"thumbnail_path"`
	ScanMTime     *time.Time `db:"scan_mtime"`
}

// New creates a Scanner instance.
func New(db *database.DB, mediaPath, thumbPath string, log *zap.Logger) *Scanner {
	return &Scanner{
		db:        db,
		mediaPath: mediaPath,
		thumbPath: thumbPath,
		log:       log,
		client:    remotehttp.NewClient(30 * time.Second),
	}
}

// Run executes a full scan of the media directory.
func (s *Scanner) Run(ctx context.Context) error {
	s.log.Info("scanner: starting scan", zap.String("path", s.mediaPath))
	start := time.Now()

	seen := map[string]bool{}
	stats := scanStats{}

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
		stats.Seen++

		result, err := s.upsert(ctx, path, mediaType)
		if err != nil {
			s.log.Error("scanner: upsert failed", zap.String("path", path), zap.Error(err))
		} else {
			if result.Hashed {
				stats.Hashed++
			} else {
				stats.Reused++
			}
			if result.Inserted {
				stats.Inserted++
			} else {
				stats.Updated++
			}
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
		zap.Int("hashed", stats.Hashed),
		zap.Int("reused", stats.Reused),
		zap.Int("inserted", stats.Inserted),
		zap.Int("updated", stats.Updated),
		zap.Duration("elapsed", time.Since(start)),
	)
	return nil
}

// upsert inserts a new Media record or updates the checksum/size if the file
// has changed since the last scan.
func (s *Scanner) upsert(ctx context.Context, path string, mediaType models.MediaType) (upsertResult, error) {
	info, err := os.Stat(path)
	if err != nil {
		return upsertResult{}, fmt.Errorf("stat %s: %w", path, err)
	}

	scanMTime := s.scanMTime(path, info.ModTime())
	existing, err := s.lookupExisting(ctx, path)
	if err != nil {
		return upsertResult{}, fmt.Errorf("lookup existing %s: %w", path, err)
	}

	checksum := ""
	needsHash := existing == nil ||
		existing.FileSize != info.Size() ||
		!sameScanTime(existing.ScanMTime, scanMTime)
	if needsHash {
		checksum, err = sha256File(path)
		if err != nil {
			return upsertResult{}, fmt.Errorf("checksum %s: %w", path, err)
		}
	} else {
		checksum = existing.Checksum
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
	var inserted bool
	err = s.db.QueryRowxContext(ctx, `
		INSERT INTO media (id, title, type, file_path, file_size, checksum, source_url, scan_mtime)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
		ON CONFLICT (file_path) DO UPDATE SET
			type       = EXCLUDED.type,
			file_size  = EXCLUDED.file_size,
			checksum   = EXCLUDED.checksum,
			scan_mtime = EXCLUDED.scan_mtime,
			title      = CASE
				WHEN media.title = $9 OR media.title = '' THEN EXCLUDED.title
				ELSE media.title
			END,
			source_url = CASE
				WHEN media.source_url = '' THEN EXCLUDED.source_url
				ELSE media.source_url
			END,
			phash = CASE
				WHEN media.file_size IS DISTINCT FROM EXCLUDED.file_size
				  OR media.scan_mtime IS DISTINCT FROM EXCLUDED.scan_mtime
				  OR media.checksum IS DISTINCT FROM EXCLUDED.checksum
				THEN NULL
				ELSE media.phash
			END,
			phash_computed_at = CASE
				WHEN media.file_size IS DISTINCT FROM EXCLUDED.file_size
				  OR media.scan_mtime IS DISTINCT FROM EXCLUDED.scan_mtime
				  OR media.checksum IS DISTINCT FROM EXCLUDED.checksum
				THEN NULL
				ELSE media.phash_computed_at
			END,
			deleted_at = NULL,
			updated_at = NOW()
		RETURNING id, thumbnail_path, (xmax = 0) AS inserted
	`,
		uuid.NewString(),
		title,
		string(mediaType),
		path,
		info.Size(),
		checksum,
		sourceURL,
		scanMTime,
		defaultTitle,
	).Scan(&mediaID, &thumbnailPath, &inserted)
	if err != nil {
		return upsertResult{}, err
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

	return upsertResult{
		Hashed:   needsHash,
		Inserted: inserted,
	}, nil
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

func (s *Scanner) lookupExisting(ctx context.Context, path string) (*existingMedia, error) {
	var existing existingMedia
	err := s.db.GetContext(ctx, &existing, `
		SELECT id, file_size, checksum, thumbnail_path, scan_mtime
		FROM media
		WHERE file_path = $1
		LIMIT 1
	`, path)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, err
	}
	return &existing, nil
}

func (s *Scanner) scanMTime(mediaPath string, fileTime time.Time) time.Time {
	latest := normalizeScanTime(fileTime)
	for _, candidate := range []string{mediaPath + ".tanuki.json", mediaPath + ".info.json"} {
		info, err := os.Stat(candidate)
		if err != nil {
			continue
		}
		candidateTime := normalizeScanTime(info.ModTime())
		if candidateTime.After(latest) {
			latest = candidateTime
		}
	}
	return latest
}

func sameScanTime(existing *time.Time, current time.Time) bool {
	if existing == nil {
		return false
	}
	return normalizeScanTime(*existing).Equal(normalizeScanTime(current))
}

func normalizeScanTime(value time.Time) time.Time {
	return value.UTC().Truncate(time.Microsecond)
}

func (s *Scanner) loadImportMetadata(mediaPath string) (*models.ImportMetadata, error) {
	sidecarPath := mediaPath + ".tanuki.json"
	body, err := os.ReadFile(sidecarPath)
	if err != nil {
		if !os.IsNotExist(err) {
			return nil, err
		}
	} else {
		var metadata models.ImportMetadata
		if err := json.Unmarshal(body, &metadata); err != nil {
			return nil, err
		}
		return &metadata, nil
	}

	infoPath := mediaPath + ".info.json"
	body, err = os.ReadFile(infoPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}

	var payload struct {
		Title       string   `json:"title"`
		WebpageURL  string   `json:"webpage_url"`
		OriginalURL string   `json:"original_url"`
		Thumbnail   string   `json:"thumbnail"`
		Tags        []string `json:"tags"`
	}
	if err := json.Unmarshal(body, &payload); err != nil {
		return nil, err
	}

	sourceURL := strings.TrimSpace(payload.WebpageURL)
	if sourceURL == "" {
		sourceURL = strings.TrimSpace(payload.OriginalURL)
	}

	return &models.ImportMetadata{
		Title:     strings.TrimSpace(payload.Title),
		SourceURL: sourceURL,
		PosterURL: strings.TrimSpace(payload.Thumbnail),
		Tags:      payload.Tags,
	}, nil
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
	if err := remotehttp.ValidateURL(posterURL); err != nil {
		return err
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

	if _, err := io.Copy(out, io.LimitReader(resp.Body, 20<<20)); err != nil {
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
