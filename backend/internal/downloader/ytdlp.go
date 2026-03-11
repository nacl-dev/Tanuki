package downloader

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io/fs"
	"math"
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
	configPath        string
	cookiesPath       string
	impersonateClient string
	log               *zap.Logger
	progress          func(id string, downloaded, total int64, files, totalFiles int)
}

// NewYtDlpEngine creates a YtDlpEngine using an optional config file.
func NewYtDlpEngine(configPath, cookiesPath, impersonateClient string, log *zap.Logger) *YtDlpEngine {
	return &YtDlpEngine{
		configPath:        configPath,
		cookiesPath:       cookiesPath,
		impersonateClient: strings.TrimSpace(impersonateClient),
		log:               log,
	}
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

	args := append(e.baseArgs(), []string{
		"--output", dest + "/%(title)s.%(ext)s",
		"--write-info-json",
		"--no-playlist",
		"--progress",
		"--newline",
		"--progress-template", "download:tanuki:%(progress.downloaded_bytes)s:%(progress.total_bytes_estimate)s:%(progress.total_bytes)s",
	}...)
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

	stderrLines := &logBuffer{}
	go func() {
		scanner := bufio.NewScanner(stderr)
		for scanner.Scan() {
			line := scanner.Text()
			stderrLines.Add(line)
			e.log.Debug("yt-dlp", zap.String("stderr", line))
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
		if unsupportedErr := classifyYtDlpError(stderrLines.Joined()); unsupportedErr != nil {
			return unsupportedErr
		}
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
	args := append(e.baseArgs(), "--dump-json", "--no-playlist")
	args = append(args, url)

	out, err := exec.Command("yt-dlp", args...).CombinedOutput()
	if err != nil {
		if unsupportedErr := classifyYtDlpError(string(out)); unsupportedErr != nil {
			return nil, unsupportedErr
		}
		trimmed := strings.TrimSpace(string(out))
		if trimmed != "" {
			return nil, fmt.Errorf("yt-dlp metadata: %w: %s", err, trimmed)
		}
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

func (e *YtDlpEngine) baseArgs() []string {
	args := make([]string, 0, 8)
	if e.hasConfig() {
		args = append(args, "--config-location", e.configPath)
	}
	if strings.TrimSpace(e.cookiesPath) != "" {
		args = append(args, "--cookies", e.cookiesPath)
	}
	if strings.TrimSpace(e.impersonateClient) != "" {
		args = append(args, "--impersonate", e.impersonateClient)
		args = append(args, "--extractor-args", "generic:impersonate="+e.impersonateClient)
	}
	return args
}

func classifyYtDlpError(output string) error {
	lower := strings.ToLower(output)
	switch {
	case strings.Contains(lower, "cloudflare anti-bot challenge"):
		return newUnsupportedURLError("yt-dlp", blockedChallengeDetail("source blocked by Cloudflare anti-bot challenge"))
	case strings.Contains(lower, "unsupported url"):
		return newUnsupportedURLError("yt-dlp", strings.TrimSpace(output))
	case strings.Contains(lower, "http error 403"), strings.Contains(lower, "http error 401"), strings.Contains(lower, "http error 429"):
		return newUnsupportedURLError("yt-dlp", blockedChallengeDetail("remote source blocked yt-dlp with an HTTP challenge"))
	default:
		return nil
	}
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
