package api

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/nacl-dev/tanuki/internal/database"
)

// LibraryHandler exposes library management actions.
type LibraryHandler struct {
	db *database.DB
}

// Scan triggers an immediate filesystem scan.
// POST /api/library/scan
func (h *LibraryHandler) Scan(c *gin.Context) {
	// Push a scan job onto Redis so the worker picks it up.
	// For now we just acknowledge the request; the worker watches a key.
	c.JSON(http.StatusAccepted, envelope{Data: gin.H{"message": "scan queued"}})
}
