package api

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/nacl-dev/tanuki/internal/database"
	"github.com/nacl-dev/tanuki/internal/downloader"
	"github.com/nacl-dev/tanuki/internal/models"
)

// ScheduleHandler manages recurring download schedules.
type ScheduleHandler struct {
	db           *database.DB
	downloadsDir string
	mediaPath    string
}

// List returns all download schedules.
// GET /api/schedules
func (h *ScheduleHandler) List(c *gin.Context) {
	userID := c.GetString("userID")

	var schedules []models.DownloadSchedule
	if err := h.db.Select(&schedules, `
		SELECT * FROM download_schedules
		WHERE user_id = $1 OR user_id IS NULL
		ORDER BY created_at DESC
	`, userID); err != nil {
		respondError(c, http.StatusInternalServerError, "query schedules: "+err.Error())
		return
	}

	respondOK(c, schedules, &Meta{Total: len(schedules)})
}

// Create adds a new scheduled download.
// POST /api/schedules
func (h *ScheduleHandler) Create(c *gin.Context) {
	var body struct {
		Name            string   `json:"name"             binding:"required"`
		URLPattern      string   `json:"url_pattern"      binding:"required"`
		SourceType      string   `json:"source_type"      binding:"-"`
		CronExpression  string   `json:"cron_expression"  binding:"required"`
		TargetDirectory string   `json:"target_directory" binding:"-"`
		DefaultTags     []string `json:"default_tags"     binding:"-"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		respondError(c, http.StatusBadRequest, err.Error())
		return
	}
	cronExpression, nextRun, err := downloader.ValidateCronExpression(body.CronExpression)
	if err != nil {
		respondError(c, http.StatusBadRequest, "invalid cron expression")
		return
	}
	targetDirectory, err := downloader.NormalizeTargetDirectory(body.TargetDirectory, h.downloadsDir, h.mediaPath)
	if err != nil {
		respondError(c, http.StatusBadRequest, err.Error())
		return
	}
	_, defaultTagsRaw, err := prepareAutoTags(c.Request.Context(), h.db, body.DefaultTags)
	if err != nil {
		respondError(c, http.StatusInternalServerError, "prepare default tags: "+err.Error())
		return
	}

	id := uuid.NewString()
	if _, err := h.db.Exec(`
		INSERT INTO download_schedules
			(id, user_id, name, url_pattern, source_type, cron_expression, enabled, default_tags, target_directory, next_run)
		VALUES ($1, $2, $3, $4, $5, $6, true, $7, $8, $9)
	`, id, stringPtr(c.GetString("userID")), body.Name, body.URLPattern, body.SourceType, cronExpression, defaultTagsRaw, targetDirectory, nextRun); err != nil {
		respondError(c, http.StatusInternalServerError, "create schedule: "+err.Error())
		return
	}

	var sched models.DownloadSchedule
	if err := h.db.Get(&sched, `
		SELECT * FROM download_schedules
		WHERE id = $1 AND (user_id = $2 OR user_id IS NULL)
	`, id, c.GetString("userID")); err != nil {
		respondError(c, http.StatusInternalServerError, "fetch schedule: "+err.Error())
		return
	}

	c.JSON(http.StatusCreated, envelope{Data: sched})
}

// Update modifies a schedule.
// PATCH /api/schedules/:id
func (h *ScheduleHandler) Update(c *gin.Context) {
	id := c.Param("id")
	userID := c.GetString("userID")

	var body struct {
		Name            *string   `json:"name"`
		CronExpression  *string   `json:"cron_expression"`
		Enabled         *bool     `json:"enabled"`
		TargetDirectory *string   `json:"target_directory"`
		DefaultTags     *[]string `json:"default_tags"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		respondError(c, http.StatusBadRequest, err.Error())
		return
	}

	var current models.DownloadSchedule
	if err := h.db.Get(&current, `
		SELECT * FROM download_schedules
		WHERE id = $1 AND (user_id = $2 OR user_id IS NULL)
	`, id, userID); err != nil {
		respondError(c, http.StatusNotFound, "schedule not found")
		return
	}

	var targetDirectory any
	if body.TargetDirectory != nil {
		normalizedTargetDirectory, err := downloader.NormalizeTargetDirectory(*body.TargetDirectory, h.downloadsDir, h.mediaPath)
		if err != nil {
			respondError(c, http.StatusBadRequest, err.Error())
			return
		}
		targetDirectory = normalizedTargetDirectory
	}

	cronExpression := current.CronExpression
	if body.CronExpression != nil {
		normalized, _, err := downloader.ValidateCronExpression(*body.CronExpression)
		if err != nil {
			respondError(c, http.StatusBadRequest, "invalid cron expression")
			return
		}
		cronExpression = normalized
	}

	enabled := current.Enabled
	if body.Enabled != nil {
		enabled = *body.Enabled
	}
	var nextRun any
	if enabled {
		_, computedNextRun, err := downloader.ValidateCronExpression(cronExpression)
		if err == nil {
			nextRun = computedNextRun
		}
	}
	updateDefaultTags := body.DefaultTags != nil
	var defaultTagsRaw any
	if updateDefaultTags {
		_, payload, err := prepareAutoTags(c.Request.Context(), h.db, *body.DefaultTags)
		if err != nil {
			respondError(c, http.StatusInternalServerError, "prepare default tags: "+err.Error())
			return
		}
		defaultTagsRaw = payload
	}

	if _, err := h.db.Exec(`
		UPDATE download_schedules SET
			name            = COALESCE($2, name),
			cron_expression = COALESCE($3, cron_expression),
			enabled         = COALESCE($4, enabled),
			target_directory = COALESCE($5, target_directory),
			default_tags    = CASE WHEN $6 THEN $7 ELSE default_tags END,
			next_run        = $8,
			updated_at      = NOW()
		WHERE id = $1 AND (user_id = $9 OR user_id IS NULL)
	`, id, body.Name, cronExpression, body.Enabled, targetDirectory, updateDefaultTags, defaultTagsRaw, nextRun, userID); err != nil {
		respondError(c, http.StatusInternalServerError, "update schedule: "+err.Error())
		return
	}

	var sched models.DownloadSchedule
	if err := h.db.Get(&sched, `
		SELECT * FROM download_schedules
		WHERE id = $1 AND (user_id = $2 OR user_id IS NULL)
	`, id, userID); err != nil {
		respondError(c, http.StatusNotFound, "schedule not found")
		return
	}

	respondOK(c, sched, nil)
}

// Delete removes a schedule.
// DELETE /api/schedules/:id
func (h *ScheduleHandler) Delete(c *gin.Context) {
	id := c.Param("id")
	userID := c.GetString("userID")

	if _, err := h.db.Exec(`DELETE FROM download_schedules WHERE id = $1 AND (user_id = $2 OR user_id IS NULL)`, id, userID); err != nil {
		respondError(c, http.StatusInternalServerError, "delete schedule: "+err.Error())
		return
	}

	c.Status(http.StatusNoContent)
}
