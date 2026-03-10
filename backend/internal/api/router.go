// Package api wires together all HTTP route handlers.
package api

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/nacl-dev/tanuki/internal/database"
)

// Router creates and returns a configured Gin engine with all API routes
// and a static file server for the compiled frontend.
func Router(db *database.DB, staticDir string) *gin.Engine {
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
		c.JSON(http.StatusOK, gin.H{"status": "ok", "version": "0.1.0"})
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
		dh := &DownloadHandler{db: db}
		downloads := api.Group("/downloads")
		{
			downloads.GET("", dh.List)
			downloads.POST("", dh.Create)
			downloads.POST("/batch", dh.Batch)
			downloads.GET("/:id", dh.Get)
			downloads.PATCH("/:id", dh.Update)
			downloads.DELETE("/:id", dh.Delete)
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
