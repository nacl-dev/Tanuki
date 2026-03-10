package api

import (
	"archive/zip"
	"fmt"
	"io"
	"mime"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/nacl-dev/tanuki/internal/database"
	"github.com/nacl-dev/tanuki/internal/models"
)

// MediaHandler handles CRUD operations for media items.
type MediaHandler struct {
	db *database.DB
}

// List returns a paginated list of media items with optional filtering.
// GET /api/media?page=1&limit=50&type=video&q=search&favorite=true&tag=artist&tags=a,b&sort=newest&min_rating=3
func (h *MediaHandler) List(c *gin.Context) {
	type query struct {
		Page      int    `form:"page"       binding:"-"`
		Limit     int    `form:"limit"      binding:"-"`
		Type      string `form:"type"       binding:"-"`
		Q         string `form:"q"          binding:"-"`
		Favorite  *bool  `form:"favorite"   binding:"-"`
		Tag       string `form:"tag"        binding:"-"`
		Tags      string `form:"tags"       binding:"-"`
		Sort      string `form:"sort"       binding:"-"`
		MinRating *int   `form:"min_rating" binding:"-"`
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
	whereClause := ` WHERE m.deleted_at IS NULL`
	args := []interface{}{}
	argIdx := 1

	if q.Type != "" {
		whereClause += ` AND m.type = $` + itoa(argIdx)
		args = append(args, q.Type)
		argIdx++
	}
	if q.Q != "" {
		whereClause += ` AND m.title ILIKE $` + itoa(argIdx)
		args = append(args, "%"+q.Q+"%")
		argIdx++
	}
	if q.Favorite != nil {
		whereClause += ` AND m.favorite = $` + itoa(argIdx)
		args = append(args, *q.Favorite)
		argIdx++
	}
	if q.MinRating != nil {
		whereClause += ` AND m.rating >= $` + itoa(argIdx)
		args = append(args, *q.MinRating)
		argIdx++
	}
	// Single tag filter.
	if q.Tag != "" {
		whereClause += ` AND m.id IN (SELECT mt.media_id FROM media_tags mt JOIN tags t ON t.id = mt.tag_id WHERE t.name = $` + itoa(argIdx) + `)`
		args = append(args, q.Tag)
		argIdx++
	}
	// Multi-tag filter (AND logic): each tag listed in ?tags= must be present.
	if q.Tags != "" {
		for _, tag := range splitTags(q.Tags) {
			if tag == "" {
				continue
			}
			whereClause += ` AND m.id IN (SELECT mt.media_id FROM media_tags mt JOIN tags t ON t.id = mt.tag_id WHERE t.name = $` + itoa(argIdx) + `)`
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

	respondOK(c, items, &Meta{Page: q.Page, Total: total})
}

// Get returns a single media item by its UUID.
// GET /api/media/:id
func (h *MediaHandler) Get(c *gin.Context) {
	id := c.Param("id")

	var item models.Media
	if err := h.db.Get(&item, `SELECT * FROM media WHERE id = $1 AND deleted_at IS NULL`, id); err != nil {
		respondError(c, http.StatusNotFound, "media not found")
		return
	}

	// Increment view count asynchronously; errors are non-fatal.
	h.db.Exec(`UPDATE media SET view_count = view_count + 1 WHERE id = $1`, id) //nolint:errcheck

	// Load associated tags.
	if err := h.db.Select(&item.Tags, `
		SELECT t.* FROM tags t
		JOIN media_tags mt ON mt.tag_id = t.id
		WHERE mt.media_id = $1
	`, id); err != nil {
		respondError(c, http.StatusInternalServerError, "load tags: "+err.Error())
		return
	}

	respondOK(c, item, nil)
}

// Update patches mutable fields of a media item.
// PATCH /api/media/:id
func (h *MediaHandler) Update(c *gin.Context) {
	id := c.Param("id")

	var body struct {
		Title        *string `json:"title"`
		Rating       *int    `json:"rating"`
		Favorite     *bool   `json:"favorite"`
		Language     *string `json:"language"`
		SourceURL    *string `json:"source_url"`
		ReadProgress *int    `json:"read_progress"`
		ReadTotal    *int    `json:"read_total"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		respondError(c, http.StatusBadRequest, err.Error())
		return
	}

	_, err := h.db.Exec(`
		UPDATE media SET
			title         = COALESCE($2, title),
			rating        = COALESCE($3, rating),
			favorite      = COALESCE($4, favorite),
			language      = COALESCE($5, language),
			source_url    = COALESCE($6, source_url),
			read_progress = COALESCE($7, read_progress),
			read_total    = COALESCE($8, read_total),
			updated_at    = NOW()
		WHERE id = $1 AND deleted_at IS NULL
	`, id, body.Title, body.Rating, body.Favorite, body.Language, body.SourceURL,
		body.ReadProgress, body.ReadTotal)
	if err != nil {
		respondError(c, http.StatusInternalServerError, "update media: "+err.Error())
		return
	}

	h.Get(c) // Return the updated record.
}

// Delete soft-deletes a media item.
// DELETE /api/media/:id
func (h *MediaHandler) Delete(c *gin.Context) {
	id := c.Param("id")

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

	if _, err := os.Stat(item.FilePath); os.IsNotExist(err) {
		respondError(c, http.StatusNotFound, "file not found on disk")
		return
	}

	c.Header("Cache-Control", "public, max-age=86400")
	c.File(item.FilePath)
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

	if item.ThumbnailPath == "" {
		c.Status(http.StatusNotFound)
		return
	}

	if _, err := os.Stat(item.ThumbnailPath); os.IsNotExist(err) {
		c.Status(http.StatusNotFound)
		return
	}

	c.Header("Cache-Control", "public, max-age=86400")
	c.File(item.ThumbnailPath)
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

// listCBRImages lists image filenames inside a CBR/RAR archive using unrar.
// The file path comes from the database and is not user-controlled.
// exec.Command passes arguments directly to execve (no shell), so shell
// metacharacters in the path are not interpreted.
func listCBRImages(path string) ([]string, error) {
	out, err := exec.Command("unrar", "lb", "--", path).Output()
	if err != nil {
		return nil, fmt.Errorf("unrar lb: %w", err)
	}
	var names []string
	for _, line := range strings.Split(string(out), "\n") {
		line = strings.TrimSpace(line)
		if line != "" && isImageFile(line) && !strings.HasPrefix(line, "-") {
			names = append(names, line)
		}
	}
	sort.Strings(names)
	return names, nil
}

// ListPages returns the list of image pages inside an archive.
// GET /api/media/:id/pages
func (h *MediaHandler) ListPages(c *gin.Context) {
	id := c.Param("id")

	var item models.Media
	if err := h.db.Get(&item, `SELECT * FROM media WHERE id = $1 AND deleted_at IS NULL`, id); err != nil {
		respondError(c, http.StatusNotFound, "media not found")
		return
	}

	ext := strings.ToLower(filepath.Ext(item.FilePath))
	var names []string
	var err error

	switch ext {
	case ".zip", ".cbz":
		names, err = listZipImages(item.FilePath)
	case ".cbr":
		names, err = listCBRImages(item.FilePath)
	default:
		respondError(c, http.StatusBadRequest, "media is not an archive type")
		return
	}

	if err != nil {
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

// ServePage streams a single page image from an archive.
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

	ext := strings.ToLower(filepath.Ext(item.FilePath))
	c.Header("Cache-Control", "public, max-age=86400")

	switch ext {
	case ".zip", ".cbz":
		if err := serveZipPage(c, item.FilePath, pageIdx); err != nil {
			respondError(c, http.StatusInternalServerError, "serve page: "+err.Error())
		}
	case ".cbr":
		if err := serveCBRPage(c, item.FilePath, pageIdx); err != nil {
			respondError(c, http.StatusInternalServerError, "serve page: "+err.Error())
		}
	default:
		respondError(c, http.StatusBadRequest, "media is not an archive type")
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

// serveCBRPage extracts and streams a single page from a CBR/RAR archive using unrar.
func serveCBRPage(c *gin.Context, path string, pageIdx int) error {
	names, err := listCBRImages(path)
	if err != nil {
		return err
	}
	if pageIdx >= len(names) {
		c.Status(http.StatusNotFound)
		return nil
	}

	target := names[pageIdx]
	// Pass "--" before filename arguments so unrar cannot misinterpret
	// archive-embedded filenames as command-line flags.
	cmd := exec.Command("unrar", "p", "-inul", "--", path, target)
	out, err := cmd.Output()
	if err != nil {
		return fmt.Errorf("unrar p: %w", err)
	}

	ct := mime.TypeByExtension(strings.ToLower(filepath.Ext(target)))
	if ct == "" {
		ct = "image/jpeg"
	}
	c.Header("Content-Type", ct)
	c.Data(http.StatusOK, ct, out)
	return nil
}
