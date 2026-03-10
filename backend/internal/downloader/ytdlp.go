package downloader

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"strings"

	"github.com/nacl-dev/tanuki/internal/models"
	"go.uber.org/zap"
)

// YtDlpEngine wraps the yt-dlp CLI tool.
type YtDlpEngine struct {
	configPath string
	log        *zap.Logger
}

// NewYtDlpEngine creates a YtDlpEngine using an optional config file.
func NewYtDlpEngine(configPath string, log *zap.Logger) *YtDlpEngine {
	return &YtDlpEngine{configPath: configPath, log: log}
}

// CanHandle returns true for video-hosting URLs that yt-dlp handles well.
func (e *YtDlpEngine) CanHandle(url string) bool {
	lower := strings.ToLower(url)
	videoHosts := []string{
		"youtube.com", "youtu.be", "twitch.tv", "vimeo.com",
		"dailymotion.com", "nicovideo.jp", "iwara.tv", "pornhub.com",
		"xvideos.com", "xhamster.com",
	}
	for _, h := range videoHosts {
		if strings.Contains(lower, h) {
			return true
		}
	}
	return false
}

// Download runs yt-dlp for the given job.
func (e *YtDlpEngine) Download(ctx context.Context, job *models.DownloadJob) error {
	dest := job.TargetDirectory
	if dest == "" {
		dest = "/downloads"
	}

	args := []string{
		"--output", dest + "/%(title)s.%(ext)s",
		"--write-info-json",
		"--no-playlist",
		"--progress",
		"--newline",
	}
	if e.configPath != "" {
		args = append(args, "--config-location", e.configPath)
	}
	args = append(args, job.URL)

	cmd := exec.CommandContext(ctx, "yt-dlp", args...)
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return fmt.Errorf("yt-dlp stdout pipe: %w", err)
	}
	stderr, err := cmd.StderrPipe()
	if err != nil {
		return fmt.Errorf("yt-dlp stderr pipe: %w", err)
	}

	if err := cmd.Start(); err != nil {
		return fmt.Errorf("yt-dlp start: %w", err)
	}

	go func() {
		scanner := bufio.NewScanner(stderr)
		for scanner.Scan() {
			e.log.Debug("yt-dlp", zap.String("stderr", scanner.Text()))
		}
	}()

	scanner := bufio.NewScanner(stdout)
	for scanner.Scan() {
		line := scanner.Text()
		e.log.Debug("yt-dlp", zap.String("stdout", line))
	}

	if err := cmd.Wait(); err != nil {
		return fmt.Errorf("yt-dlp: %w", err)
	}

	return nil
}

// FetchMetadata uses yt-dlp --dump-json to retrieve video metadata.
func (e *YtDlpEngine) FetchMetadata(url string) (*SourceMetadata, error) {
	args := []string{"--dump-json", "--no-playlist", url}

	out, err := exec.Command("yt-dlp", args...).Output()
	if err != nil {
		return nil, fmt.Errorf("yt-dlp metadata: %w", err)
	}

	var obj map[string]interface{}
	if jsonErr := json.Unmarshal(out, &obj); jsonErr != nil {
		return nil, fmt.Errorf("parse metadata: %w", jsonErr)
	}

	meta := &SourceMetadata{}
	if t, ok := obj["title"].(string); ok {
		meta.Title = t
	}
	if d, ok := obj["description"].(string); ok {
		meta.Description = d
	}
	if tags, ok := obj["tags"].([]interface{}); ok {
		for _, t := range tags {
			if s, ok := t.(string); ok {
				meta.Tags = append(meta.Tags, s)
			}
		}
	}

	return meta, nil
}
