package api

import (
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/nacl-dev/tanuki/internal/downloader"
	"github.com/nacl-dev/tanuki/internal/importmeta"
	"github.com/nacl-dev/tanuki/internal/models"
	"github.com/nacl-dev/tanuki/internal/scanner"
)

type inboxUploadResult struct {
	BatchName   string   `json:"batch_name"`
	SourcePath  string   `json:"source_path"`
	FileCount   int      `json:"file_count"`
	TotalBytes  int64    `json:"total_bytes"`
	DefaultTags []string `json:"default_tags,omitempty"`
}

// UploadInbox stores uploaded files in a dedicated inbox batch folder so they
// can be organized into the library afterwards.
// POST /api/library/inbox/upload
func (h *LibraryHandler) UploadInbox(c *gin.Context) {
	if strings.TrimSpace(h.inboxPath) == "" {
		respondError(c, http.StatusConflict, "inbox path is not configured")
		return
	}

	form, err := c.MultipartForm()
	if err != nil {
		respondError(c, http.StatusBadRequest, "invalid upload form")
		return
	}

	files := append([]*multipart.FileHeader{}, form.File["files"]...)
	if len(files) == 0 {
		files = append(files, form.File["file"]...)
	}
	if len(files) == 0 {
		respondError(c, http.StatusBadRequest, "missing upload files")
		return
	}

	batchName := uniqueInboxBatchName(h.inboxPath, generateInboxBatchName(firstNonEmpty(form.Value["batch_name"]...)))
	defaultTags, _, err := prepareAutoTags(c.Request.Context(), h.db, form.Value["default_tags"])
	if err != nil {
		respondError(c, http.StatusInternalServerError, "prepare default tags: "+err.Error())
		return
	}
	relativePaths := form.Value["paths"]
	sanitizedPaths := make([]string, 0, len(files))
	for index, fileHeader := range files {
		relativePath := fileHeader.Filename
		if index < len(relativePaths) && strings.TrimSpace(relativePaths[index]) != "" {
			relativePath = relativePaths[index]
		}

		sanitizedPath, err := sanitizeInboxRelativePath(relativePath, fileHeader.Filename)
		if err != nil {
			respondError(c, http.StatusBadRequest, err.Error())
			return
		}
		sanitizedPaths = append(sanitizedPaths, sanitizedPath)
	}

	batchRoot, err := ensureManagedPath(filepath.Join(h.inboxPath, batchName), h.inboxPath)
	if err != nil {
		respondError(c, http.StatusBadRequest, "invalid inbox batch")
		return
	}
	if err := os.MkdirAll(batchRoot, 0o755); err != nil {
		respondError(c, http.StatusInternalServerError, "create inbox batch: "+err.Error())
		return
	}

	var totalBytes int64
	savedMediaPaths := make([]string, 0, len(files))
	for index, fileHeader := range files {
		targetPath, err := prepareInboxTargetPath(batchRoot, sanitizedPaths[index])
		if err != nil {
			_ = os.RemoveAll(batchRoot)
			respondError(c, http.StatusBadRequest, err.Error())
			return
		}
		if err := os.MkdirAll(filepath.Dir(targetPath), 0o755); err != nil {
			_ = os.RemoveAll(batchRoot)
			respondError(c, http.StatusInternalServerError, "prepare inbox path: "+err.Error())
			return
		}

		if err := saveUploadedMultipartFile(fileHeader, targetPath); err != nil {
			_ = os.RemoveAll(batchRoot)
			respondError(c, http.StatusInternalServerError, "save upload: "+err.Error())
			return
		}
		totalBytes += fileHeader.Size
		if _, ok := scanner.MediaTypeForExtension(strings.ToLower(filepath.Ext(targetPath))); ok {
			savedMediaPaths = append(savedMediaPaths, targetPath)
		}
	}

	if len(defaultTags) > 0 {
		for _, mediaPath := range savedMediaPaths {
			if err := mergeInboxImportMetadata(mediaPath, models.ImportMetadata{Tags: defaultTags}); err != nil {
				_ = os.RemoveAll(batchRoot)
				respondError(c, http.StatusInternalServerError, "write upload metadata: "+err.Error())
				return
			}
		}
	}

	respondOK(c, inboxUploadResult{
		BatchName:   batchName,
		SourcePath:  filepath.ToSlash(filepath.Join("inbox", batchName)),
		FileCount:   len(files),
		TotalBytes:  totalBytes,
		DefaultTags: defaultTags,
	}, nil)
}

func generateInboxBatchName(raw string) string {
	name := sanitizeFolderName(strings.TrimSpace(raw))
	if name == "" {
		name = "upload-" + time.Now().UTC().Format("20060102-150405")
	}
	return name
}

func uniqueInboxBatchName(inboxRoot, baseName string) string {
	candidate := baseName
	index := 1
	for {
		target, err := ensureManagedPath(filepath.Join(inboxRoot, candidate), inboxRoot)
		if err != nil {
			return candidate
		}
		if _, err := os.Stat(target); os.IsNotExist(err) {
			return candidate
		}
		candidate = fmt.Sprintf("%s (%d)", baseName, index)
		index++
	}
}

func sanitizeInboxRelativePath(raw, fallback string) (string, error) {
	candidate := strings.TrimSpace(raw)
	if candidate == "" {
		candidate = strings.TrimSpace(fallback)
	}
	candidate = strings.ReplaceAll(candidate, "\\", "/")
	if candidate == "" {
		return "", fmt.Errorf("upload path is empty")
	}

	cleaned := path.Clean(candidate)
	if cleaned == "." {
		cleaned = path.Clean(strings.TrimSpace(fallback))
	}
	if cleaned == "." || cleaned == "" {
		return "", fmt.Errorf("upload path is empty")
	}
	if strings.HasPrefix(cleaned, "/") || cleaned == ".." || strings.HasPrefix(cleaned, "../") {
		return "", fmt.Errorf("upload paths must stay inside the inbox batch")
	}
	if volume := filepath.VolumeName(cleaned); volume != "" {
		return "", fmt.Errorf("upload paths must be relative")
	}

	parts := strings.Split(cleaned, "/")
	sanitized := make([]string, 0, len(parts))
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part == "" || part == "." {
			continue
		}
		if part == ".." {
			return "", fmt.Errorf("upload paths must stay inside the inbox batch")
		}
		part = sanitizeFolderName(part)
		if part == "" {
			return "", fmt.Errorf("upload path contains an invalid segment")
		}
		sanitized = append(sanitized, part)
	}
	if len(sanitized) == 0 {
		return "", fmt.Errorf("upload path is empty")
	}

	return filepath.Join(sanitized...), nil
}

func prepareInboxTargetPath(batchRoot, relativePath string) (string, error) {
	targetPath, err := ensureManagedPath(filepath.Join(batchRoot, relativePath), batchRoot)
	if err != nil {
		return "", fmt.Errorf("upload path must stay inside the inbox batch")
	}
	if _, err := os.Stat(targetPath); err == nil {
		targetPath = uniqueTargetPath(filepath.Dir(targetPath), filepath.Base(targetPath))
	}
	return targetPath, nil
}

func saveUploadedMultipartFile(fileHeader *multipart.FileHeader, targetPath string) error {
	src, err := fileHeader.Open()
	if err != nil {
		return err
	}
	defer src.Close()

	dst, err := os.OpenFile(targetPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o644)
	if err != nil {
		return err
	}
	defer dst.Close()

	if _, err := io.Copy(dst, src); err != nil {
		return err
	}
	return dst.Close()
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return value
		}
	}
	return ""
}

func mergeInboxImportMetadata(mediaPath string, updates models.ImportMetadata) error {
	metadata, err := importmeta.LoadMedia(mediaPath)
	if err != nil {
		if !os.IsNotExist(err) {
			return err
		}
		metadata = nil
	}
	if metadata == nil {
		metadata = &models.ImportMetadata{}
	}

	if title := strings.TrimSpace(updates.Title); title != "" && strings.TrimSpace(metadata.Title) == "" {
		metadata.Title = title
	}
	if workTitle := strings.TrimSpace(updates.WorkTitle); workTitle != "" && strings.TrimSpace(metadata.WorkTitle) == "" {
		metadata.WorkTitle = workTitle
	}
	if updates.WorkIndex > 0 && metadata.WorkIndex == 0 {
		metadata.WorkIndex = updates.WorkIndex
	}
	if sourceURL := strings.TrimSpace(updates.SourceURL); sourceURL != "" && strings.TrimSpace(metadata.SourceURL) == "" {
		metadata.SourceURL = sourceURL
	}
	if posterURL := strings.TrimSpace(updates.PosterURL); posterURL != "" && strings.TrimSpace(metadata.PosterURL) == "" {
		metadata.PosterURL = posterURL
	}
	metadata.Tags = downloader.NormalizeDownloadAutoTags(append(metadata.Tags, updates.Tags...))

	body, err := json.MarshalIndent(metadata, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(mediaPath+".tanuki.json", body, 0o644)
}
