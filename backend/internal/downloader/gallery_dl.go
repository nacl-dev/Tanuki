package downloader

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"

	"github.com/nacl-dev/tanuki/internal/importmeta"
	"github.com/nacl-dev/tanuki/internal/models"
	"go.uber.org/zap"
)

// GalleryDLEngine wraps the gallery-dl CLI tool.
type GalleryDLEngine struct {
	configPath  string
	cookiesPath string
	log         *zap.Logger
	progress    func(id string, downloaded, total int64, files, totalFiles int)
}

type logBuffer struct {
	mu    sync.Mutex
	lines []string
}

func (b *logBuffer) Add(line string) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.lines = append(b.lines, line)
}

func (b *logBuffer) Joined() string {
	b.mu.Lock()
	defer b.mu.Unlock()
	return strings.Join(b.lines, "\n")
}

// NewGalleryDLEngine creates a GalleryDLEngine using an optional config file.
func NewGalleryDLEngine(configPath, cookiesPath string, log *zap.Logger) *GalleryDLEngine {
	return &GalleryDLEngine{configPath: configPath, cookiesPath: cookiesPath, log: log}
}

func (e *GalleryDLEngine) SetProgressUpdater(fn func(id string, downloaded, total int64, files, totalFiles int)) {
	e.progress = fn
}

// CanHandle returns true for any URL that gallery-dl is likely to support.
// gallery-dl supports 300+ sites; we use a deny-list for video-only sites that
// yt-dlp handles better.
func (e *GalleryDLEngine) CanHandle(url string) bool {
	lower := strings.ToLower(url)
	videoOnly := []string{"youtube.com", "youtu.be", "twitch.tv", "vimeo.com"}
	for _, v := range videoOnly {
		if strings.Contains(lower, v) {
			return false
		}
	}
	return strings.HasPrefix(lower, "http")
}

// Download runs gallery-dl for the given job and streams stdout/stderr.
func (e *GalleryDLEngine) Download(ctx context.Context, job *models.DownloadJob) error {
	dest := job.TargetDirectory
	if dest == "" {
		dest = "/downloads"
	}

	args := []string{
		"--dest", dest,
		"--write-metadata",
	}
	if e.hasConfig() {
		args = append(args, "--config", e.configPath)
	}
	if strings.TrimSpace(e.cookiesPath) != "" {
		args = append(args, "--cookies", e.cookiesPath)
	}
	args = append(args, job.URL)

	cmd := exec.CommandContext(ctx, "gallery-dl", args...)
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return fmt.Errorf("gallery-dl stdout pipe: %w", err)
	}
	stderr, err := cmd.StderrPipe()
	if err != nil {
		return fmt.Errorf("gallery-dl stderr pipe: %w", err)
	}

	if err := cmd.Start(); err != nil {
		return fmt.Errorf("gallery-dl start: %w", err)
	}

	stderrLines := &logBuffer{}
	go func() {
		scanner := bufio.NewScanner(stderr)
		for scanner.Scan() {
			line := scanner.Text()
			stderrLines.Add(line)
			e.log.Debug("gallery-dl", zap.String("stderr", line))
		}
	}()

	scanner := bufio.NewScanner(stdout)
	count := 0
	for scanner.Scan() {
		line := scanner.Text()
		if filepath.IsAbs(line) || strings.HasPrefix(line, dest) {
			count++
			if e.progress != nil {
				e.progress(job.ID, 0, 0, count, job.TotalFiles)
			}
			e.log.Debug("gallery-dl: downloaded", zap.String("file", line), zap.Int("count", count))
		}
	}

	if err := cmd.Wait(); err != nil {
		stderrText := stderrLines.Joined()
		if strings.Contains(stderrText, "Unsupported URL") {
			return newUnsupportedURLError("gallery-dl", stderrText)
		}
		if stderrText != "" {
			return fmt.Errorf("gallery-dl: %w: %s", err, stderrText)
		}
		return fmt.Errorf("gallery-dl: %w", err)
	}

	return nil
}

// FetchMetadata uses gallery-dl --dump-json to retrieve metadata without downloading.
func (e *GalleryDLEngine) FetchMetadata(url string) (*SourceMetadata, error) {
	args := []string{"--dump-json"}
	if e.hasConfig() {
		args = append(args, "--config", e.configPath)
	}
	if strings.TrimSpace(e.cookiesPath) != "" {
		args = append(args, "--cookies", e.cookiesPath)
	}
	args = append(args, url)

	out, err := exec.Command("gallery-dl", args...).Output()
	if err != nil {
		return nil, fmt.Errorf("gallery-dl metadata: %w", err)
	}

	// gallery-dl outputs one JSON object per line.
	var meta SourceMetadata
	lines := strings.SplitN(string(out), "\n", 2)
	if len(lines) > 0 {
		if parsed, recognized, jsonErr := importmeta.ParseGalleryDLMetadata([]byte(lines[0])); jsonErr == nil && recognized && parsed != nil {
			meta.Title = parsed.Title
			meta.WorkTitle = parsed.WorkTitle
			meta.WorkIndex = parsed.WorkIndex
			meta.Tags = parsed.Tags
			meta.Extra = map[string]string{}
			if strings.TrimSpace(parsed.SourceURL) != "" {
				meta.Extra["source_url"] = strings.TrimSpace(parsed.SourceURL)
			}
			if strings.TrimSpace(parsed.PosterURL) != "" {
				meta.Extra["poster_url"] = strings.TrimSpace(parsed.PosterURL)
			}
		} else {
			var obj map[string]interface{}
			if jsonErr := json.Unmarshal([]byte(lines[0]), &obj); jsonErr == nil {
				if t, ok := obj["title"].(string); ok {
					meta.Title = t
				}
			}
		}
	}

	return &meta, nil
}

func (e *GalleryDLEngine) hasConfig() bool {
	if e.configPath == "" {
		return false
	}
	if _, err := os.Stat(e.configPath); err != nil {
		return false
	}
	return true
}
