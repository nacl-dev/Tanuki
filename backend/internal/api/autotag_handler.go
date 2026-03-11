package api

import (
	"context"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/nacl-dev/tanuki/internal/autotag"
	"github.com/nacl-dev/tanuki/internal/config"
	"github.com/nacl-dev/tanuki/internal/database"
	"github.com/nacl-dev/tanuki/internal/models"
	"github.com/nacl-dev/tanuki/internal/taskqueue"
	"go.uber.org/zap"
)

// AutoTagHandler handles auto-tagging endpoints.
type AutoTagHandler struct {
	db      *database.DB
	service *autotag.Service
	tasks   *taskqueue.Manager
}

// newAutoTagHandler creates an AutoTagHandler using the provided config and logger.
func newAutoTagHandler(db *database.DB, cfg *config.Config, log *zap.Logger, tasks *taskqueue.Manager) *AutoTagHandler {
	svc := autotag.NewService(db, autotag.Config{
		SauceNAOAPIKey: cfg.SauceNAOAPIKey,
		IQDBEnabled:    cfg.IQDBEnabled,
		Threshold:      float64(cfg.AutoTagSimilarityThreshold),
		RateLimitMs:    cfg.AutoTagRateLimitMs,
	}, log)
	return &AutoTagHandler{db: db, service: svc, tasks: tasks}
}

// AutoTag handles POST /api/media/:id/autotag
func (h *AutoTagHandler) AutoTag(c *gin.Context) {
	id := c.Param("id")

	var body struct {
		Force     bool                   `json:"force"`
		ApplyTags []autotag.SuggestedTag `json:"apply_tags"`
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

	task := h.tasks.Start("media.autotag_batch", c.GetString("userID"), map[string]any{
		"queued":       len(ids),
		"all_untagged": body.AllUntagged,
	}, func(ctx context.Context, handle *taskqueue.Handle) (any, error) {
		total := len(ids)
		successCount := 0
		failedCount := 0
		handle.SetProgress(0, total)

		for index, id := range ids {
			handle.SetMessage("Auto-tagging item %d of %d", index+1, total)
			if err := h.service.MarkProcessing(ctx, id); err != nil {
				failedCount++
				handle.Increment(total)
				continue
			}

			var item models.Media
			if err := h.db.QueryRowxContext(
				ctx,
				`SELECT * FROM media WHERE id = $1 AND deleted_at IS NULL`,
				id,
			).StructScan(&item); err != nil {
				failedCount++
				_ = h.service.MarkFailed(ctx, id)
				handle.Increment(total)
				continue
			}

			result, err := h.service.AutoTag(ctx, &item)
			if err != nil || result == nil || result.Source == "none" {
				failedCount++
				_ = h.service.MarkFailed(ctx, id)
				handle.Increment(total)
				continue
			}
			if err := h.service.ApplyTags(ctx, id, result, result.SuggestedTags); err != nil {
				failedCount++
				_ = h.service.MarkFailed(ctx, id)
				handle.Increment(total)
				continue
			}

			successCount++
			handle.Increment(total)
		}

		handle.SetMessage("Batch auto-tag completed")
		return gin.H{
			"queued":    total,
			"completed": successCount,
			"failed":    failedCount,
		}, nil
	})

	respondAccepted(c, gin.H{"queued": len(ids), "task_id": task.ID}, nil)
}
