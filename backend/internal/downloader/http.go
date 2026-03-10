package downloader

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/nacl-dev/tanuki/internal/models"
	"go.uber.org/zap"
)

// HTTPEngine downloads files via plain HTTP/HTTPS.
type HTTPEngine struct {
	client *http.Client
	log    *zap.Logger
}

// NewHTTPEngine creates an HTTPEngine.
func NewHTTPEngine(log *zap.Logger) *HTTPEngine {
	return &HTTPEngine{
		client: &http.Client{},
		log:    log,
	}
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
		return fmt.Errorf("unexpected status: %d", resp.StatusCode)
	}

	// Derive filename from URL path.
	filename := filepath.Base(req.URL.Path)
	if filename == "" || filename == "." {
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

	written, err := io.Copy(f, resp.Body)
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
