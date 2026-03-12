package api

import (
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"errors"
	"mime"
	"net/http"
	"path/filepath"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/nacl-dev/tanuki/internal/config"
	"github.com/nacl-dev/tanuki/internal/database"
	"github.com/nacl-dev/tanuki/internal/models"
)

// MobileHandler exposes compact manifests and sync helpers for mobile clients.
type MobileHandler struct {
	db        *database.DB
	mediaPath string
	thumbPath string
	cfg       *config.Config
}

type mobileMediaSummary struct {
	ID           string           `json:"id"`
	Title        string           `json:"title"`
	Type         models.MediaType `json:"type"`
	Rating       int              `json:"rating"`
	Favorite     bool             `json:"favorite"`
	ReadProgress int              `json:"read_progress"`
	ReadTotal    int              `json:"read_total"`
	ViewCount    int              `json:"view_count"`
	Language     string           `json:"language"`
	FileSize     int64            `json:"file_size"`
	SourceURL    string           `json:"source_url"`
	CreatedAt    string           `json:"created_at"`
	UpdatedAt    string           `json:"updated_at"`
	ThumbnailURL string           `json:"thumbnail_url"`
	Tags         []mobileTag      `json:"tags,omitempty"`
}

type mobileTag struct {
	Name string `json:"name"`
}

type mobilePageManifest struct {
	Index    int    `json:"index"`
	Filename string `json:"filename"`
	URL      string `json:"url"`
}

type mobileContentManifest struct {
	Media       mobileMediaSummary   `json:"media"`
	ContentKind string               `json:"content_kind"`
	MimeType    string               `json:"mime_type,omitempty"`
	FileURL     string               `json:"file_url,omitempty"`
	DownloadURL string               `json:"download_url,omitempty"`
	TotalPages  int                  `json:"total_pages,omitempty"`
	Pages       []mobilePageManifest `json:"pages,omitempty"`
}

type mobileProgressSyncRequest struct {
	Updates []mobileProgressUpdate `json:"updates" binding:"required"`
}

type mobileProgressUpdate struct {
	MediaID      string `json:"media_id" binding:"required"`
	ReadProgress int    `json:"read_progress"`
	ReadTotal    int    `json:"read_total"`
}

type mobileProgressSyncItem struct {
	MediaID      string `json:"media_id"`
	ReadProgress int    `json:"read_progress"`
	ReadTotal    int    `json:"read_total"`
	Status       string `json:"status"`
	Reason       string `json:"reason,omitempty"`
}

type mobileBootstrapCapabilities struct {
	MobileContent bool `json:"mobile_content"`
	ProgressSync  bool `json:"progress_sync"`
	VideoPlayback bool `json:"video_playback"`
	OfflinePages  bool `json:"offline_pages"`
}

type mobileBootstrapResponse struct {
	InstanceID           string                      `json:"instance_id"`
	Version              string                      `json:"version"`
	RegistrationEnabled  bool                        `json:"registration_enabled"`
	AuthRefreshSupported bool                        `json:"auth_refresh_supported"`
	Capabilities         mobileBootstrapCapabilities `json:"capabilities"`
}

// Bootstrap returns a compact, public mobile bootstrap payload.
// GET /api/mobile/bootstrap
func (h *MobileHandler) Bootstrap(c *gin.Context) {
	respondOK(c, mobileBootstrapResponse{
		InstanceID:           stableInstanceID(h.cfg),
		Version:              appVersion,
		RegistrationEnabled:  h.cfg != nil && h.cfg.RegistrationEnabled,
		AuthRefreshSupported: true,
		Capabilities: mobileBootstrapCapabilities{
			MobileContent: true,
			ProgressSync:  true,
			VideoPlayback: true,
			OfflinePages:  true,
		},
	}, nil)
}

// GetContent returns a compact content manifest for a single media item.
// GET /api/mobile/media/:id/content
func (h *MobileHandler) GetContent(c *gin.Context) {
	item, err := h.loadMedia(c.Param("id"), c.GetString("userID"))
	if err != nil {
		if err == sql.ErrNoRows {
			respondError(c, http.StatusNotFound, "media not found")
			return
		}
		respondError(c, http.StatusInternalServerError, "load media: "+err.Error())
		return
	}

	manifest := mobileContentManifest{
		Media:       h.toMediaSummary(c, item),
		ContentKind: classifyMobileContentKind(item, h.mediaPath),
		MimeType:    mediaMimeType(item),
		DownloadURL: absoluteAPIURL(c, "/api/media/"+item.ID+"/file"),
	}

	switch manifest.ContentKind {
	case "pages":
		pageNames, err := h.listPageNames(item)
		if err != nil {
			respondError(c, http.StatusInternalServerError, "list pages: "+err.Error())
			return
		}

		manifest.TotalPages = len(pageNames)
		manifest.Pages = make([]mobilePageManifest, len(pageNames))
		for idx, name := range pageNames {
			manifest.Pages[idx] = mobilePageManifest{
				Index:    idx,
				Filename: filepath.Base(name),
				URL:      absoluteAPIURL(c, "/api/media/"+item.ID+"/pages/"+itoa(idx)),
			}
		}
	default:
		manifest.FileURL = absoluteAPIURL(c, "/api/media/"+item.ID+"/file")
	}

	respondOK(c, manifest, nil)
}

// SyncProgress applies a batch of media progress updates from mobile clients.
// POST /api/mobile/progress/sync
func (h *MobileHandler) SyncProgress(c *gin.Context) {
	var req mobileProgressSyncRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		respondError(c, http.StatusBadRequest, err.Error())
		return
	}

	results := make([]mobileProgressSyncItem, 0, len(req.Updates))
	applied := 0

	for _, update := range req.Updates {
		item := normalizeProgressUpdate(update)
		switch {
		case item.MediaID == "":
			results = append(results, mobileProgressSyncItem{
				Status: "skipped",
				Reason: "media_id is required",
			})
			continue
		case item.ReadProgress < 0 || item.ReadTotal < 0:
			results = append(results, mobileProgressSyncItem{
				MediaID:      item.MediaID,
				ReadProgress: item.ReadProgress,
				ReadTotal:    item.ReadTotal,
				Status:       "skipped",
				Reason:       "read_progress and read_total must be >= 0",
			})
			continue
		}

		total := item.ReadTotal
		if total > 0 && item.ReadProgress > total {
			total = item.ReadProgress
		}

		result, err := h.db.Exec(`
			UPDATE media
			SET read_progress = $2,
				read_total = $3,
				updated_at = NOW()
			WHERE id = $1 AND deleted_at IS NULL
		`, item.MediaID, item.ReadProgress, total)
		if err != nil {
			respondError(c, http.StatusInternalServerError, "sync progress: "+err.Error())
			return
		}

		rowsAffected, err := result.RowsAffected()
		if err != nil {
			respondError(c, http.StatusInternalServerError, "sync progress: "+err.Error())
			return
		}

		if rowsAffected == 0 {
			results = append(results, mobileProgressSyncItem{
				MediaID:      item.MediaID,
				ReadProgress: item.ReadProgress,
				ReadTotal:    total,
				Status:       "not_found",
			})
			continue
		}

		applied++
		results = append(results, mobileProgressSyncItem{
			MediaID:      item.MediaID,
			ReadProgress: item.ReadProgress,
			ReadTotal:    total,
			Status:       "applied",
		})
	}

	respondOK(c, gin.H{
		"received": len(req.Updates),
		"applied":  applied,
		"skipped":  len(req.Updates) - applied,
		"items":    results,
	}, nil)
}

func (h *MobileHandler) loadMedia(mediaID, requesterUserID string) (models.Media, error) {
	mediaHandler := &MediaHandler{
		db:        h.db,
		mediaPath: h.mediaPath,
		thumbPath: h.thumbPath,
	}
	return mediaHandler.findMediaByID(mediaID, requesterUserID, false)
}

func (h *MobileHandler) toMediaSummary(c *gin.Context, item models.Media) mobileMediaSummary {
	tags := make([]mobileTag, 0, len(item.Tags))
	for _, tag := range item.Tags {
		name := strings.TrimSpace(tag.Name)
		if name == "" {
			continue
		}
		tags = append(tags, mobileTag{Name: name})
	}

	return mobileMediaSummary{
		ID:           item.ID,
		Title:        item.Title,
		Type:         item.Type,
		Rating:       item.Rating,
		Favorite:     item.Favorite,
		ReadProgress: item.ReadProgress,
		ReadTotal:    item.ReadTotal,
		ViewCount:    item.ViewCount,
		Language:     item.Language,
		FileSize:     item.FileSize,
		SourceURL:    item.SourceURL,
		CreatedAt:    item.CreatedAt.Format(time.RFC3339),
		UpdatedAt:    item.UpdatedAt.Format(time.RFC3339),
		ThumbnailURL: absoluteAPIURL(c, "/api/media/"+item.ID+"/thumbnail"),
		Tags:         tags,
	}
}

func (h *MobileHandler) listPageNames(item models.Media) ([]string, error) {
	names, err := listPageNamesForMediaPath(item.FilePath, h.mediaPath)
	if errors.Is(err, errUnsupportedPagedMedia) {
		return nil, nil
	}
	return names, err
}

func normalizeProgressUpdate(update mobileProgressUpdate) mobileProgressUpdate {
	update.MediaID = strings.TrimSpace(update.MediaID)
	return update
}

func classifyMobileContentKind(item models.Media, mediaRoot string) string {
	if isPagedMedia(item, mediaRoot) {
		return "pages"
	}

	switch item.Type {
	case models.MediaTypeVideo:
		return "video"
	default:
		return "image"
	}
}

func mediaMimeType(item models.Media) string {
	ext := strings.ToLower(filepath.Ext(item.FilePath))
	switch ext {
	case ".cbz":
		return "application/vnd.comicbook+zip"
	case ".cbr":
		return "application/vnd.comicbook-rar"
	}

	if value := mime.TypeByExtension(ext); value != "" {
		return value
	}

	switch item.Type {
	case models.MediaTypeVideo:
		return "video/*"
	default:
		return "image/*"
	}
}

func absoluteAPIURL(c *gin.Context, path string) string {
	scheme := strings.TrimSpace(c.GetHeader("X-Forwarded-Proto"))
	if scheme == "" {
		if c.Request.TLS != nil {
			scheme = "https"
		} else {
			scheme = "http"
		}
	}

	host := strings.TrimSpace(c.GetHeader("X-Forwarded-Host"))
	if host == "" {
		host = c.Request.Host
	}

	base := scheme + "://" + host
	return strings.TrimRight(base, "/") + "/" + strings.TrimLeft(path, "/")
}

func stableInstanceID(cfg *config.Config) string {
	if cfg == nil {
		return "tanuki-unknown"
	}

	seed := strings.TrimSpace(cfg.SecretKey)
	if seed == "" {
		seed = strings.TrimSpace(cfg.JWTSecret)
	}
	if seed == "" {
		return "tanuki-unknown"
	}

	sum := sha256.Sum256([]byte(seed))
	return "tanuki-" + hex.EncodeToString(sum[:8])
}
