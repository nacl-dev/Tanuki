package api

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/nacl-dev/tanuki/internal/database"
	"github.com/nacl-dev/tanuki/internal/models"
)

// TagHandler handles CRUD operations for tags.
type TagHandler struct {
	db *database.DB
}

const tagUsageSelect = `
	SELECT
		t.id,
		t.name,
		t.category,
		COALESCE(COUNT(m.id), 0) AS usage_count
	FROM tags t
	LEFT JOIN media_tags mt ON mt.tag_id = t.id
	LEFT JOIN media m ON m.id = mt.media_id AND m.deleted_at IS NULL
`

// List returns all tags, optionally filtered by category.
// GET /api/tags?category=artist
func (h *TagHandler) List(c *gin.Context) {
	category := c.Query("category")

	var tags []models.Tag
	var err error

	if category != "" {
		err = h.db.Select(&tags, tagUsageSelect+`
			WHERE t.category = $1
			GROUP BY t.id, t.name, t.category
			ORDER BY usage_count DESC, t.name ASC
		`, category)
	} else {
		err = h.db.Select(&tags, tagUsageSelect+`
			GROUP BY t.id, t.name, t.category
			ORDER BY usage_count DESC, t.name ASC
		`)
	}

	if err != nil {
		respondError(c, http.StatusInternalServerError, "query tags: "+err.Error())
		return
	}

	respondOK(c, tags, &Meta{Total: len(tags)})
}

// Search provides autocomplete suggestions for tag names.
// GET /api/tags/search?q=blon
func (h *TagHandler) Search(c *gin.Context) {
	q := c.Query("q")
	if q == "" {
		respondOK(c, []models.Tag{}, nil)
		return
	}

	var tags []models.Tag
	if err := h.db.Select(&tags, `
	`+tagUsageSelect+`
		WHERE t.name ILIKE $1
		GROUP BY t.id, t.name, t.category
		ORDER BY usage_count DESC, t.name ASC
		LIMIT 20
	`, q+"%"); err != nil {
		respondError(c, http.StatusInternalServerError, "search tags: "+err.Error())
		return
	}

	respondOK(c, tags, nil)
}

// Create adds a new tag.
// POST /api/tags
func (h *TagHandler) Create(c *gin.Context) {
	var body struct {
		Name     string             `json:"name"     binding:"required"`
		Category models.TagCategory `json:"category" binding:"required"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		respondError(c, http.StatusBadRequest, err.Error())
		return
	}

	id := uuid.NewString()
	if _, err := h.db.Exec(`
		INSERT INTO tags (id, name, category, usage_count)
		VALUES ($1, $2, $3, 0)
		ON CONFLICT (name) DO NOTHING
	`, id, body.Name, body.Category); err != nil {
		respondError(c, http.StatusInternalServerError, "create tag: "+err.Error())
		return
	}

	var tag models.Tag
	if err := h.db.Get(&tag, tagUsageSelect+`
		WHERE t.name = $1
		GROUP BY t.id, t.name, t.category
	`, body.Name); err != nil {
		respondError(c, http.StatusInternalServerError, "fetch tag: "+err.Error())
		return
	}

	c.JSON(http.StatusCreated, envelope{Data: tag})
}

// Update renames a tag or changes its category.
// PATCH /api/tags/:id
func (h *TagHandler) Update(c *gin.Context) {
	id := c.Param("id")

	var body struct {
		Name     *string             `json:"name"`
		Category *models.TagCategory `json:"category"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		respondError(c, http.StatusBadRequest, err.Error())
		return
	}

	if _, err := h.db.Exec(`
		UPDATE tags SET
			name     = COALESCE($2, name),
			category = COALESCE($3, category)
		WHERE id = $1
	`, id, body.Name, body.Category); err != nil {
		respondError(c, http.StatusInternalServerError, "update tag: "+err.Error())
		return
	}

	var tag models.Tag
	if err := h.db.Get(&tag, tagUsageSelect+`
		WHERE t.id = $1
		GROUP BY t.id, t.name, t.category
	`, id); err != nil {
		respondError(c, http.StatusNotFound, "tag not found")
		return
	}

	respondOK(c, tag, nil)
}

// Delete removes a tag and its associations.
// DELETE /api/tags/:id
func (h *TagHandler) Delete(c *gin.Context) {
	id := c.Param("id")

	tx, err := h.db.Beginx()
	if err != nil {
		respondError(c, http.StatusInternalServerError, "begin tx: "+err.Error())
		return
	}
	defer tx.Rollback() //nolint:errcheck

	if _, err := tx.Exec(`DELETE FROM media_tags WHERE tag_id = $1`, id); err != nil {
		respondError(c, http.StatusInternalServerError, "delete media_tags: "+err.Error())
		return
	}
	if _, err := tx.Exec(`DELETE FROM tags WHERE id = $1`, id); err != nil {
		respondError(c, http.StatusInternalServerError, "delete tag: "+err.Error())
		return
	}
	if err := tx.Commit(); err != nil {
		respondError(c, http.StatusInternalServerError, "commit: "+err.Error())
		return
	}

	c.Status(http.StatusNoContent)
}
