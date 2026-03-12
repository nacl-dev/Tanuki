package api

import (
	"context"
	"database/sql"
	"encoding/json"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/nacl-dev/tanuki/internal/database"
	"github.com/nacl-dev/tanuki/internal/downloader"
	"github.com/nacl-dev/tanuki/internal/models"
	"github.com/nacl-dev/tanuki/internal/tagrules"
)

// DownloadHandler manages download job CRUD.
type DownloadHandler struct {
	db           *database.DB
	downloadsDir string
	mediaPath    string
}

// List returns all download jobs, optionally filtered by status.
// GET /api/downloads?status=queued
func (h *DownloadHandler) List(c *gin.Context) {
	status := c.Query("status")
	userID := c.GetString("userID")

	jobs, err := h.listForUser(userID, status)
	if err != nil {
		respondError(c, http.StatusInternalServerError, "query downloads: "+err.Error())
		return
	}

	respondOK(c, jobs, &Meta{Total: len(jobs)})
}

// Stream emits the current download job list whenever it changes.
// GET /api/downloads/stream
func (h *DownloadHandler) Stream(c *gin.Context) {
	status := c.Query("status")
	userID := c.GetString("userID")

	streamJSON(c, time.Second, func() (any, error) {
		return h.listForUser(userID, status)
	})
}

// Create enqueues a new download job.
// POST /api/downloads
func (h *DownloadHandler) Create(c *gin.Context) {
	var body struct {
		URL             string   `json:"url"              binding:"required,url"`
		TargetDirectory string   `json:"target_directory" binding:"-"`
		AutoTags        []string `json:"auto_tags"        binding:"-"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		respondError(c, http.StatusBadRequest, err.Error())
		return
	}
	targetDirectory, err := downloader.NormalizeTargetDirectory(body.TargetDirectory, h.downloadsDir, h.mediaPath)
	if err != nil {
		respondError(c, http.StatusBadRequest, err.Error())
		return
	}
	autoTags, autoTagsRaw, err := prepareAutoTags(c.Request.Context(), h.db, body.AutoTags)
	if err != nil {
		respondError(c, http.StatusInternalServerError, "prepare auto tags: "+err.Error())
		return
	}

	job := models.DownloadJob{
		ID:              uuid.NewString(),
		UserID:          stringPtr(c.GetString("userID")),
		URL:             body.URL,
		Status:          models.DownloadStatusQueued,
		TargetDirectory: targetDirectory,
		AutoTags:        autoTagsRaw,
	}

	if _, err := h.db.Exec(`
		INSERT INTO download_jobs
			(id, user_id, url, source_type, status, progress, target_directory, auto_tags, retry_count)
		VALUES ($1, $2, $3, 'auto', $4, 0, $5, $6, 0)
	`, job.ID, job.UserID, job.URL, job.Status, job.TargetDirectory, autoTagsRaw); err != nil {
		respondError(c, http.StatusInternalServerError, "create job: "+err.Error())
		return
	}
	if len(autoTags) > 0 {
		job.AutoTags = autoTagsRaw
	}

	// Notify downloader via Redis key (fire and forget; downloader polls).
	c.JSON(http.StatusCreated, envelope{Data: job})
}

// Batch enqueues multiple download jobs at once.
// POST /api/downloads/batch
func (h *DownloadHandler) Batch(c *gin.Context) {
	userID := stringPtr(c.GetString("userID"))

	var body struct {
		URLs            []string `json:"urls"             binding:"required"`
		TargetDirectory string   `json:"target_directory" binding:"-"`
		AutoTags        []string `json:"auto_tags"        binding:"-"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		respondError(c, http.StatusBadRequest, err.Error())
		return
	}
	targetDirectory, err := downloader.NormalizeTargetDirectory(body.TargetDirectory, h.downloadsDir, h.mediaPath)
	if err != nil {
		respondError(c, http.StatusBadRequest, err.Error())
		return
	}
	_, autoTagsRaw, err := prepareAutoTags(c.Request.Context(), h.db, body.AutoTags)
	if err != nil {
		respondError(c, http.StatusInternalServerError, "prepare auto tags: "+err.Error())
		return
	}

	created := make([]string, 0, len(body.URLs))
	for _, rawURL := range body.URLs {
		id := uuid.NewString()
		if _, err := h.db.Exec(`
			INSERT INTO download_jobs
				(id, user_id, url, source_type, status, progress, target_directory, auto_tags, retry_count)
			VALUES ($1, $2, $3, 'auto', 'queued', 0, $4, $5, 0)
		`, id, userID, rawURL, targetDirectory, autoTagsRaw); err == nil {
			created = append(created, id)
		}
	}

	respondOK(c, gin.H{"created": created}, nil)
}

func prepareAutoTags(ctx context.Context, db *database.DB, rawTags []string) ([]string, *json.RawMessage, error) {
	normalized := downloader.NormalizeDownloadAutoTags(rawTags)
	if len(normalized) == 0 {
		return nil, nil, nil
	}
	if db == nil {
		payload, err := downloader.EncodeDownloadAutoTags(normalized)
		if err != nil {
			return nil, nil, err
		}
		return normalized, payload, nil
	}

	svc := tagrules.NewService(db)
	canonical := make([]string, 0, len(normalized))
	for _, raw := range normalized {
		expression, err := svc.CanonicalizeExpression(ctx, raw)
		if err != nil && err != sql.ErrNoRows {
			return nil, nil, err
		}
		if expression == "" {
			expression = raw
		}
		canonical = append(canonical, expression)
	}
	canonical = downloader.NormalizeDownloadAutoTags(canonical)

	payload, err := downloader.EncodeDownloadAutoTags(canonical)
	if err != nil {
		return nil, nil, err
	}
	return canonical, payload, nil
}

// Get returns a single download job.
// GET /api/downloads/:id
func (h *DownloadHandler) Get(c *gin.Context) {
	id := c.Param("id")
	userID := c.GetString("userID")

	var job models.DownloadJob
	if err := h.db.Get(&job, `
		SELECT * FROM download_jobs
		WHERE id = $1 AND (user_id = $2 OR user_id IS NULL)
	`, id, userID); err != nil {
		respondError(c, http.StatusNotFound, "download job not found")
		return
	}

	respondOK(c, job, nil)
}

// Update applies a control action (pause, resume, cancel, retry) or changes
// mutable fields on a download job.
// PATCH /api/downloads/:id
func (h *DownloadHandler) Update(c *gin.Context) {
	id := c.Param("id")
	userID := c.GetString("userID")

	var body struct {
		Action          string  `json:"action"           binding:"-"` // pause|resume|cancel|retry
		TargetDirectory *string `json:"target_directory" binding:"-"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		respondError(c, http.StatusBadRequest, err.Error())
		return
	}

	var job models.DownloadJob
	if err := h.db.Get(&job, `
		SELECT * FROM download_jobs
		WHERE id = $1 AND (user_id = $2 OR user_id IS NULL)
	`, id, userID); err != nil {
		respondError(c, http.StatusNotFound, "download job not found")
		return
	}

	switch body.Action {
	case "pause":
		if job.Status != models.DownloadStatusQueued && job.Status != models.DownloadStatusDownloading {
			respondError(c, http.StatusConflict, "download can only be paused while queued or downloading")
			return
		}
		h.db.Exec(`UPDATE download_jobs SET status = 'paused', updated_at = NOW() WHERE id = $1 AND (user_id = $2 OR user_id IS NULL)`, id, userID) //nolint:errcheck
	case "resume":
		if job.Status != models.DownloadStatusPaused {
			respondError(c, http.StatusConflict, "download can only be resumed from paused state")
			return
		}
		h.db.Exec(`
			UPDATE download_jobs
			SET status = 'queued',
				progress = 0,
				total_files = 0,
				downloaded_files = 0,
				total_bytes = 0,
				downloaded_bytes = 0,
				error_message = '',
				completed_at = NULL,
				updated_at = NOW()
			WHERE id = $1 AND (user_id = $2 OR user_id IS NULL)
		`, id, userID) //nolint:errcheck
	case "cancel":
		if job.Status == models.DownloadStatusCompleted {
			respondError(c, http.StatusConflict, "completed downloads cannot be cancelled")
			return
		}
		h.db.Exec(`UPDATE download_jobs SET status = 'failed', updated_at = NOW(), error_message = 'cancelled by user' WHERE id = $1 AND (user_id = $2 OR user_id IS NULL)`, id, userID) //nolint:errcheck
	case "retry":
		if job.Status != models.DownloadStatusFailed {
			respondError(c, http.StatusConflict, "download can only be retried after it failed")
			return
		}
		h.db.Exec(`
			UPDATE download_jobs
			SET status = 'queued',
				progress = 0,
				total_files = 0,
				downloaded_files = 0,
				total_bytes = 0,
				downloaded_bytes = 0,
				error_message = '',
				completed_at = NULL,
				updated_at = NOW()
			WHERE id = $1 AND (user_id = $2 OR user_id IS NULL)
		`, id, userID) //nolint:errcheck
	case "":
		// no-op, target directory update handled below
	default:
		respondError(c, http.StatusBadRequest, "unknown download action")
		return
	}
	if body.TargetDirectory != nil {
		targetDirectory, err := downloader.NormalizeTargetDirectory(*body.TargetDirectory, h.downloadsDir, h.mediaPath)
		if err != nil {
			respondError(c, http.StatusBadRequest, err.Error())
			return
		}
		h.db.Exec(`UPDATE download_jobs SET target_directory = $2, updated_at = NOW() WHERE id = $1 AND (user_id = $3 OR user_id IS NULL)`, id, targetDirectory, userID) //nolint:errcheck
	}

	h.Get(c)
}

// Delete removes a download job.
// DELETE /api/downloads/:id
func (h *DownloadHandler) Delete(c *gin.Context) {
	id := c.Param("id")
	userID := c.GetString("userID")

	if _, err := h.db.Exec(`DELETE FROM download_jobs WHERE id = $1 AND (user_id = $2 OR user_id IS NULL)`, id, userID); err != nil {
		respondError(c, http.StatusInternalServerError, "delete job: "+err.Error())
		return
	}

	c.Status(http.StatusNoContent)
}

func stringPtr(v string) *string {
	if v == "" {
		return nil
	}
	return &v
}

func (h *DownloadHandler) listForUser(userID, status string) ([]models.DownloadJob, error) {
	var jobs []models.DownloadJob
	var err error

	if status != "" {
		err = h.db.Select(&jobs, `
			SELECT * FROM download_jobs
			WHERE status = $1 AND (user_id = $2 OR user_id IS NULL)
			ORDER BY created_at DESC
		`, status, userID)
	} else {
		err = h.db.Select(&jobs, `
			SELECT * FROM download_jobs
			WHERE user_id = $1 OR user_id IS NULL
			ORDER BY created_at DESC
		`, userID)
	}

	return jobs, err
}
