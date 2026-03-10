// Package api wires together all HTTP route handlers.
package api

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/nacl-dev/tanuki/internal/config"
	"github.com/nacl-dev/tanuki/internal/database"
	"go.uber.org/zap"
)

// Router creates and returns a configured Gin engine with all API routes
// and a static file server for the compiled frontend.
func Router(db *database.DB, staticDir string, cfg *config.Config, log *zap.Logger) *gin.Engine {
	r := gin.New()
	r.Use(gin.Recovery())
	r.Use(gin.Logger())

	// ─── CORS (development convenience) ──────────────────────────────────────
	r.Use(func(c *gin.Context) {
		c.Header("Access-Control-Allow-Origin", "*")
		c.Header("Access-Control-Allow-Methods", "GET, POST, PATCH, DELETE, OPTIONS")
		c.Header("Access-Control-Allow-Headers", "Content-Type, Authorization")
		if c.Request.Method == http.MethodOptions {
			c.AbortWithStatus(http.StatusNoContent)
			return
		}
		c.Next()
	})

	// ─── Health check ─────────────────────────────────────────────────────────
	r.GET("/healthz", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	})
	r.GET("/api/health", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok", "version": "0.5.0"})
	})

	// ─── API routes ───────────────────────────────────────────────────────────
	api := r.Group("/api")
	{
		// Media
		mh := &MediaHandler{db: db}
		media := api.Group("/media")
		{
			media.GET("", mh.List)
			media.GET("/:id", mh.Get)
			media.PATCH("/:id", mh.Update)
			media.DELETE("/:id", mh.Delete)
			media.GET("/:id/file", mh.ServeFile)
			media.GET("/:id/thumbnail", mh.ServeThumbnail)
			media.GET("/:id/pages", mh.ListPages)
			media.GET("/:id/pages/:page", mh.ServePage)

			// Auto-tagging (v0.4)
			ah := newAutoTagHandler(db, cfg, log)
			media.POST("/:id/autotag", ah.AutoTag)
			media.POST("/autotag/batch", ah.AutoTagBatch)

			// Duplicate detection (v0.5)
			dh := newDedupHandler(db, cfg.DuplicateThreshold, log)
			media.GET("/:id/duplicates", dh.GetDuplicates)
			media.POST("/:id/phash", dh.ComputePHash)
		}

		// Tags
		th := &TagHandler{db: db}
		tags := api.Group("/tags")
		{
			tags.GET("", th.List)
			tags.GET("/search", th.Search)
			tags.POST("", th.Create)
			tags.PATCH("/:id", th.Update)
			tags.DELETE("/:id", th.Delete)
		}

		// Downloads
		dldh := &DownloadHandler{db: db}
		downloads := api.Group("/downloads")
		{
			downloads.GET("", dldh.List)
			downloads.POST("", dldh.Create)
			downloads.POST("/batch", dldh.Batch)
			downloads.GET("/:id", dldh.Get)
			downloads.PATCH("/:id", dldh.Update)
			downloads.DELETE("/:id", dldh.Delete)
		}

		// Schedules
		sh := &ScheduleHandler{db: db}
		schedules := api.Group("/schedules")
		{
			schedules.GET("", sh.List)
			schedules.POST("", sh.Create)
			schedules.PATCH("/:id", sh.Update)
			schedules.DELETE("/:id", sh.Delete)
		}

		// Library
		lh := &LibraryHandler{db: db}
		api.POST("/library/scan", lh.Scan)

		// Duplicates (v0.5)
		ddh := newDedupHandler(db, cfg.DuplicateThreshold, log)
		duplicates := api.Group("/duplicates")
		{
			duplicates.GET("", ddh.ListDuplicates)
			duplicates.POST("/resolve", ddh.ResolveDuplicates)
		}
	}

	// ─── Static frontend ──────────────────────────────────────────────────────
	if staticDir != "" {
		r.Static("/assets", staticDir+"/assets")
		r.StaticFile("/favicon.ico", staticDir+"/favicon.ico")
		// Catch-all: serve index.html for all non-API routes (SPA routing)
		r.NoRoute(func(c *gin.Context) {
			c.File(staticDir + "/index.html")
		})
	}

	return r
}
