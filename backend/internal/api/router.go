// Package api wires together all HTTP route handlers.
package api

import (
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/nacl-dev/tanuki/internal/auth"
	"github.com/nacl-dev/tanuki/internal/config"
	"github.com/nacl-dev/tanuki/internal/database"
	"github.com/nacl-dev/tanuki/internal/plugins"
	"github.com/nacl-dev/tanuki/internal/taskqueue"
	"go.uber.org/zap"
)

// Router creates and returns a configured Gin engine with all API routes
// and a static file server for the compiled frontend.
func Router(db *database.DB, staticDir string, cfg *config.Config, pluginRegistry *plugins.Registry, log *zap.Logger) *gin.Engine {
	r := gin.New()
	r.Use(gin.Recovery())
	r.Use(func(c *gin.Context) {
		requestID := c.GetHeader("X-Request-ID")
		if requestID == "" {
			requestID = uuid.NewString()
		}
		c.Set("requestID", requestID)
		c.Header("X-Request-ID", requestID)
		c.Next()
	})
	r.Use(func(c *gin.Context) {
		start := time.Now()
		c.Next()

		log.Info("http request",
			zap.String("request_id", c.GetString("requestID")),
			zap.String("method", c.Request.Method),
			zap.String("path", c.FullPath()),
			zap.String("raw_path", c.Request.URL.Path),
			zap.Int("status", c.Writer.Status()),
			zap.Duration("duration", time.Since(start)),
			zap.String("client_ip", c.ClientIP()),
			zap.String("user_id", c.GetString("userID")),
		)
	})

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
	taskManager := taskqueue.New(log)
	{
		authH := &AuthHandler{db: db, cfg: cfg}
		authRoutes := apiGroup.Group("/auth")
		{
			authRoutes.POST("/register", authH.Register)
			authRoutes.POST("/login", authH.Login)
			authRoutes.POST("/logout", authH.Logout)
			authRoutes.GET("/me", auth.AuthRequired(cfg.JWTSecret), authH.Me)
			authRoutes.PATCH("/me", auth.AuthRequired(cfg.JWTSecret), authH.UpdateMe)
		}

		// ─── Protected API routes ─────────────────────────────────────────────
		protected := apiGroup.Group("", auth.AuthRequired(cfg.JWTSecret))
		{
			// Media
			mh := &MediaHandler{db: db, mediaPath: cfg.MediaPath, thumbPath: cfg.ThumbnailsPath}
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
				ah := newAutoTagHandler(db, cfg, log, taskManager)
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
			dldh := &DownloadHandler{db: db, downloadsDir: cfg.DownloadsPath, mediaPath: cfg.MediaPath}
			downloads := protected.Group("/downloads")
			{
				downloads.GET("", dldh.List)
				downloads.GET("/stream", dldh.Stream)
				downloads.POST("", dldh.Create)
				downloads.POST("/batch", dldh.Batch)
				downloads.GET("/:id", dldh.Get)
				downloads.PATCH("/:id", dldh.Update)
				downloads.DELETE("/:id", dldh.Delete)
			}

			// Schedules
			sh := &ScheduleHandler{db: db, downloadsDir: cfg.DownloadsPath, mediaPath: cfg.MediaPath}
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
				tasks:     taskManager,
			}
			protected.POST("/library/scan", lh.Scan)
			protected.POST("/library/organize", lh.Organize)
			protected.POST("/library/inbox/upload", lh.UploadInbox)

			taskH := newTaskHandler(taskManager)
			protected.GET("/tasks", taskH.List)
			protected.GET("/tasks/stream", taskH.Stream)
			protected.GET("/tasks/:id", taskH.Get)

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
				pluginsGroup := protected.Group("/plugins", auth.AdminRequired())
				{
					pluginsGroup.GET("", ph.List)
					pluginsGroup.POST("/scan", ph.Scan)
					pluginsGroup.PATCH("/:id", ph.Update)
					pluginsGroup.DELETE("/:id", ph.Delete)
				}
			}

			// System info (v1.0)
			protected.GET("/system/info", func(c *gin.Context) {
				userID := c.GetString("userID")
				isAdmin := c.GetString("role") == "admin"
				var mediaCount int
				_ = db.Get(&mediaCount, `SELECT COUNT(*) FROM media WHERE deleted_at IS NULL`)
				var pluginCount int
				_ = db.Get(&pluginCount, `SELECT COUNT(*) FROM plugins`)

				downloadCountQuery := `SELECT COUNT(*) FROM download_jobs`
				downloadActiveQuery := `SELECT COUNT(*) FROM download_jobs WHERE status IN ('queued', 'downloading', 'processing', 'paused')`
				downloadFailedQuery := `SELECT COUNT(*) FROM download_jobs WHERE status = 'failed'`
				lastCompletedDownloadQuery := `SELECT completed_at FROM download_jobs WHERE completed_at IS NOT NULL ORDER BY completed_at DESC LIMIT 1`
				scheduleTotalQuery := `SELECT COUNT(*) FROM download_schedules`
				scheduleEnabledQuery := `SELECT COUNT(*) FROM download_schedules WHERE enabled = TRUE`
				queryArgs := []interface{}{}
				taskSummary := taskManager.Summary()

				backgroundTasksActive := taskSummary.Active
				backgroundTasksFailed := taskSummary.Failed
				if !isAdmin {
					queryArgs = append(queryArgs, userID)
					downloadCountQuery += ` WHERE user_id = $1`
					downloadActiveQuery += ` AND user_id = $1`
					downloadFailedQuery += ` AND user_id = $1`
					lastCompletedDownloadQuery = `SELECT completed_at FROM download_jobs WHERE user_id = $1 AND completed_at IS NOT NULL ORDER BY completed_at DESC LIMIT 1`
					scheduleTotalQuery += ` WHERE user_id = $1`
					scheduleEnabledQuery += ` WHERE user_id = $1 AND enabled = TRUE`

					backgroundTasksActive = 0
					backgroundTasksFailed = 0
					for _, task := range taskManager.ListForRequester(userID, 0) {
						switch task.Status {
						case taskqueue.StatusQueued, taskqueue.StatusRunning:
							backgroundTasksActive++
						case taskqueue.StatusFailed:
							backgroundTasksFailed++
						}
					}
				}

				var downloadsTotal int
				_ = db.Get(&downloadsTotal, downloadCountQuery, queryArgs...)
				var downloadsActive int
				_ = db.Get(&downloadsActive, downloadActiveQuery, queryArgs...)
				var downloadsFailed int
				_ = db.Get(&downloadsFailed, downloadFailedQuery, queryArgs...)
				var schedulesTotal int
				_ = db.Get(&schedulesTotal, scheduleTotalQuery, queryArgs...)
				var schedulesEnabled int
				_ = db.Get(&schedulesEnabled, scheduleEnabledQuery, queryArgs...)
				var autoTagPending int
				_ = db.Get(&autoTagPending, `SELECT COUNT(*) FROM media WHERE deleted_at IS NULL AND auto_tag_status IN ('pending', 'processing')`)
				var lastCompletedDownload *time.Time
				_ = db.Get(&lastCompletedDownload, lastCompletedDownloadQuery, queryArgs...)
				pathHealth := gin.H{
					"media":      pathStatus(cfg.MediaPath),
					"downloads":  pathStatus(cfg.DownloadsPath),
					"thumbnails": pathStatus(cfg.ThumbnailsPath),
					"inbox":      pathStatus(cfg.InboxPath),
				}
				payload := gin.H{
					"version":                 "1.0.0",
					"media_count":             mediaCount,
					"plugin_count":            pluginCount,
					"downloads_total":         downloadsTotal,
					"downloads_active":        downloadsActive,
					"downloads_failed":        downloadsFailed,
					"schedules_total":         schedulesTotal,
					"schedules_enabled":       schedulesEnabled,
					"autotag_pending":         autoTagPending,
					"background_tasks_active": backgroundTasksActive,
					"background_tasks_failed": backgroundTasksFailed,
					"last_completed_download": lastCompletedDownload,
					"plugins_enabled":         cfg.PluginsEnabled,
					"registration_enabled":    cfg.RegistrationEnabled,
					"runtime_details_visible": isAdmin,
					"library_scope":           "shared",
					"tag_scope":               "shared",
					"collection_scope":        "personal",
					"download_scope":          "personal",
					"schedule_scope":          "personal",
					"owner_mode":              "shared_library_owner_id_unused",
				}
				if isAdmin {
					payload["media_path"] = cfg.MediaPath
					payload["downloads_path"] = cfg.DownloadsPath
					payload["thumbnails_path"] = cfg.ThumbnailsPath
					payload["inbox_path"] = cfg.InboxPath
					payload["path_health"] = pathHealth
					payload["scan_interval"] = cfg.ScanInterval
					payload["max_concurrent_downloads"] = cfg.MaxConcurrentDownloads
					payload["rate_limit_delay"] = cfg.RateLimitDelay
				} else {
					payload["path_health"] = gin.H{}
				}
				respondOK(c, payload, nil)
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
