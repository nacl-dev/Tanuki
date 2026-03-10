package api

import (
	"net/http"
	"os"

	"github.com/gin-gonic/gin"
	"github.com/nacl-dev/tanuki/internal/database"
	"github.com/nacl-dev/tanuki/internal/models"
)

// MediaHandler handles CRUD operations for media items.
type MediaHandler struct {
	db *database.DB
}

// List returns a paginated list of media items with optional filtering.
// GET /api/media?page=1&limit=50&type=video&q=search&favorite=true
func (h *MediaHandler) List(c *gin.Context) {
	type query struct {
		Page     int    `form:"page"     binding:"-"`
		Limit    int    `form:"limit"    binding:"-"`
		Type     string `form:"type"     binding:"-"`
		Q        string `form:"q"        binding:"-"`
		Favorite *bool  `form:"favorite" binding:"-"`
		Tag      string `form:"tag"      binding:"-"`
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

	// Count query applies the same filters.
	var total int
	countQuery := `SELECT COUNT(*) FROM media m` + whereClause
	if err := h.db.QueryRow(countQuery, args...).Scan(&total); err != nil {
		respondError(c, http.StatusInternalServerError, "count media: "+err.Error())
		return
	}

	sqlQuery := `SELECT m.* FROM media m` + whereClause +
		` ORDER BY m.created_at DESC LIMIT $` + itoa(argIdx) + ` OFFSET $` + itoa(argIdx+1)
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
		Title     *string `json:"title"`
		Rating    *int    `json:"rating"`
		Favorite  *bool   `json:"favorite"`
		Language  *string `json:"language"`
		SourceURL *string `json:"source_url"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		respondError(c, http.StatusBadRequest, err.Error())
		return
	}

	_, err := h.db.Exec(`
		UPDATE media SET
			title      = COALESCE($2, title),
			rating     = COALESCE($3, rating),
			favorite   = COALESCE($4, favorite),
			language   = COALESCE($5, language),
			source_url = COALESCE($6, source_url),
			updated_at = NOW()
		WHERE id = $1 AND deleted_at IS NULL
	`, id, body.Title, body.Rating, body.Favorite, body.Language, body.SourceURL)
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

