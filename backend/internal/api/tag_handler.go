package api

import (
	"database/sql"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/nacl-dev/tanuki/internal/database"
	"github.com/nacl-dev/tanuki/internal/models"
	"github.com/nacl-dev/tanuki/internal/tagrules"
)

// TagHandler handles CRUD operations for tags.
type TagHandler struct {
	db    *database.DB
	rules *tagrules.Service
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
	q := strings.TrimSpace(c.Query("q"))
	if q == "" {
		respondOK(c, []models.Tag{}, nil)
		return
	}

	lowerQ := strings.ToLower(q)
	namespace, remainder, hasNamespace := strings.Cut(lowerQ, ":")
	if hasNamespace {
		if category, ok := models.TagCategoryForNamespace(namespace); ok {
			tags, err := h.searchWithinCategory(category, strings.TrimSpace(remainder))
			if err != nil {
				respondError(c, http.StatusInternalServerError, "search tags: "+err.Error())
				return
			}
			respondOK(c, tags, nil)
			return
		}
	}

	var tags []models.Tag
	if err := h.db.Select(&tags, `
	`+tagUsageSelect+`
		WHERE t.name ILIKE $1 OR t.name ILIKE $2
		   OR EXISTS (
				SELECT 1
				FROM tag_aliases ta
				WHERE ta.tag_id = t.id
				  AND (ta.alias_name ILIKE $1 OR ta.alias_name ILIKE $2)
		   )
		GROUP BY t.id, t.name, t.category
		ORDER BY
			CASE
				WHEN t.name ILIKE $1 THEN 0
				WHEN EXISTS (
					SELECT 1 FROM tag_aliases ta
					WHERE ta.tag_id = t.id AND ta.alias_name ILIKE $1
				) THEN 1
				ELSE 2
			END,
			usage_count DESC,
			t.name ASC
		LIMIT 20
	`, q+"%", "%"+q+"%"); err != nil {
		respondError(c, http.StatusInternalServerError, "search tags: "+err.Error())
		return
	}

	respondOK(c, tags, nil)
}

func (h *TagHandler) searchWithinCategory(category models.TagCategory, term string) ([]models.Tag, error) {
	var tags []models.Tag
	if term == "" {
		err := h.db.Select(&tags, `
	`+tagUsageSelect+`
			WHERE t.category = $1
			GROUP BY t.id, t.name, t.category
			ORDER BY usage_count DESC, t.name ASC
			LIMIT 20
		`, category)
		return tags, err
	}

	err := h.db.Select(&tags, `
	`+tagUsageSelect+`
		WHERE t.category = $1
		  AND (
			t.name ILIKE $2 OR t.name ILIKE $3
			OR EXISTS (
				SELECT 1
				FROM tag_aliases ta
				WHERE ta.tag_id = t.id
				  AND (ta.alias_name ILIKE $2 OR ta.alias_name ILIKE $3)
			)
		  )
		GROUP BY t.id, t.name, t.category
		ORDER BY
			CASE
				WHEN t.name ILIKE $2 THEN 0
				WHEN EXISTS (
					SELECT 1 FROM tag_aliases ta
					WHERE ta.tag_id = t.id AND ta.alias_name ILIKE $2
				) THEN 1
				ELSE 2
			END,
			usage_count DESC,
			t.name ASC
		LIMIT 20
	`, category, term+"%", "%"+term+"%")
	return tags, err
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

func (h *TagHandler) ruleService() *tagrules.Service {
	if h.rules == nil {
		h.rules = tagrules.NewService(h.db)
	}
	return h.rules
}

func (h *TagHandler) hydrateAliasRules(rows []models.TagAlias) error {
	for i := range rows {
		var tag models.Tag
		if err := h.db.Get(&tag, `
			SELECT id, name, category, usage_count
			FROM tags
			WHERE id = $1
		`, rows[i].TagID); err != nil {
			return err
		}
		rows[i].Tag = tag
	}
	return nil
}

func (h *TagHandler) hydrateImplicationRules(rows []models.TagImplication) error {
	for i := range rows {
		var source models.Tag
		if err := h.db.Get(&source, `
			SELECT id, name, category, usage_count
			FROM tags
			WHERE id = $1
		`, rows[i].TagID); err != nil {
			return err
		}
		var implied models.Tag
		if err := h.db.Get(&implied, `
			SELECT id, name, category, usage_count
			FROM tags
			WHERE id = $1
		`, rows[i].ImpliedTagID); err != nil {
			return err
		}
		rows[i].Tag = source
		rows[i].ImpliedTag = implied
	}
	return nil
}

func (h *TagHandler) resolveRuleTarget(c *gin.Context, raw string) (models.Tag, bool) {
	tag, err := h.ruleService().ResolveExistingOrCreate(c.Request.Context(), raw)
	if err == nil {
		return tag, true
	}
	if err == sql.ErrNoRows {
		respondError(c, http.StatusBadRequest, "tag is required")
		return models.Tag{}, false
	}
	respondError(c, http.StatusInternalServerError, "resolve tag: "+err.Error())
	return models.Tag{}, false
}
