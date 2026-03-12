package api

import (
	"errors"
	"mime"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/nacl-dev/tanuki/internal/models"
)

var errUnsupportedPagedMedia = errors.New("media is not a paged type")

func listDirectoryImages(path string) ([]string, error) {
	entries, err := os.ReadDir(path)
	if err != nil {
		return nil, err
	}

	names := make([]string, 0, len(entries))
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		if !isImageFile(entry.Name()) {
			continue
		}
		names = append(names, entry.Name())
	}

	sort.Strings(names)
	return names, nil
}

func listPageNamesForMediaPath(mediaPath, mediaRoot string) ([]string, error) {
	managedPath, err := ensureManagedPath(mediaPath, mediaRoot)
	if err != nil {
		return nil, err
	}

	info, err := os.Stat(managedPath)
	if err != nil {
		return nil, err
	}
	if info.IsDir() {
		return listDirectoryImages(managedPath)
	}

	switch strings.ToLower(filepath.Ext(managedPath)) {
	case ".zip", ".cbz":
		return listZipImages(managedPath)
	case ".cbr", ".rar":
		return listRARImages(managedPath)
	default:
		return nil, errUnsupportedPagedMedia
	}
}

func servePageForMediaPath(c *gin.Context, mediaPath, mediaRoot string, pageIdx int) error {
	managedPath, err := ensureManagedPath(mediaPath, mediaRoot)
	if err != nil {
		return err
	}

	info, err := os.Stat(managedPath)
	if err != nil {
		return err
	}
	if info.IsDir() {
		return serveDirectoryPage(c, managedPath, mediaRoot, pageIdx)
	}

	switch strings.ToLower(filepath.Ext(managedPath)) {
	case ".zip", ".cbz":
		return serveZipPage(c, managedPath, pageIdx)
	case ".cbr", ".rar":
		return serveRARPage(c, managedPath, pageIdx)
	default:
		return errUnsupportedPagedMedia
	}
}

func serveDirectoryPage(c *gin.Context, dirPath, mediaRoot string, pageIdx int) error {
	names, err := listDirectoryImages(dirPath)
	if err != nil {
		return err
	}
	if pageIdx >= len(names) {
		c.Status(http.StatusNotFound)
		return nil
	}

	pagePath, err := ensureManagedPath(filepath.Join(dirPath, names[pageIdx]), mediaRoot)
	if err != nil {
		return err
	}
	if _, err := os.Stat(pagePath); err != nil {
		return err
	}

	if ct := mime.TypeByExtension(strings.ToLower(filepath.Ext(pagePath))); ct != "" {
		c.Header("Content-Type", ct)
	}
	c.File(pagePath)
	return nil
}

func isPagedMedia(item models.Media, mediaRoot string) bool {
	if item.Type != models.MediaTypeManga && item.Type != models.MediaTypeComic && item.Type != models.MediaTypeDoujinshi {
		return false
	}

	_, err := listPageNamesForMediaPath(item.FilePath, mediaRoot)
	return err == nil
}
