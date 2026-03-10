package downloader

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"math"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/nacl-dev/tanuki/internal/models"
	"go.uber.org/zap"
)

// YtDlpEngine wraps the yt-dlp CLI tool.
type YtDlpEngine struct {
	configPath string
	log        *zap.Logger
	progress   func(id string, downloaded, total int64, files, totalFiles int)
}

// NewYtDlpEngine creates a YtDlpEngine using an optional config file.
func NewYtDlpEngine(configPath string, log *zap.Logger) *YtDlpEngine {
	return &YtDlpEngine{configPath: configPath, log: log}
}

func (e *YtDlpEngine) SetProgressUpdater(fn func(id string, downloaded, total int64, files, totalFiles int)) {
	e.progress = fn
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
	meta, _ := e.extractMetadata(job.URL)

	args := []string{
		"--output", dest + "/%(title)s.%(ext)s",
		"--write-info-json",
		"--no-playlist",
		"--progress",
		"--newline",
		"--progress-template", "download:tanuki:%(progress.downloaded_bytes)s:%(progress.total_bytes_estimate)s:%(progress.total_bytes)s",
	}
	if e.hasConfig() {
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
		if e.progress != nil {
			if downloaded, total, ok := parseYtDlpProgress(line); ok {
				e.progress(job.ID, downloaded, total, 0, 1)
			}
		}
		e.log.Debug("yt-dlp", zap.String("stdout", line))
	}

	if err := cmd.Wait(); err != nil {
		return fmt.Errorf("yt-dlp: %w", err)
	}

	if meta != nil {
		if outputPath, err := detectLatestDownloadedFile(dest); err == nil {
			sidecar := models.ImportMetadata{
				Title:     meta.Title,
				SourceURL: firstNonEmpty(strings.TrimSpace(meta.Extra["source_url"]), job.URL),
				PosterURL: strings.TrimSpace(meta.Extra["poster_url"]),
				Tags:      meta.Tags,
			}
			if err := writeImportMetadata(outputPath, sidecar); err != nil {
				e.log.Warn("yt-dlp: write metadata sidecar failed", zap.String("path", outputPath), zap.Error(err))
			}
		}
	}

	return nil
}

// FetchMetadata uses yt-dlp --dump-json to retrieve video metadata.
func (e *YtDlpEngine) FetchMetadata(url string) (*SourceMetadata, error) {
	return e.extractMetadata(url)
}

func (e *YtDlpEngine) extractMetadata(url string) (*SourceMetadata, error) {
	args := []string{"--dump-json", "--no-playlist"}
	if e.hasConfig() {
		args = append(args, "--config-location", e.configPath)
	}
	args = append(args, url)

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
	meta.Extra = map[string]string{}
	if webpageURL, ok := obj["webpage_url"].(string); ok {
		meta.Extra["source_url"] = webpageURL
	}
	if thumbnail, ok := obj["thumbnail"].(string); ok {
		meta.Extra["poster_url"] = thumbnail
	}

	meta.TotalFiles = 1
	return meta, nil
}

func (e *YtDlpEngine) hasConfig() bool {
	if e.configPath == "" {
		return false
	}
	if _, err := os.Stat(e.configPath); err != nil {
		return false
	}
	return true
}

func parseYtDlpProgress(line string) (int64, int64, bool) {
	const prefix = "download:tanuki:"
	if !strings.HasPrefix(line, prefix) {
		return 0, 0, false
	}

	parts := strings.Split(strings.TrimPrefix(line, prefix), ":")
	if len(parts) != 3 {
		return 0, 0, false
	}

	downloaded := parseProgressNumber(parts[0])
	estimated := parseProgressNumber(parts[1])
	total := parseProgressNumber(parts[2])
	if total <= 0 {
		total = estimated
	}
	if downloaded < 0 {
		downloaded = 0
	}
	return downloaded, total, true
}

func parseProgressNumber(raw string) int64 {
	raw = strings.TrimSpace(raw)
	if raw == "" || strings.EqualFold(raw, "NA") {
		return 0
	}

	value, err := strconv.ParseFloat(raw, 64)
	if err != nil || math.IsNaN(value) || math.IsInf(value, 0) {
		return 0
	}
	if value < 0 {
		return 0
	}
	return int64(value)
}

func detectLatestDownloadedFile(dir string) (string, error) {
	var latestPath string
	var latestTime int64

	err := filepath.WalkDir(dir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		lower := strings.ToLower(path)
		if strings.HasSuffix(lower, ".info.json") || strings.HasSuffix(lower, ".tanuki.json") || strings.HasSuffix(lower, ".part") {
			return nil
		}
		info, err := d.Info()
		if err != nil {
			return nil
		}
		if info.ModTime().UnixNano() > latestTime {
			latestTime = info.ModTime().UnixNano()
			latestPath = path
		}
		return nil
	})
	if err != nil {
		return "", err
	}
	if latestPath == "" {
		return "", fmt.Errorf("no downloaded media file found in %s", dir)
	}
	return latestPath, nil
}
