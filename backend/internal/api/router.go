// Package api wires together all HTTP route handlers.
package api

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/nacl-dev/tanuki/internal/auth"
	"github.com/nacl-dev/tanuki/internal/config"
	"github.com/nacl-dev/tanuki/internal/database"
	"github.com/nacl-dev/tanuki/internal/plugins"
	"go.uber.org/zap"
)

// Router creates and returns a configured Gin engine with all API routes
// and a static file server for the compiled frontend.
func Router(db *database.DB, staticDir string, cfg *config.Config, pluginRegistry *plugins.Registry, log *zap.Logger) *gin.Engine {
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
		c.JSON(http.StatusOK, gin.H{"status": "ok", "version": "1.0.0"})
	})

	// ─── Auth routes (public) ─────────────────────────────────────────────────
	apiGroup := r.Group("/api")
	{
		authH := &AuthHandler{db: db, cfg: cfg}
		authRoutes := apiGroup.Group("/auth")
		{
			authRoutes.POST("/register", authH.Register)
			authRoutes.POST("/login", authH.Login)
			authRoutes.GET("/me", auth.AuthRequired(cfg.JWTSecret), authH.Me)
			authRoutes.PATCH("/me", auth.AuthRequired(cfg.JWTSecret), authH.UpdateMe)
		}

		// ─── Protected API routes ─────────────────────────────────────────────
		protected := apiGroup.Group("", auth.AuthRequired(cfg.JWTSecret))
		{
			// Media
			mh := &MediaHandler{db: db, thumbPath: cfg.ThumbnailsPath}
			ch := &CollectionHandler{db: db}
			media := protected.Group("/media")
			{
				media.GET("", mh.List)
				media.GET("/suggestions", mh.Suggestions)
				media.GET("/:id", mh.Get)
				media.PATCH("/:id", mh.Update)
				media.DELETE("/:id", mh.Delete)
				media.GET("/:id/file", mh.ServeFile)
				media.GET("/:id/thumbnail", mh.ServeThumbnail)
				media.POST("/:id/thumbnail/upload", mh.UploadThumbnail)
				media.POST("/:id/thumbnail/fetch", mh.FetchThumbnail)
				media.GET("/:id/pages", mh.ListPages)
				media.GET("/:id/pages/:page", mh.ServePage)
				media.GET("/:id/collections", ch.ListForMedia)

				// Auto-tagging (v0.4)
				ah := newAutoTagHandler(db, cfg, log)
				media.POST("/:id/autotag", ah.AutoTag)
				media.POST("/autotag/batch", ah.AutoTagBatch)

				// Duplicate detection (v0.5)
				dh := newDedupHandler(db, cfg.DuplicateThreshold, log)
				media.GET("/:id/duplicates", dh.GetDuplicates)
				media.POST("/:id/phash", dh.ComputePHash)
			}

			collections := protected.Group("/collections")
			{
				collections.GET("", ch.List)
				collections.POST("", ch.Create)
				collections.GET("/:id", ch.Get)
				collections.PATCH("/:id", ch.Update)
				collections.DELETE("/:id", ch.Delete)
				collections.POST("/:id/media", ch.AddMedia)
				collections.DELETE("/:id/media/:mediaId", ch.RemoveMedia)
			}

			// Tags
			th := &TagHandler{db: db}
			tags := protected.Group("/tags")
			{
				tags.GET("", th.List)
				tags.GET("/search", th.Search)
				tags.POST("", th.Create)
				tags.PATCH("/:id", th.Update)
				tags.DELETE("/:id", th.Delete)
			}

			// Downloads
			dldh := &DownloadHandler{db: db}
			downloads := protected.Group("/downloads")
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
			schedules := protected.Group("/schedules")
			{
				schedules.GET("", sh.List)
				schedules.POST("", sh.Create)
				schedules.PATCH("/:id", sh.Update)
				schedules.DELETE("/:id", sh.Delete)
			}

			// Library
			lh := &LibraryHandler{
				db:        db,
				mediaPath: cfg.MediaPath,
				thumbPath: cfg.ThumbnailsPath,
				inboxPath: cfg.InboxPath,
				log:       log,
			}
			protected.POST("/library/scan", lh.Scan)
			protected.POST("/library/organize", lh.Organize)

			// Duplicates (v0.5)
			ddh := newDedupHandler(db, cfg.DuplicateThreshold, log)
			duplicates := protected.Group("/duplicates")
			{
				duplicates.GET("", ddh.ListDuplicates)
				duplicates.POST("/resolve", ddh.ResolveDuplicates)
			}

			// Plugins (v1.0)
			if pluginRegistry != nil {
				ph := newPluginHandler(pluginRegistry)
				pluginsGroup := protected.Group("/plugins")
				{
					pluginsGroup.GET("", ph.List)
					pluginsGroup.POST("/scan", ph.Scan)
					pluginsGroup.PATCH("/:id", ph.Update)
					pluginsGroup.DELETE("/:id", ph.Delete)
				}
			}

			// System info (v1.0)
			protected.GET("/system/info", func(c *gin.Context) {
				var mediaCount int
				_ = db.Get(&mediaCount, `SELECT COUNT(*) FROM media WHERE deleted_at IS NULL`)
				var pluginCount int
				_ = db.Get(&pluginCount, `SELECT COUNT(*) FROM plugins`)
				c.JSON(http.StatusOK, gin.H{
					"version":      "1.0.0",
					"media_count":  mediaCount,
					"plugin_count": pluginCount,
				})
			})
		}

		// ─── Admin-only routes ────────────────────────────────────────────────
		admin := apiGroup.Group("/admin", auth.AuthRequired(cfg.JWTSecret), auth.AdminRequired())
		{
			authH := &AuthHandler{db: db, cfg: cfg}
			users := admin.Group("/users")
			{
				users.GET("", authH.ListUsers)
				users.PATCH("/:id", authH.UpdateUser)
				users.DELETE("/:id", authH.DeleteUser)
			}
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
