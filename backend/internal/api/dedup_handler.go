package api

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/nacl-dev/tanuki/internal/database"
	"github.com/nacl-dev/tanuki/internal/dedup"
	"github.com/nacl-dev/tanuki/internal/models"
	"go.uber.org/zap"
)

// DedupHandler handles duplicate-detection endpoints.
type DedupHandler struct {
	db      *database.DB
	service *dedup.Service
}

// newDedupHandler creates a DedupHandler using the provided threshold and logger.
func newDedupHandler(db *database.DB, threshold int, log *zap.Logger) *DedupHandler {
	return &DedupHandler{
		db:      db,
		service: dedup.NewService(db, threshold, log),
	}
}

// GetDuplicates handles GET /api/media/:id/duplicates
func (h *DedupHandler) GetDuplicates(c *gin.Context) {
	id := c.Param("id")
	duplicates, err := h.service.FindDuplicates(c.Request.Context(), id)
	if err != nil {
		respondError(c, http.StatusInternalServerError, "duplicate search failed: "+err.Error())
		return
	}
	if duplicates == nil {
		duplicates = []dedup.DuplicateItem{}
	}
	respondOK(c, duplicates, &Meta{Total: len(duplicates)})
}

// ListDuplicates handles GET /api/duplicates
func (h *DedupHandler) ListDuplicates(c *gin.Context) {
	groups, err := h.service.ListDuplicateGroups(c.Request.Context())
	if err != nil {
		respondError(c, http.StatusInternalServerError, "list duplicates failed: "+err.Error())
		return
	}
	if groups == nil {
		groups = []dedup.DuplicateGroup{}
	}
	respondOK(c, groups, &Meta{Total: len(groups)})
}

// ResolveDuplicates handles POST /api/duplicates/resolve
func (h *DedupHandler) ResolveDuplicates(c *gin.Context) {
	var body struct {
		KeepID    string   `json:"keep_id"    binding:"required"`
		DeleteIDs []string `json:"delete_ids" binding:"required"`
		MergeTags bool     `json:"merge_tags"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		respondError(c, http.StatusBadRequest, "invalid request body: "+err.Error())
		return
	}

	ctx := c.Request.Context()

	if body.MergeTags {
		// Copy tags from each deleted item to the kept item
		for _, delID := range body.DeleteIDs {
			if _, err := h.db.ExecContext(ctx, `
				INSERT INTO media_tags (media_id, tag_id)
				SELECT $1, tag_id FROM media_tags WHERE media_id = $2
				ON CONFLICT DO NOTHING
			`, body.KeepID, delID); err != nil {
				zap.L().Warn("resolve duplicates: merge tags", zap.String("from", delID), zap.Error(err))
			}
		}
	}

	// Soft-delete the duplicate items
	deleted := 0
	for _, delID := range body.DeleteIDs {
		res, err := h.db.ExecContext(ctx,
			`UPDATE media SET deleted_at = NOW(), updated_at = NOW() WHERE id = $1 AND deleted_at IS NULL`,
			delID,
		)
		if err != nil {
			zap.L().Error("resolve duplicates: soft delete", zap.String("id", delID), zap.Error(err))
			continue
		}
		n, _ := res.RowsAffected()
		deleted += int(n)
	}

	respondOK(c, gin.H{"deleted": deleted, "kept": body.KeepID}, nil)
}

// ComputePHash handles POST /api/media/:id/phash
func (h *DedupHandler) ComputePHash(c *gin.Context) {
	id := c.Param("id")

	var item models.Media
	if err := h.db.QueryRowxContext(c.Request.Context(),
		`SELECT * FROM media WHERE id = $1 AND deleted_at IS NULL`, id,
	).StructScan(&item); err != nil {
		respondError(c, http.StatusNotFound, "media not found")
		return
	}

	if err := h.service.ComputeAndStore(c.Request.Context(), &item); err != nil {
		respondError(c, http.StatusInternalServerError, "phash computation failed: "+err.Error())
		return
	}

	// Re-fetch to return updated phash
	_ = h.db.QueryRowxContext(c.Request.Context(),
		`SELECT * FROM media WHERE id = $1`, id,
	).StructScan(&item)

	respondOK(c, gin.H{"id": item.ID, "phash": item.PHash}, nil)
}
