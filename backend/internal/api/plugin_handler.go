package api

import (
	"net/http"
	"os"

	"github.com/gin-gonic/gin"
	"github.com/nacl-dev/tanuki/internal/plugins"
)

// PluginHandler implements REST endpoints for plugin management.
type PluginHandler struct {
	registry *plugins.Registry
}

func newPluginHandler(registry *plugins.Registry) *PluginHandler {
	return &PluginHandler{registry: registry}
}

// List returns all installed plugins.
func (h *PluginHandler) List(c *gin.Context) {
	items, err := h.registry.ListPlugins(c.Request.Context())
	if err != nil {
		respondError(c, http.StatusInternalServerError, err.Error())
		return
	}
	respondOK(c, items, nil)
}

// Scan triggers a re-scan of the plugins directory.
func (h *PluginHandler) Scan(c *gin.Context) {
	if err := h.registry.LoadAll(); err != nil {
		respondError(c, http.StatusInternalServerError, err.Error())
		return
	}
	items, err := h.registry.ListPlugins(c.Request.Context())
	if err != nil {
		respondError(c, http.StatusInternalServerError, err.Error())
		return
	}
	respondOK(c, items, nil)
}

// Update enables or disables a plugin.
func (h *PluginHandler) Update(c *gin.Context) {
	id := c.Param("id")

	var body struct {
		Enabled *bool `json:"enabled"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		respondError(c, http.StatusBadRequest, "invalid request body")
		return
	}
	if body.Enabled == nil {
		respondError(c, http.StatusBadRequest, "field 'enabled' is required")
		return
	}

	if err := h.registry.TogglePlugin(c.Request.Context(), id, *body.Enabled); err != nil {
		respondError(c, http.StatusInternalServerError, err.Error())
		return
	}

	respondOK(c, gin.H{"id": id, "enabled": *body.Enabled}, nil)
}

// Delete removes a plugin (DB record + file).
func (h *PluginHandler) Delete(c *gin.Context) {
	id := c.Param("id")

	// Look up the plugin to get its file path.
	items, err := h.registry.ListPlugins(c.Request.Context())
	if err != nil {
		respondError(c, http.StatusInternalServerError, err.Error())
		return
	}

	var filePath string
	for _, p := range items {
		if p.ID == id {
			filePath = p.FilePath
			break
		}
	}

	if err := h.registry.DeletePlugin(c.Request.Context(), id); err != nil {
		respondError(c, http.StatusInternalServerError, err.Error())
		return
	}

	// Best-effort delete the file.
	if filePath != "" {
		_ = os.Remove(filePath)
	}

	respondOK(c, gin.H{"deleted": id}, nil)
}
