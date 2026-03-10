package downloader

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/nacl-dev/tanuki/internal/models"
	"go.uber.org/zap"
)

// GalleryDLEngine wraps the gallery-dl CLI tool.
type GalleryDLEngine struct {
	configPath string
	log        *zap.Logger
}

// NewGalleryDLEngine creates a GalleryDLEngine using an optional config file.
func NewGalleryDLEngine(configPath string, log *zap.Logger) *GalleryDLEngine {
	return &GalleryDLEngine{configPath: configPath, log: log}
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
	if e.configPath != "" {
		args = append(args, "--config", e.configPath)
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

	// Drain stderr
	go func() {
		scanner := bufio.NewScanner(stderr)
		for scanner.Scan() {
			e.log.Debug("gallery-dl", zap.String("stderr", scanner.Text()))
		}
	}()

	// Parse stdout for downloaded file paths
	scanner := bufio.NewScanner(stdout)
	count := 0
	for scanner.Scan() {
		line := scanner.Text()
		if filepath.IsAbs(line) || strings.HasPrefix(line, dest) {
			count++
			e.log.Debug("gallery-dl: downloaded", zap.String("file", line), zap.Int("count", count))
		}
	}

	if err := cmd.Wait(); err != nil {
		return fmt.Errorf("gallery-dl: %w", err)
	}

	return nil
}

// FetchMetadata uses gallery-dl --dump-json to retrieve metadata without downloading.
func (e *GalleryDLEngine) FetchMetadata(url string) (*SourceMetadata, error) {
	args := []string{"--dump-json", url}
	if e.configPath != "" {
		args = append([]string{"--config", e.configPath}, args...)
	}

	out, err := exec.Command("gallery-dl", args...).Output()
	if err != nil {
		return nil, fmt.Errorf("gallery-dl metadata: %w", err)
	}

	// gallery-dl outputs one JSON object per line.
	var meta SourceMetadata
	lines := strings.SplitN(string(out), "\n", 2)
	if len(lines) > 0 {
		var obj map[string]interface{}
		if jsonErr := json.Unmarshal([]byte(lines[0]), &obj); jsonErr == nil {
			if t, ok := obj["title"].(string); ok {
				meta.Title = t
			}
		}
	}

	return &meta, nil
}
