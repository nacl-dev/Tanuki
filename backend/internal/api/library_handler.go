package api

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/nacl-dev/tanuki/internal/database"
	"github.com/nacl-dev/tanuki/internal/models"
	"github.com/nacl-dev/tanuki/internal/scanner"
	"github.com/nacl-dev/tanuki/internal/taskqueue"
	"github.com/nacl-dev/tanuki/internal/thumbnails"
	"go.uber.org/zap"
)

// LibraryHandler exposes library management actions.
type LibraryHandler struct {
	db        *database.DB
	mediaPath string
	thumbPath string
	inboxPath string
	log       *zap.Logger
	tasks     *taskqueue.Manager
}

// Scan triggers an immediate filesystem scan.
// POST /api/library/scan
func (h *LibraryHandler) Scan(c *gin.Context) {
	task := h.tasks.Start("library.scan", c.GetString("userID"), map[string]any{
		"media_path": h.mediaPath,
	}, func(ctx context.Context, handle *taskqueue.Handle) (any, error) {
		handle.SetMessage("Scanning library")
		sc := scanner.New(h.db, h.mediaPath, h.thumbPath, h.log)
		if err := sc.Run(ctx); err != nil {
			return nil, fmt.Errorf("scan library: %w", err)
		}
		handle.SetMessage("Library scan completed")
		handle.SetProgress(1, 1)
		return gin.H{"message": "scan completed"}, nil
	})
	respondAccepted(c, gin.H{"message": "scan queued", "task_id": task.ID}, nil)
}

// Organize moves or copies recognized media files from a staging directory under
// the media root into type-based library folders and triggers a rescan.
// POST /api/library/organize
func (h *LibraryHandler) Organize(c *gin.Context) {
	var body struct {
		SourcePath string `json:"source_path" binding:"required"`
		Mode       string `json:"mode" binding:"-"`
		Preview    bool   `json:"preview" binding:"-"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		respondError(c, http.StatusBadRequest, err.Error())
		return
	}

	mode := strings.ToLower(strings.TrimSpace(body.Mode))
	if mode == "" {
		mode = "move"
	}
	if mode != "move" && mode != "copy" {
		respondError(c, http.StatusBadRequest, "mode must be 'move' or 'copy'")
		return
	}

	sourcePath, err := resolveLibraryPath(h.mediaPath, h.inboxPath, body.SourcePath)
	if err != nil {
		respondError(c, http.StatusBadRequest, err.Error())
		return
	}
	if sourcePath == h.mediaPath || sourcePath == h.inboxPath {
		respondError(c, http.StatusBadRequest, "source_path must point to a subfolder, not the root folder itself")
		return
	}

	if body.Preview {
		stats, err := h.organizeDirectory(sourcePath, mode, true)
		if err != nil {
			respondError(c, http.StatusInternalServerError, "organize library: "+err.Error())
			return
		}
		respondAccepted(c, stats, nil)
		return
	}

	task := h.tasks.Start("library.organize", c.GetString("userID"), map[string]any{
		"source_path": sourcePath,
		"mode":        mode,
	}, func(ctx context.Context, handle *taskqueue.Handle) (any, error) {
		handle.SetMessage("Organizing files")
		handle.SetProgress(0, 3)

		organizedStats, err := h.organizeDirectory(sourcePath, mode, false)
		if err != nil {
			return nil, fmt.Errorf("organize library: %w", err)
		}

		handle.SetProgress(1, 3)
		handle.SetMessage("Refreshing library index")
		sc := scanner.New(h.db, h.mediaPath, h.thumbPath, h.log)
		if err := sc.Run(ctx); err != nil {
			return nil, fmt.Errorf("scan library: %w", err)
		}

		handle.SetProgress(2, 3)
		handle.SetMessage("Generating thumbnails")
		if err := h.generateThumbnailsForOrganizedItems(ctx, organizedStats); err != nil {
			h.log.Warn("library: generate thumbnails after organize", zap.Error(err))
		}

		handle.SetProgress(3, 3)
		handle.SetMessage("Library organize completed")
		return organizedStats, nil
	})

	respondAccepted(c, gin.H{
		"message": "organize queued",
		"task_id": task.ID,
	}, nil)
}

type organizeStats struct {
	SourcePath string                `json:"source_path"`
	Mode       string                `json:"mode"`
	Moved      int                   `json:"moved"`
	Skipped    int                   `json:"skipped"`
	Preview    bool                  `json:"preview"`
	Items      []organizePreviewItem `json:"items,omitempty"`
}

type organizePreviewItem struct {
	SourcePath string `json:"source_path"`
	TargetPath string `json:"target_path"`
	MediaType  string `json:"media_type"`
	Action     string `json:"action"`
	Skipped    bool   `json:"skipped"`
	Reason     string `json:"reason,omitempty"`
}

var organizeFolders = map[models.MediaType]string{
	models.MediaTypeVideo:     filepath.Join("Video", "3D (Real)"),
	models.MediaTypeImage:     filepath.Join("Image", "Random"),
	models.MediaTypeManga:     filepath.Join("Comics", "Manga"),
	models.MediaTypeComic:     filepath.Join("Comics", "Manga"),
	models.MediaTypeDoujinshi: filepath.Join("Comics", "Doujins"),
}

func (h *LibraryHandler) organizeDirectory(sourcePath, mode string, preview bool) (*organizeStats, error) {
	stats := &organizeStats{SourcePath: sourcePath, Mode: mode, Preview: preview}

	err := filepath.WalkDir(sourcePath, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}

		ext := strings.ToLower(filepath.Ext(d.Name()))
		mediaType, ok := scanner.MediaTypeForExtension(ext)
		if !ok {
			stats.Skipped++
			stats.Items = append(stats.Items, organizePreviewItem{
				SourcePath: path,
				Action:     mode,
				Skipped:    true,
				Reason:     "unsupported file type",
			})
			return nil
		}

		targetDirName := classifyOrganizeTarget(sourcePath, path, d.Name(), mediaType)
		targetDir := filepath.Join(h.mediaPath, targetDirName)
		if err := os.MkdirAll(targetDir, 0o755); err != nil {
			return fmt.Errorf("mkdir %s: %w", targetDir, err)
		}

		targetPath := uniqueTargetPath(targetDir, d.Name())
		stats.Items = append(stats.Items, organizePreviewItem{
			SourcePath: path,
			TargetPath: targetPath,
			MediaType:  string(mediaType),
			Action:     mode,
		})
		if preview {
			stats.Moved++
			return nil
		}
		if mode == "copy" {
			if err := copyFile(path, targetPath); err != nil {
				return err
			}
		} else {
			if err := moveFile(path, targetPath); err != nil {
				return err
			}
		}

		stats.Moved++
		return nil
	})
	if err != nil {
		return nil, err
	}

	return stats, nil
}

func resolveLibraryPath(mediaRoot, inboxRoot, raw string) (string, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return "", fmt.Errorf("source_path is required")
	}

	var candidate string
	if filepath.IsAbs(raw) {
		candidate = filepath.Clean(raw)
	} else if raw == "inbox" || strings.HasPrefix(raw, "inbox"+string(filepath.Separator)) || strings.HasPrefix(raw, "inbox/") {
		trimmed := strings.TrimPrefix(strings.TrimPrefix(raw, "inbox/"), "inbox"+string(filepath.Separator))
		candidate = filepath.Join(inboxRoot, trimmed)
	} else {
		candidate = filepath.Join(mediaRoot, raw)
	}

	allowedRoots := []string{mediaRoot}
	if strings.TrimSpace(inboxRoot) != "" {
		allowedRoots = append(allowedRoots, inboxRoot)
	}
	var err error
	candidate, err = ensureManagedPath(candidate, allowedRoots...)
	if err != nil {
		return "", fmt.Errorf("source_path must stay inside /media or /inbox")
	}

	info, err := os.Stat(candidate)
	if err != nil {
		return "", fmt.Errorf("source_path not found")
	}
	if !info.IsDir() {
		return "", fmt.Errorf("source_path must be a directory")
	}

	return candidate, nil
}

func moveFile(src, dst string) error {
	if err := os.Rename(src, dst); err == nil {
		return nil
	}
	if err := copyFile(src, dst); err != nil {
		return err
	}
	return os.Remove(src)
}

func copyFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return fmt.Errorf("open %s: %w", src, err)
	}
	defer in.Close()

	out, err := os.Create(dst)
	if err != nil {
		return fmt.Errorf("create %s: %w", dst, err)
	}
	defer out.Close()

	if _, err := io.Copy(out, in); err != nil {
		return fmt.Errorf("copy %s: %w", src, err)
	}
	return out.Close()
}

func uniqueTargetPath(dir, name string) string {
	base := strings.TrimSuffix(name, filepath.Ext(name))
	ext := filepath.Ext(name)
	candidate := filepath.Join(dir, name)
	idx := 1
	for {
		if _, err := os.Stat(candidate); os.IsNotExist(err) {
			return candidate
		}
		candidate = filepath.Join(dir, fmt.Sprintf("%s (%d)%s", base, idx, ext))
		idx++
	}
}

func classifyOrganizeTarget(sourceRoot, fullPath, fileName string, mediaType models.MediaType) string {
	defaultDir, ok := organizeFolders[mediaType]
	if !ok {
		return "Other"
	}

	lowerName := strings.ToLower(fileName)
	lowerPath := strings.ToLower(fullPath)
	relPath, err := filepath.Rel(sourceRoot, fullPath)
	if err == nil {
		lowerPath = strings.ToLower(relPath)
	}

	switch mediaType {
	case models.MediaTypeVideo:
		if looksLike2DVideo(lowerName, lowerPath) {
			return filepath.Join("Video", "2D (Hentai)")
		}
		studio := deriveStudioFolder(sourceRoot, fullPath)
		if studio != "" {
			return filepath.Join("Video", "3D (Real)", studio)
		}
		return filepath.Join("Video", "3D (Real)")
	case models.MediaTypeImage:
		if strings.EqualFold(filepath.Ext(fileName), ".gif") {
			return filepath.Join("Image", "GIFs")
		}
		if strings.Contains(lowerName, "cg") || strings.Contains(lowerPath, "cg") {
			return filepath.Join("Image", "CG Sets")
		}
		return filepath.Join("Image", "Random")
	case models.MediaTypeDoujinshi:
		return filepath.Join("Comics", "Doujins")
	case models.MediaTypeManga, models.MediaTypeComic:
		return filepath.Join("Comics", "Manga")
	default:
		return defaultDir
	}
}

func looksLike2DVideo(lowerName, lowerPath string) bool {
	for _, marker := range []string{
		"hentai", "anime", "ova", "doujin", "2d", "animated", "uncensored", "subbed",
	} {
		if strings.Contains(lowerName, marker) || strings.Contains(lowerPath, marker) {
			return true
		}
	}
	return false
}

func deriveStudioFolder(sourceRoot, fullPath string) string {
	rel, err := filepath.Rel(sourceRoot, fullPath)
	if err != nil {
		return ""
	}
	parts := strings.Split(filepath.ToSlash(rel), "/")
	if len(parts) < 2 {
		return ""
	}

	candidate := strings.TrimSpace(parts[0])
	if candidate == "" {
		return ""
	}
	lower := strings.ToLower(candidate)
	for _, generic := range []string{
		"inbox", "unsorted", "video", "videos", "3d", "2d", "real", "new", "downloads",
	} {
		if lower == generic {
			return ""
		}
	}
	return sanitizeFolderName(candidate)
}

func sanitizeFolderName(name string) string {
	name = strings.TrimSpace(name)
	if name == "" {
		return ""
	}

	replacer := strings.NewReplacer(
		"<", "",
		">", "",
		":", " -",
		"\"", "",
		"/", "-",
		"\\", "-",
		"|", "-",
		"?", "",
		"*", "",
	)
	name = replacer.Replace(name)
	name = strings.Join(strings.Fields(name), " ")
	name = strings.Trim(name, ". ")
	return name
}

func (h *LibraryHandler) generateThumbnailsForOrganizedItems(ctx context.Context, stats *organizeStats) error {
	if h.thumbPath == "" {
		return nil
	}

	gen := thumbnails.New(h.thumbPath, h.log)
	for _, item := range stats.Items {
		if item.Skipped || item.TargetPath == "" {
			continue
		}

		var media models.Media
		if err := h.db.GetContext(ctx, &media, `
			SELECT * FROM media
			WHERE file_path = $1 AND deleted_at IS NULL
		`, item.TargetPath); err != nil {
			continue
		}
		if strings.TrimSpace(media.ThumbnailPath) != "" {
			continue
		}

		thumbPath, err := gen.GenerateForMedia(ctx, &media)
		if err != nil {
			h.log.Warn("library: thumbnail generation failed",
				zap.String("path", media.FilePath),
				zap.Error(err),
			)
			continue
		}
		if _, err := h.db.ExecContext(ctx, `
			UPDATE media SET thumbnail_path = $1, updated_at = NOW()
			WHERE id = $2
		`, thumbPath, media.ID); err != nil {
			h.log.Warn("library: update thumbnail path",
				zap.String("id", media.ID),
				zap.Error(err),
			)
		}
	}

	return nil
}
