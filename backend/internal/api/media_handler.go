package api

import (
	"archive/zip"
	"bytes"
	"context"
	"database/sql"
	"errors"
	"fmt"
	"image"
	_ "image/gif"
	"image/jpeg"
	_ "image/png"
	"io"
	"math"
	"mime"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/nacl-dev/tanuki/internal/database"
	"github.com/nacl-dev/tanuki/internal/models"
	"github.com/nacl-dev/tanuki/internal/remotehttp"
	"github.com/nacl-dev/tanuki/internal/tagrules"
	"github.com/nacl-dev/tanuki/internal/thumbnails"
	"golang.org/x/image/draw"
	_ "golang.org/x/image/webp"
)

// MediaHandler handles CRUD operations for media items.
type MediaHandler struct {
	db        *database.DB
	mediaPath string
	thumbPath string
	tagRules  *tagrules.Service
}

// List returns a paginated list of media items with optional filtering.
// GET /api/media?page=1&limit=50&type=video&q=search&favorite=true&tag=artist&tags=a,b&sort=newest&min_rating=3
func (h *MediaHandler) List(c *gin.Context) {
	type query struct {
		Page       int    `form:"page"       binding:"-"`
		Limit      int    `form:"limit"      binding:"-"`
		Type       string `form:"type"       binding:"-"`
		Q          string `form:"q"          binding:"-"`
		Favorite   *bool  `form:"favorite"   binding:"-"`
		InProgress *bool  `form:"in_progress" binding:"-"`
		Tag        string `form:"tag"        binding:"-"`
		Tags       string `form:"tags"       binding:"-"`
		Sort       string `form:"sort"       binding:"-"`
		MinRating  *int   `form:"min_rating" binding:"-"`
	}

	var q query
	if err := c.ShouldBindQuery(&q); err != nil {
		respondError(c, http.StatusBadRequest, err.Error())
		return
	}

	if q.Page <= 0 {
		q.Page = 1
	}
	if q.Limit <= 0 || q.Limit > 200 {
		q.Limit = 50
	}

	offset := (q.Page - 1) * q.Limit

	// Build a shared WHERE clause so that the count and the data query use the
	// same filters.
	ctx := c.Request.Context()
	whereClause := ` WHERE m.deleted_at IS NULL`
	args := []interface{}{}
	argIdx := 1

	if q.Type != "" {
		whereClause += ` AND m.type = $` + itoa(argIdx)
		args = append(args, q.Type)
		argIdx++
	}
	if q.Q != "" {
		whereClause += ` AND (
			m.title ILIKE $` + itoa(argIdx) + `
			OR m.work_title ILIKE $` + itoa(argIdx) + `
			OR m.id IN (
				SELECT mt.media_id
				FROM media_tags mt
				JOIN tags t ON t.id = mt.tag_id
				WHERE t.name ILIKE $` + itoa(argIdx) + `
				   OR EXISTS (
						SELECT 1
						FROM tag_aliases ta
						WHERE ta.tag_id = t.id
						  AND ta.alias_name ILIKE $` + itoa(argIdx) + `
				   )
			)
		)`
		args = append(args, "%"+q.Q+"%")
		argIdx++
	}
	if q.Favorite != nil {
		whereClause += ` AND m.favorite = $` + itoa(argIdx)
		args = append(args, *q.Favorite)
		argIdx++
	}
	if q.InProgress != nil {
		if *q.InProgress {
			whereClause += ` AND m.read_progress > 0`
		} else {
			whereClause += ` AND COALESCE(m.read_progress, 0) = 0`
		}
	}
	if q.MinRating != nil {
		whereClause += ` AND m.rating >= $` + itoa(argIdx)
		args = append(args, *q.MinRating)
		argIdx++
	}
	// Single tag filter.
	if q.Tag != "" {
		canonicalTag, err := h.tagRulesService().CanonicalizeExpression(ctx, q.Tag)
		if err != nil {
			respondError(c, http.StatusInternalServerError, "resolve tag filter: "+err.Error())
			return
		}
		q.Tag = canonicalTag
		whereClause += ` AND m.id IN (
			SELECT mt.media_id
			FROM media_tags mt
			JOIN tags t ON t.id = mt.tag_id
			WHERE ` + tagExpressionMatchSQL(`$`+itoa(argIdx), "t.name", "t.category") + `
		)`
		args = append(args, q.Tag)
		argIdx++
	}
	// Multi-tag filter (AND logic): each tag listed in ?tags= must be present.
	if q.Tags != "" {
		for _, rawTag := range splitTags(q.Tags) {
			tag, err := h.tagRulesService().CanonicalizeExpression(ctx, rawTag)
			if err != nil {
				respondError(c, http.StatusInternalServerError, "resolve tag filters: "+err.Error())
				return
			}
			if tag == "" {
				continue
			}
			whereClause += ` AND m.id IN (
				SELECT mt.media_id
				FROM media_tags mt
				JOIN tags t ON t.id = mt.tag_id
				WHERE ` + tagExpressionMatchSQL(`$`+itoa(argIdx), "t.name", "t.category") + `
			)`
			args = append(args, tag)
			argIdx++
		}
	}

	// Determine ORDER BY clause based on the sort parameter.
	orderClause := ` ORDER BY m.created_at DESC`
	switch q.Sort {
	case "oldest":
		orderClause = ` ORDER BY m.created_at ASC`
	case "title":
		orderClause = ` ORDER BY m.title ASC`
	case "rating":
		orderClause = ` ORDER BY m.rating DESC, m.created_at DESC`
	case "size":
		orderClause = ` ORDER BY m.file_size DESC`
	case "views":
		orderClause = ` ORDER BY m.view_count DESC`
	}

	// Count query applies the same filters.
	var total int
	countQuery := `SELECT COUNT(*) FROM media m` + whereClause
	if err := h.db.QueryRow(countQuery, args...).Scan(&total); err != nil {
		respondError(c, http.StatusInternalServerError, "count media: "+err.Error())
		return
	}

	sqlQuery := `SELECT m.* FROM media m` + whereClause + orderClause +
		` LIMIT $` + itoa(argIdx) + ` OFFSET $` + itoa(argIdx+1)
	args = append(args, q.Limit, offset)

	var items []models.Media
	if err := h.db.Select(&items, sqlQuery, args...); err != nil {
		respondError(c, http.StatusInternalServerError, "query media: "+err.Error())
		return
	}

	if err := h.populateMediaRelations(c.GetString("userID"), items); err != nil {
		respondError(c, http.StatusInternalServerError, "load media relations: "+err.Error())
		return
	}

	respondOK(c, items, &Meta{Page: q.Page, Total: total})
}

// Suggestions returns lightweight search suggestions for titles.
// GET /api/media/suggestions?q=blue
func (h *MediaHandler) Suggestions(c *gin.Context) {
	q := strings.TrimSpace(c.Query("q"))
	if q == "" {
		respondOK(c, []gin.H{}, nil)
		return
	}

	type suggestion struct {
		Type  string `db:"type" json:"type"`
		Value string `db:"value" json:"value"`
		Label string `db:"label" json:"label"`
	}

	var suggestions []suggestion
	if err := h.db.Select(&suggestions, `
		SELECT type, value, label
		FROM (
			SELECT
				'title' AS type,
				m.title AS value,
				m.title AS label,
				0 AS rank
			FROM media m
			WHERE m.deleted_at IS NULL
				AND m.title <> ''
				AND m.title ILIKE $1
			GROUP BY m.title

			UNION ALL

			SELECT
				'title' AS type,
				m.work_title AS value,
				m.work_title AS label,
				1 AS rank
			FROM media m
			WHERE m.deleted_at IS NULL
				AND m.work_title <> ''
				AND m.work_title ILIKE $1
			GROUP BY m.work_title

			UNION ALL

			SELECT
				'title' AS type,
				m.title AS value,
				m.title AS label,
				2 AS rank
			FROM media m
			WHERE m.deleted_at IS NULL
				AND m.title <> ''
				AND m.title ILIKE $2
			GROUP BY m.title

			UNION ALL

			SELECT
				'title' AS type,
				m.work_title AS value,
				m.work_title AS label,
				3 AS rank
			FROM media m
			WHERE m.deleted_at IS NULL
				AND m.work_title <> ''
				AND m.work_title ILIKE $2
			GROUP BY m.work_title
		) s
		ORDER BY rank ASC, label ASC
		LIMIT 12
	`, q+"%", "%"+q+"%"); err != nil {
		respondError(c, http.StatusInternalServerError, "suggest media: "+err.Error())
		return
	}

	respondOK(c, suggestions, nil)
}

// Get returns a single media item by its UUID.
// GET /api/media/:id
func (h *MediaHandler) Get(c *gin.Context) {
	h.respondMedia(c, c.Param("id"), true)
}

// Update patches mutable fields of a media item.
// PATCH /api/media/:id
func (h *MediaHandler) Update(c *gin.Context) {
	id := c.Param("id")

	var body struct {
		Title        *string  `json:"title"`
		WorkTitle    *string  `json:"work_title"`
		WorkIndex    *int     `json:"work_index"`
		Rating       *int     `json:"rating"`
		Favorite     *bool    `json:"favorite"`
		Language     *string  `json:"language"`
		SourceURL    *string  `json:"source_url"`
		CreatedAt    *string  `json:"created_at"`
		TagNames     []string `json:"tag_names"`
		ReadProgress *int     `json:"read_progress"`
		ReadTotal    *int     `json:"read_total"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		respondError(c, http.StatusBadRequest, err.Error())
		return
	}

	var createdAt any = nil
	if body.CreatedAt != nil {
		parsed, err := time.Parse(time.RFC3339, *body.CreatedAt)
		if err != nil {
			parsed, err = time.Parse("2006-01-02", *body.CreatedAt)
			if err != nil {
				respondError(c, http.StatusBadRequest, "created_at must be ISO date or RFC3339")
				return
			}
		}
		createdAt = parsed
	}
	if body.WorkTitle != nil {
		trimmed := strings.TrimSpace(*body.WorkTitle)
		body.WorkTitle = &trimmed
	}
	if body.WorkIndex != nil && *body.WorkIndex < 0 {
		respondError(c, http.StatusBadRequest, "work_index must be 0 or greater")
		return
	}

	_, err := h.db.Exec(`
		UPDATE media SET
			title         = COALESCE($2, title),
			work_title    = COALESCE($3, work_title),
			work_index    = COALESCE($4, work_index),
			rating        = COALESCE($5, rating),
			favorite      = COALESCE($6, favorite),
			language      = COALESCE($7, language),
			source_url    = COALESCE($8, source_url),
			created_at    = COALESCE($9, created_at),
			read_progress = COALESCE($10, read_progress),
			read_total    = COALESCE($11, read_total),
			updated_at    = NOW()
		WHERE id = $1 AND deleted_at IS NULL
	`, id, body.Title, body.WorkTitle, body.WorkIndex, body.Rating, body.Favorite, body.Language, body.SourceURL,
		createdAt, body.ReadProgress, body.ReadTotal)
	if err != nil {
		respondError(c, http.StatusInternalServerError, "update media: "+err.Error())
		return
	}

	if body.TagNames != nil {
		if err := h.replaceMediaTags(c.Request.Context(), id, body.TagNames); err != nil {
			respondError(c, http.StatusInternalServerError, "update media tags: "+err.Error())
			return
		}
	}

	h.respondMedia(c, id, false)
}

// Delete soft-deletes a media item.
// DELETE /api/media/:id
func (h *MediaHandler) Delete(c *gin.Context) {
	id := c.Param("id")

	var body struct {
		DeleteFile bool `json:"delete_file"`
	}
	_ = c.ShouldBindJSON(&body)

	var item models.Media
	if err := h.db.Get(&item, `SELECT * FROM media WHERE id = $1 AND deleted_at IS NULL`, id); err != nil {
		respondError(c, http.StatusNotFound, "media not found")
		return
	}

	if body.DeleteFile {
		filePath, err := ensureManagedPath(item.FilePath, h.mediaPath)
		if err != nil {
			respondError(c, http.StatusInternalServerError, "delete file: "+err.Error())
			return
		}
		if err := removeIfExists(filePath); err != nil {
			respondError(c, http.StatusInternalServerError, "delete file: "+err.Error())
			return
		}
		if strings.TrimSpace(item.ThumbnailPath) != "" {
			thumbnailPath, err := ensureManagedPath(item.ThumbnailPath, h.thumbPath)
			if err != nil {
				respondError(c, http.StatusInternalServerError, "delete thumbnail: "+err.Error())
				return
			}
			if err := removeIfExists(thumbnailPath); err != nil {
				respondError(c, http.StatusInternalServerError, "delete thumbnail: "+err.Error())
				return
			}
		}
	}

	if _, err := h.db.Exec(`UPDATE media SET deleted_at = NOW() WHERE id = $1`, id); err != nil {
		respondError(c, http.StatusInternalServerError, "delete media: "+err.Error())
		return
	}

	c.Status(http.StatusNoContent)
}

// ServeFile streams the original media file.
// GET /api/media/:id/file
func (h *MediaHandler) ServeFile(c *gin.Context) {
	id := c.Param("id")

	var item models.Media
	if err := h.db.Get(&item, `SELECT * FROM media WHERE id = $1 AND deleted_at IS NULL`, id); err != nil {
		respondError(c, http.StatusNotFound, "media not found")
		return
	}

	filePath, err := ensureManagedPath(item.FilePath, h.mediaPath)
	if err != nil {
		respondError(c, http.StatusInternalServerError, "file path is outside managed media roots")
		return
	}
	if _, err := os.Stat(filePath); os.IsNotExist(err) {
		respondError(c, http.StatusNotFound, "file not found on disk")
		return
	}

	c.Header("Cache-Control", "private, max-age=86400")
	c.File(filePath)
}

// ServeThumbnail serves the thumbnail image for a media item.
// GET /api/media/:id/thumbnail
func (h *MediaHandler) ServeThumbnail(c *gin.Context) {
	id := c.Param("id")

	var item models.Media
	if err := h.db.Get(&item, `SELECT * FROM media WHERE id = $1 AND deleted_at IS NULL`, id); err != nil {
		respondError(c, http.StatusNotFound, "media not found")
		return
	}

	if item.ThumbnailPath == "" && strings.TrimSpace(h.thumbPath) != "" {
		gen := thumbnails.New(h.thumbPath, nil)
		if thumbPath, genErr := gen.GenerateForMedia(c.Request.Context(), &item); genErr == nil {
			item.ThumbnailPath = thumbPath
			_, _ = h.db.Exec(`
				UPDATE media
				SET thumbnail_path = $2, updated_at = NOW()
				WHERE id = $1 AND deleted_at IS NULL
			`, item.ID, thumbPath)
		}
	}

	if item.ThumbnailPath == "" {
		c.Status(http.StatusNotFound)
		return
	}

	thumbnailPath, err := ensureManagedPath(item.ThumbnailPath, h.thumbPath)
	if err != nil {
		c.Status(http.StatusNotFound)
		return
	}
	if _, err := os.Stat(thumbnailPath); os.IsNotExist(err) {
		if strings.TrimSpace(h.thumbPath) != "" {
			gen := thumbnails.New(h.thumbPath, nil)
			if thumbPath, genErr := gen.GenerateForMedia(c.Request.Context(), &item); genErr == nil {
				item.ThumbnailPath = thumbPath
				_, _ = h.db.Exec(`
					UPDATE media
					SET thumbnail_path = $2, updated_at = NOW()
					WHERE id = $1 AND deleted_at IS NULL
				`, item.ID, thumbPath)
			}
		}
		thumbnailPath, err = ensureManagedPath(item.ThumbnailPath, h.thumbPath)
		if err != nil {
			c.Status(http.StatusNotFound)
			return
		}
		if _, err := os.Stat(thumbnailPath); os.IsNotExist(err) {
			c.Status(http.StatusNotFound)
			return
		}
	}

	c.Header("Cache-Control", "private, max-age=86400")
	c.File(thumbnailPath)
}

// UploadThumbnail stores a custom thumbnail uploaded by the user.
// POST /api/media/:id/thumbnail/upload
func (h *MediaHandler) UploadThumbnail(c *gin.Context) {
	id := c.Param("id")

	item, ok := h.loadMediaForThumbnail(c, id)
	if !ok {
		return
	}

	file, err := c.FormFile("thumbnail")
	if err != nil {
		respondError(c, http.StatusBadRequest, "missing thumbnail file")
		return
	}

	src, err := file.Open()
	if err != nil {
		respondError(c, http.StatusBadRequest, "open thumbnail upload: "+err.Error())
		return
	}
	defer src.Close()

	buf, err := io.ReadAll(io.LimitReader(src, 20<<20))
	if err != nil {
		respondError(c, http.StatusBadRequest, "read thumbnail upload: "+err.Error())
		return
	}

	if err := h.persistThumbnail(id, item.ThumbnailPath, buf); err != nil {
		respondError(c, http.StatusBadRequest, "save thumbnail: "+err.Error())
		return
	}

	h.respondMedia(c, id, false)
}

// FetchThumbnail downloads a remote image and stores it as the custom thumbnail.
// POST /api/media/:id/thumbnail/fetch
func (h *MediaHandler) FetchThumbnail(c *gin.Context) {
	id := c.Param("id")

	item, ok := h.loadMediaForThumbnail(c, id)
	if !ok {
		return
	}

	var body struct {
		URL string `json:"url"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		respondError(c, http.StatusBadRequest, err.Error())
		return
	}

	body.URL = strings.TrimSpace(body.URL)
	if body.URL == "" {
		respondError(c, http.StatusBadRequest, "url is required")
		return
	}
	if err := remotehttp.ValidateURL(body.URL); err != nil {
		respondError(c, http.StatusBadRequest, "invalid url: "+err.Error())
		return
	}

	req, err := http.NewRequestWithContext(c.Request.Context(), http.MethodGet, body.URL, nil)
	if err != nil {
		respondError(c, http.StatusBadRequest, "invalid url: "+err.Error())
		return
	}

	resp, err := remotehttp.NewClient(15 * time.Second).Do(req)
	if err != nil {
		respondError(c, http.StatusBadGateway, "download thumbnail: "+err.Error())
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		respondError(c, http.StatusBadGateway, fmt.Sprintf("download thumbnail: remote returned %d", resp.StatusCode))
		return
	}

	buf, err := io.ReadAll(io.LimitReader(resp.Body, 20<<20))
	if err != nil {
		respondError(c, http.StatusBadGateway, "read remote thumbnail: "+err.Error())
		return
	}

	if err := h.persistThumbnail(id, item.ThumbnailPath, buf); err != nil {
		respondError(c, http.StatusBadRequest, "save thumbnail: "+err.Error())
		return
	}

	h.respondMedia(c, id, false)
}

// splitTags splits a comma-separated tag string into individual tag names,
// trimming whitespace from each entry.
func splitTags(s string) []string {
	parts := strings.Split(s, ",")
	for i, p := range parts {
		parts[i] = strings.TrimSpace(p)
	}
	return parts
}

// imageExtensions is the set of file extensions considered images inside archives.
var imageExtensions = map[string]struct{}{
	".jpg": {}, ".jpeg": {}, ".png": {}, ".webp": {}, ".gif": {},
}

// isImageFile returns true when the filename has an image extension.
func isImageFile(name string) bool {
	_, ok := imageExtensions[strings.ToLower(filepath.Ext(name))]
	return ok
}

// PageInfo describes a single page inside an archive.
type PageInfo struct {
	Index    int    `json:"index"`
	Filename string `json:"filename"`
}

// listZipImages returns image filenames from a ZIP/CBZ archive, sorted alphabetically.
func listZipImages(path string) ([]string, error) {
	r, err := zip.OpenReader(path)
	if err != nil {
		return nil, err
	}
	defer r.Close()

	var names []string
	for _, f := range r.File {
		if f.FileInfo().IsDir() {
			continue
		}
		if isImageFile(f.Name) {
			names = append(names, f.Name)
		}
	}
	sort.Strings(names)
	return names, nil
}

// listRARImages lists image filenames inside a CBR/RAR archive using bsdtar.
// The file path comes from the database and is not user-controlled.
func listRARImages(path string) ([]string, error) {
	out, err := exec.Command("bsdtar", "-tf", path).Output()
	if err != nil {
		return nil, fmt.Errorf("bsdtar -tf: %w", err)
	}
	var names []string
	for _, line := range strings.Split(string(out), "\n") {
		line = strings.TrimSpace(line)
		if line != "" && isImageFile(line) {
			names = append(names, line)
		}
	}
	sort.Strings(names)
	return names, nil
}

// ListPages returns the list of image pages inside an archive or gallery folder.
// GET /api/media/:id/pages
func (h *MediaHandler) ListPages(c *gin.Context) {
	id := c.Param("id")

	var item models.Media
	if err := h.db.Get(&item, `SELECT * FROM media WHERE id = $1 AND deleted_at IS NULL`, id); err != nil {
		respondError(c, http.StatusNotFound, "media not found")
		return
	}

	names, err := listPageNamesForMediaPath(item.FilePath, h.mediaPath)
	if err != nil {
		if errors.Is(err, errUnsupportedPagedMedia) {
			respondError(c, http.StatusBadRequest, err.Error())
			return
		}
		if errors.Is(err, os.ErrNotExist) {
			respondError(c, http.StatusNotFound, "file not found on disk")
			return
		}
		respondError(c, http.StatusInternalServerError, "list pages: "+err.Error())
		return
	}

	pages := make([]PageInfo, len(names))
	for i, n := range names {
		pages[i] = PageInfo{Index: i, Filename: filepath.Base(n)}
	}

	respondOK(c, gin.H{
		"total_pages": len(pages),
		"pages":       pages,
	}, nil)
}

// ServePage streams a single page image from an archive or gallery folder.
// GET /api/media/:id/pages/:page
func (h *MediaHandler) ServePage(c *gin.Context) {
	id := c.Param("id")
	pageStr := c.Param("page")

	pageIdx, err := strconv.Atoi(pageStr)
	if err != nil || pageIdx < 0 {
		respondError(c, http.StatusBadRequest, "invalid page index")
		return
	}

	var item models.Media
	if err := h.db.Get(&item, `SELECT * FROM media WHERE id = $1 AND deleted_at IS NULL`, id); err != nil {
		respondError(c, http.StatusNotFound, "media not found")
		return
	}

	c.Header("Cache-Control", "private, max-age=86400")

	if err := servePageForMediaPath(c, item.FilePath, h.mediaPath, pageIdx); err != nil {
		if errors.Is(err, errUnsupportedPagedMedia) {
			respondError(c, http.StatusBadRequest, err.Error())
			return
		}
		if errors.Is(err, os.ErrNotExist) {
			respondError(c, http.StatusNotFound, "file not found on disk")
			return
		}
		respondError(c, http.StatusInternalServerError, "serve page: "+err.Error())
	}
}

// serveZipPage extracts and streams a single page from a ZIP/CBZ archive.
func serveZipPage(c *gin.Context, path string, pageIdx int) error {
	r, err := zip.OpenReader(path)
	if err != nil {
		return err
	}
	defer r.Close()

	var images []*zip.File
	for _, f := range r.File {
		if !f.FileInfo().IsDir() && isImageFile(f.Name) {
			images = append(images, f)
		}
	}
	sort.Slice(images, func(i, j int) bool { return images[i].Name < images[j].Name })

	if pageIdx >= len(images) {
		c.Status(http.StatusNotFound)
		return nil
	}

	f := images[pageIdx]
	rc, err := f.Open()
	if err != nil {
		return err
	}
	defer rc.Close()

	ct := mime.TypeByExtension(strings.ToLower(filepath.Ext(f.Name)))
	if ct == "" {
		ct = "image/jpeg"
	}
	c.Header("Content-Type", ct)
	c.Status(http.StatusOK)
	_, err = io.Copy(c.Writer, rc)
	return err
}

// serveRARPage extracts and streams a single page from a CBR/RAR archive using bsdtar.
func serveRARPage(c *gin.Context, path string, pageIdx int) error {
	names, err := listRARImages(path)
	if err != nil {
		return err
	}
	if pageIdx >= len(names) {
		c.Status(http.StatusNotFound)
		return nil
	}

	target := names[pageIdx]
	cmd := exec.Command("bsdtar", "-xOf", path, target)
	out, err := cmd.Output()
	if err != nil {
		return fmt.Errorf("bsdtar -xOf: %w", err)
	}

	ct := mime.TypeByExtension(strings.ToLower(filepath.Ext(target)))
	if ct == "" {
		ct = "image/jpeg"
	}
	c.Header("Content-Type", ct)
	c.Data(http.StatusOK, ct, out)
	return nil
}

func (h *MediaHandler) replaceMediaTags(ctx context.Context, mediaID string, tagNames []string) error {
	tx, err := h.db.Beginx()
	if err != nil {
		return err
	}
	defer tx.Rollback() //nolint:errcheck

	if _, err := tx.Exec(`DELETE FROM media_tags WHERE media_id = $1`, mediaID); err != nil {
		return err
	}

	tags, err := tagrules.NewService(tx).ResolveOrCreate(ctx, tagNames)
	if err != nil {
		return err
	}

	for _, tag := range tags {
		if _, err := tx.Exec(`
			INSERT INTO media_tags (media_id, tag_id)
			VALUES ($1, $2)
			ON CONFLICT DO NOTHING
		`, mediaID, tag.ID); err != nil {
			return err
		}
	}

	return tx.Commit()
}

func removeIfExists(path string) error {
	if strings.TrimSpace(path) == "" {
		return nil
	}
	if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
		return err
	}
	return nil
}

func (h *MediaHandler) respondMedia(c *gin.Context, id string, incrementView bool) {
	item, err := h.findMediaByID(id, c.GetString("userID"), incrementView)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			respondError(c, http.StatusNotFound, "media not found")
			return
		}
		respondError(c, http.StatusInternalServerError, "load media: "+err.Error())
		return
	}
	respondOK(c, item, nil)
}

func (h *MediaHandler) tagRulesService() *tagrules.Service {
	if h.tagRules == nil {
		h.tagRules = tagrules.NewService(h.db)
	}
	return h.tagRules
}

func (h *MediaHandler) findMediaByID(id, requesterUserID string, incrementView bool) (models.Media, error) {
	var item models.Media
	if err := h.db.Get(&item, `SELECT * FROM media WHERE id = $1 AND deleted_at IS NULL`, id); err != nil {
		return models.Media{}, err
	}
	if incrementView {
		if _, err := h.db.Exec(`UPDATE media SET view_count = view_count + 1 WHERE id = $1`, id); err == nil {
			item.ViewCount++
		}
	}
	items := []models.Media{item}
	if err := h.populateMediaRelations(requesterUserID, items); err != nil {
		return models.Media{}, err
	}
	return items[0], nil
}

func (h *MediaHandler) loadMediaForThumbnail(c *gin.Context, id string) (models.Media, bool) {
	var item models.Media
	if err := h.db.Get(&item, `SELECT * FROM media WHERE id = $1 AND deleted_at IS NULL`, id); err != nil {
		respondError(c, http.StatusNotFound, "media not found")
		return models.Media{}, false
	}
	return item, true
}

func (h *MediaHandler) persistThumbnail(mediaID, previousPath string, raw []byte) error {
	if strings.TrimSpace(h.thumbPath) == "" {
		return errors.New("thumbnail path is not configured")
	}
	if len(raw) == 0 {
		return errors.New("thumbnail is empty")
	}

	img, _, err := image.Decode(bytes.NewReader(raw))
	if err != nil {
		return fmt.Errorf("decode image: %w", err)
	}

	if err := os.MkdirAll(h.thumbPath, 0o755); err != nil {
		return fmt.Errorf("create thumbnails dir: %w", err)
	}

	outPath := filepath.Join(h.thumbPath, mediaID+".jpg")
	if err := saveResizedThumbnail(img, outPath); err != nil {
		return err
	}

	if previousPath != "" && previousPath != outPath {
		cleanPrevious, err := ensureManagedPath(previousPath, h.thumbPath)
		if err != nil {
			return err
		}
		if err := removeIfExists(cleanPrevious); err != nil {
			return err
		}
	}

	if _, err := h.db.Exec(`
		UPDATE media
		SET thumbnail_path = $2, updated_at = NOW()
		WHERE id = $1 AND deleted_at IS NULL
	`, mediaID, outPath); err != nil {
		return fmt.Errorf("update media thumbnail: %w", err)
	}

	return nil
}

func saveResizedThumbnail(img image.Image, path string) error {
	const maxWidth = 600
	const maxHeight = 600
	bounds := img.Bounds()
	srcW := bounds.Dx()
	srcH := bounds.Dy()
	if srcW <= 0 || srcH <= 0 {
		return errors.New("invalid image dimensions")
	}

	scale := math.Min(float64(maxWidth)/float64(srcW), float64(maxHeight)/float64(srcH))
	if scale > 1 {
		scale = 1
	}

	dstW := int(math.Round(float64(srcW) * scale))
	dstH := int(math.Round(float64(srcH) * scale))
	if dstW < 1 {
		dstW = 1
	}
	if dstH < 1 {
		dstH = 1
	}

	dst := image.NewRGBA(image.Rect(0, 0, dstW, dstH))
	draw.BiLinear.Scale(dst, dst.Bounds(), img, bounds, draw.Over, nil)

	out, err := os.Create(path)
	if err != nil {
		return fmt.Errorf("create thumbnail: %w", err)
	}
	defer out.Close()

	if err := jpeg.Encode(out, dst, &jpeg.Options{Quality: 88}); err != nil {
		return fmt.Errorf("encode thumbnail: %w", err)
	}
	return nil
}

func (h *MediaHandler) populateMediaRelations(requesterUserID string, items []models.Media) error {
	if len(items) == 0 {
		return nil
	}

	indexByID := make(map[string]*models.Media, len(items))
	ids := make([]interface{}, 0, len(items))
	placeholders := make([]string, 0, len(items))
	for i := range items {
		indexByID[items[i].ID] = &items[i]
		ids = append(ids, items[i].ID)
		placeholders = append(placeholders, "$"+itoa(i+1))
	}

	type mediaTagRow struct {
		MediaID string `db:"media_id"`
		models.Tag
	}
	var tagRows []mediaTagRow
	tagQuery := `
		SELECT mt.media_id, t.*
		FROM media_tags mt
		JOIN tags t ON t.id = mt.tag_id
		WHERE mt.media_id IN (` + strings.Join(placeholders, ",") + `)
		ORDER BY t.name ASC
	`
	if err := h.db.Select(&tagRows, tagQuery, ids...); err != nil {
		return err
	}
	for _, row := range tagRows {
		if item := indexByID[row.MediaID]; item != nil {
			item.Tags = append(item.Tags, row.Tag)
		}
	}

	type mediaCollectionRow struct {
		MediaID string `db:"media_id"`
		ID      string `db:"id"`
		Name    string `db:"name"`
	}
	var collectionRows []mediaCollectionRow
	collectionArgs := append(append([]interface{}{}, ids...), requesterUserID)
	collectionQuery := `
		SELECT mc.media_id, c.id, c.name
		FROM media_collections mc
		JOIN collections c ON c.id = mc.collection_id
		WHERE mc.media_id IN (` + strings.Join(placeholders, ",") + `)
		  AND (c.user_id = $` + itoa(len(ids)+1) + ` OR c.user_id IS NULL)
		ORDER BY c.name ASC
	`
	if err := h.db.Select(&collectionRows, collectionQuery, collectionArgs...); err != nil {
		return err
	}
	for _, row := range collectionRows {
		if item := indexByID[row.MediaID]; item != nil {
			item.Collections = append(item.Collections, models.CollectionRef{
				ID:   row.ID,
				Name: row.Name,
			})
		}
	}

	return nil
}
