package downloader

import (
	"context"
	"fmt"
	"io"
	"mime"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/nacl-dev/tanuki/internal/models"
	"go.uber.org/zap"
)

// HTTPEngine downloads files via plain HTTP/HTTPS.
type HTTPEngine struct {
	client   *http.Client
	log      *zap.Logger
	progress func(id string, downloaded, total int64, files, totalFiles int)
}

// NewHTTPEngine creates an HTTPEngine.
func NewHTTPEngine(cookiesPath string, log *zap.Logger) *HTTPEngine {
	client, err := newHTTPClientWithCookies(cookiesPath)
	if err != nil {
		log.Warn("http: cookies unavailable", zap.String("path", cookiesPath), zap.Error(err))
		client = &http.Client{}
	}
	return &HTTPEngine{
		client: client,
		log:    log,
	}
}

func (e *HTTPEngine) SetProgressUpdater(fn func(id string, downloaded, total int64, files, totalFiles int)) {
	e.progress = fn
}

// CanHandle accepts any http/https URL as a fallback.
func (e *HTTPEngine) CanHandle(url string) bool {
	lower := strings.ToLower(url)
	return strings.HasPrefix(lower, "http://") || strings.HasPrefix(lower, "https://")
}

// Download fetches the URL and writes the response body to the target directory.
func (e *HTTPEngine) Download(ctx context.Context, job *models.DownloadJob) error {
	dest := job.TargetDirectory
	if dest == "" {
		dest = "/downloads"
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, job.URL, nil)
	if err != nil {
		return fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("User-Agent", "Tanuki/1.0")

	resp, err := e.client.Do(req)
	if err != nil {
		return fmt.Errorf("http get: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		if resp.StatusCode == http.StatusForbidden || resp.StatusCode == http.StatusUnauthorized || resp.StatusCode == http.StatusTooManyRequests {
			return newUnsupportedURLError("http", blockedSourceDetail(resp.StatusCode))
		}
		return fmt.Errorf("unexpected status: %d", resp.StatusCode)
	}

	contentType := normalizeContentType(resp.Header.Get("Content-Type"))
	if !isDownloadableContentType(contentType) {
		return newUnsupportedURLError("http", "response is not a downloadable media file ("+contentType+")")
	}

	filename := filenameFromResponse(resp, req.URL.Path)
	if filepath.Ext(filename) == "" {
		if ext := extensionForContentType(contentType); ext != "" {
			filename += ext
		}
	}
	if filename == "" || filename == "." || filename == string(filepath.Separator) {
		filename = "download"
	}

	if err := os.MkdirAll(dest, 0o755); err != nil {
		return fmt.Errorf("mkdir: %w", err)
	}

	outPath := filepath.Join(dest, filename)
	f, err := os.Create(outPath)
	if err != nil {
		return fmt.Errorf("create file: %w", err)
	}
	defer f.Close()

	written, err := copyWithProgress(resp.Body, f, resp.ContentLength, func(downloaded int64, total int64) {
		if e.progress != nil {
			e.progress(job.ID, downloaded, total, 0, 1)
		}
	})
	if err != nil {
		return fmt.Errorf("write file: %w", err)
	}

	e.log.Info("http: downloaded", zap.String("path", outPath), zap.Int64("bytes", written))
	return nil
}

// FetchMetadata does a HEAD request to get content-length.
func (e *HTTPEngine) FetchMetadata(url string) (*SourceMetadata, error) {
	resp, err := e.client.Head(url)
	if err != nil {
		return nil, fmt.Errorf("head request: %w", err)
	}
	defer resp.Body.Close()

	return &SourceMetadata{
		Title:      filepath.Base(url),
		TotalFiles: 1,
	}, nil
}

func normalizeContentType(value string) string {
	return strings.ToLower(strings.TrimSpace(strings.Split(value, ";")[0]))
}

func isDownloadableContentType(contentType string) bool {
	if contentType == "" {
		return true
	}

	switch {
	case strings.HasPrefix(contentType, "video/"):
		return true
	case strings.HasPrefix(contentType, "image/"):
		return true
	case contentType == "application/octet-stream":
		return true
	case contentType == "application/zip":
		return true
	case contentType == "application/x-zip-compressed":
		return true
	case contentType == "application/x-rar-compressed":
		return true
	case contentType == "application/vnd.comicbook+zip":
		return true
	case contentType == "application/vnd.comicbook-rar":
		return true
	default:
		return false
	}
}

func filenameFromResponse(resp *http.Response, requestPath string) string {
	if cd := resp.Header.Get("Content-Disposition"); cd != "" {
		if _, params, err := mime.ParseMediaType(cd); err == nil {
			if name := strings.TrimSpace(params["filename"]); name != "" {
				return filepath.Base(name)
			}
		}
	}

	name := filepath.Base(requestPath)
	if name == "" || name == "." || name == "/" {
		return "download"
	}
	return name
}

func extensionForContentType(contentType string) string {
	exts, err := mime.ExtensionsByType(contentType)
	if err != nil || len(exts) == 0 {
		return ""
	}
	return exts[0]
}

func copyWithProgress(src io.Reader, dst io.Writer, total int64, onProgress func(downloaded int64, total int64)) (int64, error) {
	buf := make([]byte, 32*1024)
	var written int64
	for {
		nr, er := src.Read(buf)
		if nr > 0 {
			nw, ew := dst.Write(buf[0:nr])
			if nw > 0 {
				written += int64(nw)
				if onProgress != nil {
					onProgress(written, total)
				}
			}
			if ew != nil {
				return written, ew
			}
			if nr != nw {
				return written, io.ErrShortWrite
			}
		}
		if er != nil {
			if er == io.EOF {
				return written, nil
			}
			return written, er
		}
	}
}
