package api

import (
	"context"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/nacl-dev/tanuki/internal/autotag"
	"github.com/nacl-dev/tanuki/internal/config"
	"github.com/nacl-dev/tanuki/internal/database"
	"github.com/nacl-dev/tanuki/internal/models"
	"go.uber.org/zap"
)

// AutoTagHandler handles auto-tagging endpoints.
type AutoTagHandler struct {
	db      *database.DB
	service *autotag.Service
}

// newAutoTagHandler creates an AutoTagHandler using the provided config and logger.
func newAutoTagHandler(db *database.DB, cfg *config.Config, log *zap.Logger) *AutoTagHandler {
	svc := autotag.NewService(db, autotag.Config{
		SauceNAOAPIKey: cfg.SauceNAOAPIKey,
		IQDBEnabled:    cfg.IQDBEnabled,
		Threshold:      float64(cfg.AutoTagSimilarityThreshold),
		RateLimitMs:    cfg.AutoTagRateLimitMs,
	}, log)
	return &AutoTagHandler{db: db, service: svc}
}

// AutoTag handles POST /api/media/:id/autotag
func (h *AutoTagHandler) AutoTag(c *gin.Context) {
	id := c.Param("id")

	var body struct {
		Force       bool             `json:"force"`
		ApplyTags   []autotag.SuggestedTag `json:"apply_tags"`
	}
	_ = c.ShouldBindJSON(&body) // optional body

	// Fetch the media item
	var item models.Media
	if err := h.db.QueryRowxContext(c.Request.Context(),
		`SELECT * FROM media WHERE id = $1 AND deleted_at IS NULL`, id,
	).StructScan(&item); err != nil {
		respondError(c, http.StatusNotFound, "media not found")
		return
	}

	// Skip if already completed and force is false
	if item.AutoTagStatus == models.AutoTagStatusCompleted && !body.Force {
		respondError(c, http.StatusConflict, "already auto-tagged; use force=true to re-run")
		return
	}

	// Perform reverse image search
	result, err := h.service.AutoTag(c.Request.Context(), &item)
	if err != nil {
		respondError(c, http.StatusInternalServerError, "auto-tag failed: "+err.Error())
		return
	}

	// If apply_tags were provided in the request, persist them immediately.
	if len(body.ApplyTags) > 0 {
		if err := h.service.ApplyTags(c.Request.Context(), id, result, body.ApplyTags); err != nil {
			respondError(c, http.StatusInternalServerError, "apply tags failed: "+err.Error())
			return
		}
	}

	respondOK(c, gin.H{
		"suggested_tags": result.SuggestedTags,
		"source":         result.Source,
		"similarity":     result.Similarity,
		"source_url":     result.SourceURL,
	}, nil)
}

// AutoTagBatch handles POST /api/media/autotag/batch
func (h *AutoTagHandler) AutoTagBatch(c *gin.Context) {
	var body struct {
		IDs         []string `json:"ids"`
		AllUntagged bool     `json:"all_untagged"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		respondError(c, http.StatusBadRequest, "invalid request body")
		return
	}

	ctx := c.Request.Context()

	var ids []string
	if body.AllUntagged {
		var rows []struct {
			ID string `db:"id"`
		}
		if err := h.db.SelectContext(ctx, &rows, `
			SELECT id FROM media
			WHERE deleted_at IS NULL
			  AND auto_tag_status IN ('pending', 'failed')
			  AND (type = 'image' OR thumbnail_path != '')
			ORDER BY created_at DESC
			LIMIT 500
		`); err != nil {
			respondError(c, http.StatusInternalServerError, "query failed")
			return
		}
		for _, r := range rows {
			ids = append(ids, r.ID)
		}
	} else {
		ids = body.IDs
	}

	if len(ids) == 0 {
		respondOK(c, gin.H{"queued": 0}, nil)
		return
	}

	// Mark all selected items as 'processing' (lightweight queue via DB status)
	for _, id := range ids {
		if _, err := h.db.ExecContext(ctx,
			`UPDATE media SET auto_tag_status = 'processing', updated_at = NOW() WHERE id = $1`, id,
		); err != nil {
			zap.L().Warn("autotag batch: mark processing", zap.String("id", id), zap.Error(err))
		}
	}

	// Process in a goroutine so the response returns quickly
	go func() {
		bgCtx := context.Background()
		for _, id := range ids {
			var item models.Media
			if err := h.db.QueryRowx(
				`SELECT * FROM media WHERE id = $1 AND deleted_at IS NULL`, id,
			).StructScan(&item); err != nil {
				continue
			}
			result, err := h.service.AutoTag(bgCtx, &item)
			if err != nil || result == nil || result.Source == "none" {
				_ = h.service.MarkFailed(bgCtx, id)
				continue
			}
			_ = h.service.ApplyTags(bgCtx, id, result, result.SuggestedTags)
		}
	}()

	respondOK(c, gin.H{"queued": len(ids)}, nil)
}
