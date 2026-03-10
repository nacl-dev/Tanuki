package api

import (
	"context"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/nacl-dev/tanuki/internal/database"
	"github.com/nacl-dev/tanuki/internal/scanner"
	"go.uber.org/zap"
)

// LibraryHandler exposes library management actions.
type LibraryHandler struct {
	db        *database.DB
	mediaPath string
	log       *zap.Logger
}

// Scan triggers an immediate filesystem scan.
// POST /api/library/scan
func (h *LibraryHandler) Scan(c *gin.Context) {
	sc := scanner.New(h.db, h.mediaPath, h.log)
	if err := sc.Run(context.Background()); err != nil {
		respondError(c, http.StatusInternalServerError, "scan library: "+err.Error())
		return
	}
	c.JSON(http.StatusAccepted, envelope{Data: gin.H{"message": "scan completed"}})
}
