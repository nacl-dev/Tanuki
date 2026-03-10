package api

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/nacl-dev/tanuki/internal/database"
	"github.com/nacl-dev/tanuki/internal/models"
)

// DownloadHandler manages download job CRUD.
type DownloadHandler struct {
	db *database.DB
}

// List returns all download jobs, optionally filtered by status.
// GET /api/downloads?status=queued
func (h *DownloadHandler) List(c *gin.Context) {
	status := c.Query("status")

	var jobs []models.DownloadJob
	var err error

	if status != "" {
		err = h.db.Select(&jobs, `SELECT * FROM download_jobs WHERE status = $1 ORDER BY created_at DESC`, status)
	} else {
		err = h.db.Select(&jobs, `SELECT * FROM download_jobs ORDER BY created_at DESC`)
	}

	if err != nil {
		respondError(c, http.StatusInternalServerError, "query downloads: "+err.Error())
		return
	}

	respondOK(c, jobs, &Meta{Total: len(jobs)})
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

	job := models.DownloadJob{
		ID:              uuid.NewString(),
		URL:             body.URL,
		Status:          models.DownloadStatusQueued,
		TargetDirectory: body.TargetDirectory,
	}

	if _, err := h.db.Exec(`
		INSERT INTO download_jobs
			(id, url, source_type, status, progress, target_directory, retry_count)
		VALUES ($1, $2, 'auto', $3, 0, $4, 0)
	`, job.ID, job.URL, job.Status, job.TargetDirectory); err != nil {
		respondError(c, http.StatusInternalServerError, "create job: "+err.Error())
		return
	}

	// Notify downloader via Redis key (fire and forget; downloader polls).
	c.JSON(http.StatusCreated, envelope{Data: job})
}

// Batch enqueues multiple download jobs at once.
// POST /api/downloads/batch
func (h *DownloadHandler) Batch(c *gin.Context) {
	var body struct {
		URLs            []string `json:"urls"             binding:"required"`
		TargetDirectory string   `json:"target_directory" binding:"-"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		respondError(c, http.StatusBadRequest, err.Error())
		return
	}

	created := make([]string, 0, len(body.URLs))
	for _, rawURL := range body.URLs {
		id := uuid.NewString()
		if _, err := h.db.Exec(`
			INSERT INTO download_jobs
				(id, url, source_type, status, progress, target_directory, retry_count)
			VALUES ($1, $2, 'auto', 'queued', 0, $3, 0)
		`, id, rawURL, body.TargetDirectory); err == nil {
			created = append(created, id)
		}
	}

	respondOK(c, gin.H{"created": created}, nil)
}

// Get returns a single download job.
// GET /api/downloads/:id
func (h *DownloadHandler) Get(c *gin.Context) {
	id := c.Param("id")

	var job models.DownloadJob
	if err := h.db.Get(&job, `SELECT * FROM download_jobs WHERE id = $1`, id); err != nil {
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

	var body struct {
		Action          string `json:"action"           binding:"-"` // pause|resume|cancel|retry
		TargetDirectory string `json:"target_directory" binding:"-"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		respondError(c, http.StatusBadRequest, err.Error())
		return
	}

	switch body.Action {
	case "pause":
		h.db.Exec(`UPDATE download_jobs SET status = 'paused',  updated_at = NOW() WHERE id = $1`, id) //nolint:errcheck
	case "resume":
		h.db.Exec(`UPDATE download_jobs SET status = 'queued',  updated_at = NOW() WHERE id = $1`, id) //nolint:errcheck
	case "cancel":
		h.db.Exec(`UPDATE download_jobs SET status = 'failed',  updated_at = NOW(), error_message = 'cancelled by user' WHERE id = $1`, id) //nolint:errcheck
	case "retry":
		h.db.Exec(`UPDATE download_jobs SET status = 'queued',  updated_at = NOW(), error_message = '', retry_count = 0 WHERE id = $1`, id) //nolint:errcheck
	}

	h.Get(c)
}

// Delete removes a download job.
// DELETE /api/downloads/:id
func (h *DownloadHandler) Delete(c *gin.Context) {
	id := c.Param("id")

	if _, err := h.db.Exec(`DELETE FROM download_jobs WHERE id = $1`, id); err != nil {
		respondError(c, http.StatusInternalServerError, "delete job: "+err.Error())
		return
	}

	c.Status(http.StatusNoContent)
}
